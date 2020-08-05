// Copyright 2020 - See NOTICE file for copyright holders.
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

	"github.com/pkg/errors"
	simchannel "perun.network/go-perun/backend/sim/channel"
	"perun.network/go-perun/channel"
	"perun.network/go-perun/channel/test"
	"perun.network/go-perun/wallet"
	wallettest "perun.network/go-perun/wallet/test"
)

func TestFunderZeroBalance(t *testing.T) {
	test.FunderZeroBalance(t, tlfactory{})
}

func TestFunder_Fund(t *testing.T) {
	test.Funder_Fund(t, tlfactory{})
}

func TestPeerTimedOutFundingError(t *testing.T) {
	test.PeerTimedOutFundingError(t, tlfactory{})
}

func TestFunder_Fund_multi(t *testing.T) {
	test.Funder_Fund_Multi(t, tlfactory{})
}

var _ test.TestableLedger = new(uniformLedger)

// uniformLedger wraps a backend, in this case `simchannel.Ledger` and implements
// the `test.TestableLedger` interface to allow testing via generic tests.
type uniformLedger struct {
	simLedger *simchannel.Ledger
}

type tlfactory struct{}

// TLFactory builds a distinct `TestableLedger` instance which can be used in
// subsequent tests.
func (tlf tlfactory) Build() test.TestableLedger {
	return new(uniformLedger)
}

// PostFundingCheck is a test which manually calls `funder.Fund(...)` and
// asserts that ledger-specific postconditions are satisfied. E.g. nonce checks.
func (ul *uniformLedger) PostFundingCheck(t *testing.T,
	addr wallet.Address,
	funder channel.Funder,
	req channel.FundingReq,
	success bool) {
	funder.Fund(context.Background(), req)
}

// CompareOnChainAlloc compares the allocations for all accounts and assets
// onchain to the given allocation.
func (ul *uniformLedger) CompareOnChainAlloc(params *channel.Params,
	alloc channel.Allocation,
	funder channel.Funder) error {
	onChainBals := ul.simLedger.ChannelBals(params.ID())

	if len(onChainBals) != len(alloc.Balances) {
		return errors.New("onchain [][]channel.Bals length does not match expected allocation")
	}

	for assetIdx := range onChainBals {
		for partIdx := range onChainBals[assetIdx] {
			if alloc.Balances[assetIdx][partIdx].Cmp(onChainBals[assetIdx][partIdx]) != 0 {
				return errors.Errorf("balances[%d][%d] differ. Expected: %v, on-chain: %v",
					assetIdx, partIdx, alloc.Balances[assetIdx][partIdx], onChainBals[assetIdx][partIdx])
			}
		}
	}
	return nil
}

// AdvanceTimeBy advances the time for the underlying Ledger. Just calls
// `cancelFunc()` since the `simchannel.Ledger` does not have a concept of time.
func (ul *uniformLedger) AdvanceTimeBy(cancelFunc context.CancelFunc, _ *testing.T, _ uint64) {
	cancelFunc()
}

// NewNFunders creates a new prefunded blockchain for N participants and
// appropriate funds. `allocation` is the allocation which should be used
// in a `channel.fundingReq` to fund a new channel. Using this allocation
// is guaranteed to result in a successful funding.
func (ul *uniformLedger) NewNFunders(
	_ context.Context,
	t *testing.T,
	rng *rand.Rand,
	numParts int,
) (
	parts []wallet.Address,
	funders []channel.Funder,
	params *channel.Params,
	allocation *channel.Allocation,
) {
	parts = make([]wallet.Address, numParts)
	for partIdx := range parts {
		parts[partIdx] = wallettest.NewRandomAddress(rng)
	}
	assets := test.NewRandomAssets(rng, test.WithNumAssets(1))
	ledgerAlloc := test.NewRandomAllocation(rng,
		test.WithAssets(assets...),
		test.WithParts(parts...),
		test.WithBalancesInRange(int64(100), int64(200)))
	params = test.NewRandomParams(rng, test.WithParts(parts...),
		test.WithChallengeDuration(uint64(numParts)*1))

	var err error
	ul.simLedger, err = simchannel.NewPrefundedLedger(parts, ledgerAlloc)
	if err != nil {
		t.Fatal("unable to create prefunded ledger")
	}

	funders = make([]channel.Funder, numParts)
	for partIdx := range parts {
		funders[partIdx] = simchannel.NewSimFunder(ul.simLedger, parts[partIdx])
	}

	allocation = test.NewRandomAllocation(rng,
		test.WithAssets(assets...),
		test.WithParts(parts...),
		test.WithBalancesInRange(int64(1), int64(4)))
	return
}
