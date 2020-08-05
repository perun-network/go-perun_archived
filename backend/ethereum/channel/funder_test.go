// Copyright 2019 - See NOTICE file for copyright holders.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package channel_test

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"perun.network/go-perun/backend/ethereum/bindings/assets"
	ethchannel "perun.network/go-perun/backend/ethereum/channel"
	"perun.network/go-perun/backend/ethereum/channel/test"
	ethwallet "perun.network/go-perun/backend/ethereum/wallet"
	ethwallettest "perun.network/go-perun/backend/ethereum/wallet/test"
	"perun.network/go-perun/channel"
	channeltest "perun.network/go-perun/channel/test"
	"perun.network/go-perun/wallet"
	wallettest "perun.network/go-perun/wallet/test"
)

func TestFunderZeroBalance(t *testing.T) {
	channeltest.FunderZeroBalance(t, tlfactory{})
}

func TestFunder_Fund(t *testing.T) {
	channeltest.Funder_Fund(t, tlfactory{})
}

func TestPeerTimedOutFundingError(t *testing.T) {
	channeltest.PeerTimedOutFundingError(t, tlfactory{})
}

func TestFunder_Fund_multi(t *testing.T) {
	channeltest.Funder_Fund_Multi(t, tlfactory{})
}

var _ channeltest.TestableLedger = new(uniformLedger)

// uniformLedger wraps a backend, in this case `test.SimulatedBackend` and implements
// the `channeltest.TestableLedger` interface to allow testing via generic tests.
type uniformLedger struct {
	simBackend *test.SimulatedBackend
}

type tlfactory struct{}

// TLFactory builds a distinct `TestableLedger` instance which can be used in
// subsequent tests.
func (tlf tlfactory) Build() channeltest.TestableLedger {
	return new(uniformLedger)
}

// PostFundingCheck is a test which manually calls `funder.Fund(...)` and
// asserts that ledger-specific postconditions are satisfied. E.g. nonce checks.
func (ul *uniformLedger) PostFundingCheck(t *testing.T,
	addr wallet.Address,
	funder channel.Funder,
	req channel.FundingReq,
	success bool) {
	diff, err := test.NonceDiff(addr, funder.(*ethchannel.Funder), func() error {
		return funder.Fund(context.Background(), req)
	})
	require.NoError(t, err)
	if success {
		assert.Equal(t, int(1), diff, "Nonce should increase by 1")
	} else {
		assert.Zero(t, diff, "Nonce should stay the same")
	}
}

// CompareOnChainAlloc compares the allocations for all accounts and assets
// onchain to the given allocation.
func (ul *uniformLedger) CompareOnChainAlloc(params *channel.Params,
	alloc channel.Allocation,
	funder channel.Funder) error {
	cb := &funder.(*ethchannel.Funder).ContractBackend
	onChain, err := getOnChainAllocation(context.Background(), cb, params, alloc.Assets)
	if err != nil {
		return errors.WithMessage(err, "getting on-chain allocation")
	}
	for a := range onChain {
		for p := range onChain[a] {
			if alloc.Balances[a][p].Cmp(onChain[a][p]) != 0 {
				return errors.Errorf("balances[%d][%d] differ. Expected: %v, on-chain: %v",
					a, p, alloc.Balances[a][p], onChain[a][p])
			}
		}
	}
	return nil
}

// AdvanceTimeBy advances the time for the underlying Ledger.
func (ul *uniformLedger) AdvanceTimeBy(ctx context.CancelFunc, t *testing.T, challengeDuration uint64) {
	// advance block time so that funding fails for non-funders
	require.NoError(t, ul.simBackend.AdjustTime(time.Duration(challengeDuration)*time.Second))
	ul.simBackend.Commit()
}

// NewNFunders creates a new prefunded blockchain for N participants and
// appropriate funds. `allocation` is the allocation which should be used
// in a `channel.fundingReq` to fund a new channel. Using this allocation
// is guaranteed to result in a successful funding.
func (ul *uniformLedger) NewNFunders(
	ctx context.Context,
	t *testing.T,
	rng *rand.Rand,
	n int,
) (
	parts []wallet.Address,
	funders []channel.Funder,
	params *channel.Params,
	allocation *channel.Allocation,
) {
	ul.simBackend = test.NewSimulatedBackend()
	ks := ethwallettest.GetKeystore()
	deployAccount := &wallettest.NewRandomAccount(rng).(*ethwallet.Account).Account
	ul.simBackend.FundAddress(ctx, deployAccount.Address)
	contractBackend := ethchannel.NewContractBackend(ul.simBackend, ks, deployAccount)
	// Deploy Assetholder
	assetETH, err := ethchannel.DeployETHAssetholder(ctx, contractBackend, deployAccount.Address)
	require.NoError(t, err, "Deployment should succeed")
	t.Logf("asset holder address is %v", assetETH)
	parts = make([]wallet.Address, n)
	funders = make([]channel.Funder, n)
	for i := 0; i < n; i++ {
		acc := wallettest.NewRandomAccount(rng).(*ethwallet.Account)
		ul.simBackend.FundAddress(ctx, acc.Account.Address)
		parts[i] = acc.Address()
		cb := ethchannel.NewContractBackend(ul.simBackend, ks, &acc.Account)
		funders[i] = ethchannel.NewETHFunder(cb, assetETH)
	}
	// The SimBackend advances 10 sec per transaction/block, so generously add 20
	// sec funding duration per participant
	params = channeltest.NewRandomParams(rng,
		channeltest.WithParts(parts...),
		channeltest.WithChallengeDuration(uint64(n)*20))
	allocation = channeltest.NewRandomAllocation(rng,
		channeltest.WithNumParts(len(parts)),
		channeltest.WithAssets((*ethchannel.Asset)(&assetETH)))
	return
}

func getOnChainAllocation(ctx context.Context,
	cb *ethchannel.ContractBackend,
	params *channel.Params,
	_assets []channel.Asset) ([][]channel.Bal, error) {
	partIDs := ethchannel.FundingIDs(params.ID(), params.Parts...)
	alloc := make([][]channel.Bal, len(_assets))

	for k, asset := range _assets {
		alloc[k] = make([]channel.Bal, len(params.Parts))
		contract, err := assets.NewAssetHolder(common.Address(*asset.(*ethchannel.Asset)), cb)
		if err != nil {
			return nil, err
		}

		for i, id := range partIDs {
			opts := bind.CallOpts{
				Pending: false,
				Context: ctx,
			}
			val, err := contract.Holdings(&opts, id)
			if err != nil {
				return nil, err
			}
			alloc[k][i] = val
		}
	}
	return alloc, nil
}
