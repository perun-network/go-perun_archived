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

package test

import (
	"context"
	"math/big"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"perun.network/go-perun/channel"
	pkgtest "perun.network/go-perun/pkg/test"
	"perun.network/go-perun/wallet"
)

const defaultTxTimeout = 2 * time.Second

// FunderZeroBalance calls the genericFunderZeroBalanceTest with a
// `TestableLedger` resulting from calling `tlf.Build()`.
func FunderZeroBalance(t *testing.T, tlf TLFactory) {
	t.Run("1 Participant", func(t *testing.T) {
		genericFunderZeroBalanceTest(t, tlf.Build(), 1)
	})
	t.Run("2 Participant", func(t *testing.T) {
		genericFunderZeroBalanceTest(t, tlf.Build(), 2)
	})
}

// Funder_Fund calls the genericFunderFundTest with a `TestableLedger`
// resulting from calling `tlf.Build()`.
func Funder_Fund(t *testing.T, tlf TLFactory) {
	t.Run("Fund_MiscCases", func(t *testing.T) {
		genericFunderFundTest(t, tlf.Build())
	})
}

// PeerTimedOutFundingError calls the genericFundingTimeoutTest with a
// `TestableLedger` resulting from calling `tlf.Build()`.
func PeerTimedOutFundingError(t *testing.T, tlf TLFactory) {
	t.Run("peer 0 faulty out of 2", func(t *testing.T) {
		genericFundingTimeoutTest(t, tlf.Build(), 0, 2)
	})
	t.Run("peer 1 faulty out of 2", func(t *testing.T) {
		genericFundingTimeoutTest(t, tlf.Build(), 1, 2)
	})
	t.Run("peer 0 faulty out of 3", func(t *testing.T) {
		genericFundingTimeoutTest(t, tlf.Build(), 0, 3)
	})
	t.Run("peer 1 faulty out of 3", func(t *testing.T) {
		genericFundingTimeoutTest(t, tlf.Build(), 1, 3)
	})
	t.Run("peer 2 faulty out of 3", func(t *testing.T) {
		genericFundingTimeoutTest(t, tlf.Build(), 2, 3)
	})
	t.Run("peer 3 faulty out of 4", func(t *testing.T) {
		genericFundingTimeoutTest(t, tlf.Build(), 3, 4)
	})
	t.Run("peer 4 faulty out of 8", func(t *testing.T) {
		genericFundingTimeoutTest(t, tlf.Build(), 4, 8)
	})
	t.Run("peer 0 faulty out of 8", func(t *testing.T) {
		genericFundingTimeoutTest(t, tlf.Build(), 0, 8)
	})
	t.Run("peer 7 faulty out of 8", func(t *testing.T) {
		genericFundingTimeoutTest(t, tlf.Build(), 7, 8)
	})
}

// Funder_Fund_Multi calls the genericFunderFundingTest with a
// `TestableLedger` resulting from calling `tlf.Build()`.
func Funder_Fund_Multi(t *testing.T, tlf TLFactory) {
	t.Run("1-party funding", func(t *testing.T) {
		genericFunderFundingTest(t, tlf.Build(), 1)
	})
	t.Run("2-party funding", func(t *testing.T) {
		genericFunderFundingTest(t, tlf.Build(), 2)
	})
	t.Run("3-party funding", func(t *testing.T) {
		genericFunderFundingTest(t, tlf.Build(), 3)
	})
	t.Run("10-party funding", func(t *testing.T) {
		genericFunderFundingTest(t, tlf.Build(), 10)
	})
}

// TLFactory builds a distinct `TestableLedger` instance which can be used in
// subsequent tests.
type TLFactory interface {
	// Build creates a new distinct `TestableLedger` instance.
	Build() TestableLedger
}

// TestableLedger is an interface that needs to be implemented for each backend
// funder test to enable testing via generic tests.
type TestableLedger interface {
	// NewNFunders creates a new prefunded blockchain for N participants and
	// appropriate funds. `allocation` is the allocation which should be used
	// in a `channel.fundingReq` to fund a new channel. Using this allocation
	// is guaranteed to result in a successful funding.
	NewNFunders(context.Context, *testing.T, *rand.Rand, int) (parts []wallet.Address,
		funders []channel.Funder,
		params *channel.Params,
		allocation *channel.Allocation)

	// CompareOnChainAlloc compares the allocations for all accounts and assets
	// onchain to the given allocation.
	CompareOnChainAlloc(*channel.Params, channel.Allocation, channel.Funder) error

	// AdvanceTimeBy advances the time for the underlying Ledger. Gets a `Duration`
	// and a `context.CancelFunc` associated to the funding of a channel.
	AdvanceTimeBy(context.CancelFunc, *testing.T, uint64)

	// PostFundingCheck is a test which manually calls `funder.Fund(...)` and
	// asserts that ledger-specific postconditions are satisfied. E.g. nonce checks.
	PostFundingCheck(t *testing.T,
		addr wallet.Address,
		funder channel.Funder,
		req channel.FundingReq,
		success bool)
}

// genericFundingTimeoutTest tests the funding timeout logic by create `peers`
// amount of peers and excluding `faultyPeer` from funding a channel. Each
// funding call is delayed by a random amount between 1-11 milliseconds. When
// all fundercalls are issued we wait 200 milliseconds and advance the time
// on the `TestLedger` so the `ChallengeDuration` is exceeded. Afterwards we
// check if the expected faultyPeer was caught as faulty by all other peers.
func genericFundingTimeoutTest(t *testing.T, tl TestableLedger, faultyPeer, peers int) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), defaultTxTimeout)
	defer cancel()
	rng := pkgtest.Prng(t)

	ct := pkgtest.NewConcurrent(t)

	_, funders, params, allocation := tl.NewNFunders(ctx, t, rng, peers)

	sleepTime := time.Duration(rng.Int63n(10) + 1)
	for i, funder := range funders {
		i, funder := i, funder
		state := NewRandomState(rng, WithID(params.ID()), WithAllocation(allocation))
		go ct.StageN("funding loop", peers, func(rt require.TestingT) {
			// Faulty peer does not fund the channel.
			if i == faultyPeer {
				return
			}
			time.Sleep(sleepTime * time.Millisecond)
			req := channel.FundingReq{
				Params: params,
				State:  state,
				Idx:    uint16(i),
			}
			defer cancel()
			err := funder.Fund(ctx, req)
			require.True(rt, channel.IsFundingTimeoutError(err),
				"funder should return FundingTimeoutError")
			pErr := errors.Cause(err).(*channel.FundingTimeoutError) // unwrap error
			assert.Equal(t, pErr.Errors[0].Asset, 0, "Wrong asset set")
			assert.Equal(t, 1, len(pErr.Errors[0].TimedOutPeers), "Unexpected amount of faulty peers")
			assert.Equal(t, uint16(faultyPeer), pErr.Errors[0].TimedOutPeers[0],
				"Peer should be detected as erroneous")
		})
	}

	time.Sleep(400 * time.Millisecond) // give all funders enough time to fund
	tl.AdvanceTimeBy(cancel, t, params.ChallengeDuration)
	ct.Wait("funding loop")
}

// genericFunderZeroBalanceTest tests the funder logic in case of zero balance
// funding calls.
func genericFunderZeroBalanceTest(t *testing.T, tl TestableLedger, n int) {
	rng := pkgtest.Prng(t)
	parts, funders, params, allocation := tl.NewNFunders(context.Background(), t, rng, n)

	for i := range parts {
		if i%2 == 0 {
			allocation.Balances[0][i].Set(big.NewInt(0))
		} // is != 0 otherwise
		t.Logf("Part: %d ShouldFund: %t Bal: %v", i, i%2 == 1, allocation.Balances[0][i])
	}
	// fund
	var wg sync.WaitGroup
	wg.Add(n)
	state := NewRandomState(rng, WithID(params.ID()), WithAllocation(allocation))
	for i := 0; i < n; i++ {
		req := channel.FundingReq{
			Params: params,
			State:  state,
			Idx:    channel.Index(i),
		}

		// Check that the funding only changes the nonce when the balance is not zero
		go func(i int) {
			defer wg.Done()
			tl.PostFundingCheck(t, parts[i], funders[i], req, i%2 != 0)
		}(i)
	}
	wg.Wait()
	// Check result balances
	assert.NoError(t, tl.CompareOnChainAlloc(params, *allocation, funders[0]))
}

// genericFunderFundTest tests miscellaneous behaviour for a funding call in
// one go. Calling fund with ill and properly formed`channel.FundingReq`.
// Funding with a already closed context and funding without any assets.
func genericFunderFundTest(t *testing.T, tl TestableLedger) {
	rng := pkgtest.Prng(t)
	ctx, cancel := context.WithTimeout(context.Background(), defaultTxTimeout)
	defer cancel()
	parts, funders, params, allocation := tl.NewNFunders(ctx, t, rng, 1)
	// Test invalid funding request
	assert.Panics(t, func() { funders[0].Fund(ctx, channel.FundingReq{}) },
		"Funding with invalid funding req should fail")
	// Test funding without assets
	alloc := NewRandomAllocation(
		rng,
		WithNumParts(len(parts)),
		WithBalancesInRange(0, 1),
		WithNumAssets(len(allocation.Assets)),
	)
	req := channel.FundingReq{
		Params: NewRandomParams(rng, WithParts(parts...)),
		State: NewRandomState(
			rng,
			WithNumParts(len(parts)),
			WithAllocation(alloc),
			WithAssets(allocation.Assets...),
		),
		Idx: 0,
	}
	assert.NoError(t, funders[0].Fund(ctx, req), "Funding with no assets should succeed")

	// Test with valid request
	state := NewRandomState(rng, WithID(params.ID()), WithAllocation(allocation))
	req = channel.FundingReq{
		Params: params,
		State:  state,
		Idx:    0,
	}

	t.Run("Funding idempotence", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			tl.PostFundingCheck(t, parts[0], funders[0], req, i == 0)
		}
	})
	// Test already closed context
	cancel()
	assert.Error(t, funders[0].Fund(ctx, req), "funding with already cancelled context should fail")
	// Check result balances
	assert.NoError(t, tl.CompareOnChainAlloc(params, *allocation, funders[0]))
}

// genericFunderFundingTest tests the funder logic for multiple valid funding
// calls from multiple peers. After those are issued and successfully returned
// compares if the onchain-allocation for a funder is the same as intended.
func genericFunderFundingTest(t *testing.T, tl TestableLedger, n int) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTxTimeout)
	defer cancel()
	rng := pkgtest.Prng(t)

	ct := pkgtest.NewConcurrent(t)

	_, funders, params, allocation := tl.NewNFunders(ctx, t, rng, n)

	for i, funder := range funders {
		sleepTime := time.Duration(rng.Int63n(10) + 1)
		i, funder := i, funder
		state := NewRandomState(rng, WithID(params.ID()), WithAllocation(allocation))
		go ct.StageN("funding", n, func(rt require.TestingT) {
			time.Sleep(sleepTime * time.Millisecond)
			req := channel.FundingReq{
				Params: params,
				State:  state,
				Idx:    channel.Index(i),
			}
			err := funder.Fund(ctx, req)
			require.NoError(rt, err, "funding should succeed")
		})
	}

	ct.Wait("funding")
	// Check result balances
	assert.NoError(t, tl.CompareOnChainAlloc(params, *allocation, funders[0]))
}
