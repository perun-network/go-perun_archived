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
	"math/big"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	simchannel "perun.network/go-perun/backend/sim/channel"
	simwallet "perun.network/go-perun/backend/sim/wallet"
	"perun.network/go-perun/channel"
	"perun.network/go-perun/channel/test"
	"perun.network/go-perun/pkg/io"
	pkgtest "perun.network/go-perun/pkg/test"
	"perun.network/go-perun/wallet"
)

func TestNewPrefundedLedger(t *testing.T) {
	rng := pkgtest.Prng(t)
	tests := []struct {
		name     string
		accounts []wallet.Address
		alloc    channel.Allocation
		valid    bool
	}{
		{
			"single part/positive starting balance",
			[]wallet.Address{simwallet.NewRandomAddress(rng)},
			*test.NewRandomAllocation(rng, test.WithNumAssets(1), test.WithBalances([]channel.Bal{big.NewInt(10)})),
			true,
		},
		{
			"single part/negative starting balance",
			[]wallet.Address{simwallet.NewRandomAddress(rng)},
			*test.NewRandomAllocation(rng, test.WithNumAssets(1), test.WithBalances([]channel.Bal{big.NewInt(-10)})),
			false,
		},
	}

	for _, tt := range tests {
		if _, err := simchannel.NewPrefundedLedger(tt.accounts, &tt.alloc); (err == nil) != tt.valid {
			t.Errorf("test %v : got `%v`, want valid = %v", tt.name, err, tt.valid)
		}
	}
}

func TestLedgerAddSub(t *testing.T) {
	rng := pkgtest.Prng(t)
	tests := []struct {
		name      string
		account   simwallet.Address
		alloc     channel.Allocation
		amountAdd *big.Int
		amountSub *big.Int
		wantAdd   *big.Int
		wantSub   *big.Int
		validAdd  bool
		validSub  bool
	}{
		{
			"single asset/one part/add positive/sub positive",
			*simwallet.NewRandomAddress(rng),
			*test.NewRandomAllocation(rng, test.WithNumAssets(1), test.WithBalances([]channel.Bal{big.NewInt(0)})),
			big.NewInt(10),
			big.NewInt(10),
			big.NewInt(10),
			big.NewInt(0),
			true,
			true,
		},
		{
			"single asset/one part/invalid add/insufficient funds/valid sub",
			*simwallet.NewRandomAddress(rng),
			*test.NewRandomAllocation(rng, test.WithNumAssets(1), test.WithBalances([]channel.Bal{big.NewInt(0)})),
			big.NewInt(-10),
			big.NewInt(-10),
			big.NewInt(0),
			big.NewInt(10),
			false,
			true,
		},
		{
			"single asset/one part/valid add/invalid sub/insufficient funds",
			*simwallet.NewRandomAddress(rng),
			*test.NewRandomAllocation(rng, test.WithNumAssets(1), test.WithBalances([]channel.Bal{big.NewInt(0)})),
			big.NewInt(0),
			big.NewInt(10),
			big.NewInt(0),
			big.NewInt(0),
			true,
			false,
		},
	}
	for _, tt := range tests {
		ledger, _ := simchannel.NewPrefundedLedger([]wallet.Address{&tt.account}, &tt.alloc)
		asset := tt.alloc.Assets[0].(*simchannel.Asset)
		if err := ledger.Add(asset, &tt.account, tt.amountAdd); (err == nil) != tt.validAdd {
			t.Errorf("test %v : got %v, want valid = %v", tt.name, err, tt.validAdd)
		}
		if got := ledger.Bal(*asset, &tt.account); got.Cmp(tt.wantAdd) != 0 {
			t.Errorf("ledger.Add(%v) = %v, want %v ", tt.amountAdd, got, tt.wantAdd)
		}
		if err := ledger.Sub(asset, &tt.account, tt.amountSub); (err == nil) != tt.validSub {
			t.Errorf("test %v : got %v, want valid = %v", tt.name, err, tt.validSub)
		}
		if got := ledger.Bal(*asset, &tt.account); got.Cmp(tt.wantSub) != 0 {
			t.Errorf("ledger.Sub(%v) = %v, want %v ", tt.amountSub, got, tt.wantSub)
		}
	}
}

func TestLedgerTransfer(t *testing.T) {
	rng := pkgtest.Prng(t)
	tests := []struct {
		name   string
		from   simwallet.Address
		to     simwallet.Address
		alloc  channel.Allocation
		amount *big.Int
		valid  bool
	}{
		{
			"single asset/two parts/positive amount",
			*simwallet.NewRandomAddress(rng),
			*simwallet.NewRandomAddress(rng),
			*test.NewRandomAllocation(rng,
				test.WithNumAssets(1),
				test.WithBalances([]channel.Bal{big.NewInt(10), big.NewInt(0)})),
			big.NewInt(5),
			true,
		},
		{
			"single asset/two parts/invalid transfer/excessive positive amount",
			*simwallet.NewRandomAddress(rng),
			*simwallet.NewRandomAddress(rng),
			*test.NewRandomAllocation(rng,
				test.WithNumAssets(1),
				test.WithBalances([]channel.Bal{big.NewInt(10), big.NewInt(0)})),
			big.NewInt(11),
			false,
		},
		{
			"single asset/two parts/negative amount",
			*simwallet.NewRandomAddress(rng),
			*simwallet.NewRandomAddress(rng),
			*test.NewRandomAllocation(rng,
				test.WithNumAssets(1),
				test.WithBalances([]channel.Bal{big.NewInt(0), big.NewInt(10)})),
			big.NewInt(-5),
			false,
		},
		{
			"single asset/two parts/invalid transfer/excessive negative amount",
			*simwallet.NewRandomAddress(rng),
			*simwallet.NewRandomAddress(rng),
			*test.NewRandomAllocation(rng,
				test.WithNumAssets(1),
				test.WithBalances([]channel.Bal{big.NewInt(0), big.NewInt(10)})),
			big.NewInt(-11),
			false,
		},
	}

	for _, tt := range tests {
		ledger, _ := simchannel.NewPrefundedLedger([]wallet.Address{&tt.from, &tt.to}, &tt.alloc)
		asset := tt.alloc.Assets[0].(*simchannel.Asset)

		initFrom, initTo := new(big.Int).Set(ledger.Bal(*asset, &tt.from)), new(big.Int).Set(ledger.Bal(*asset, &tt.to))
		if err := ledger.Transfer(asset, &tt.from, &tt.to, tt.amount); (err == nil) != tt.valid {
			t.Errorf("test %v : got %v, want %v", tt.name, err, tt.valid)
		}
		if tt.valid {
			gotFrom, gotTo := ledger.Bal(*asset, &tt.from), ledger.Bal(*asset, &tt.to)

			if wantFrom, wantTo := initFrom.Sub(initFrom, tt.amount), initTo.Add(initTo, tt.amount); gotFrom.Cmp(wantFrom) != 0 || gotTo.Cmp(wantTo) != 0 {
				t.Errorf("ledger.Transfer(%v) = got From %v and To %v, want %v and %v ",
					tt.amount, gotFrom, gotTo, wantFrom, wantTo)
			}
		}
	}
}

func TestLedger1PartyDeposit(t *testing.T) {
	rng := pkgtest.Prng(t)

	for _, tt := range singlePartyTable {
		assets, ledger, params, chanState := setupSinglePartyTC(rng, tt)

		ledger.Deposit(
			context.Background(),
			channel.FundingReq{
				Params: params,
				State:  chanState,
				Idx:    tt.recipient,
			}, &tt.funder)

		indexes := getAssetIdxs(assets)
		for asset, userBals := range ledger.Bals() {
			for user, bal := range userBals {
				if user.Equals(&tt.funder) && bal.Cmp(tt.wantLedger[indexes[asset]][0]) != 0 {
					t.Errorf("%v Deposit(): funder balance not expected for asset %v: want %v, got %v",
						tt.name, asset, tt.wantLedger[indexes[asset]][0], bal)
				}
			}
		}

		for i, asset := range ledger.ChannelDepots(params.ID()) {
			if asset[tt.recipient].Cmp(tt.wantChan[i][0]) != 0 {
				t.Errorf("%v Deposit(): recipient balance %v, want %v",
					tt.name, asset[tt.recipient], tt.wantChan[i][0])
			}
		}
	}
}

func setupSinglePartyTC(rng *rand.Rand, tc singlePartyTest) (
	[]channel.Asset,
	*simchannel.Ledger,
	*channel.Params,
	*channel.State,
) {
	assets := test.NewRandomAssets(rng, test.WithNumAssets(tc.numAssets))
	alloc := test.NewRandomAllocation(
		rng, test.WithAssets(assets...),
		test.WithBalances(tc.initLedgerBals...),
	)
	ledger, _ := simchannel.NewPrefundedLedger([]wallet.Address{&tc.funder}, alloc)
	params, chanState := test.NewRandomParamsAndState(
		rng,
		test.WithAssets(assets...),
		test.WithBalances(tc.proposedChan...),
	)
	return assets, ledger, params, chanState
}

func TestLedger2PartiesDepositParallel(t *testing.T) {
	rng := pkgtest.Prng(t)

	for _, tt := range twoPartyTable {
		assets, ledger, params, chanState := setupTwoPartyTC(rng, tt)
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*50)
		defer cancel()

		var wg sync.WaitGroup
		for _, idx := range tt.recipientIdxs {
			wg.Add(1)
			idx := idx
			go func() {
				ledger.Deposit(ctx,
					channel.FundingReq{
						Params: params,
						State:  chanState,
						Idx:    idx,
					}, tt.parties[idx])
				wg.Done()
			}()
		}
		wg.Wait()

		assetMap := getAssetIdxs(assets)

		assertLedgerBalsFor(t, tt.name, tt.wantLedger, tt.parties[1], ledger, assetMap)
		assertChannelBalsFor(t, tt.name, tt.wantChan, params.ID(), ledger)
	}
}

func TestLedger2PartiesDepositSequential(t *testing.T) {
	rng := pkgtest.Prng(t)

	for _, tt := range twoPartyTable {
		assets, ledger, params, chanState := setupTwoPartyTC(rng, tt)
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*40)
		defer cancel()
		assetMap := getAssetIdxs(assets)

		ledger.Deposit(ctx,
			channel.FundingReq{
				Params: params,
				State:  chanState,
				Idx:    tt.recipientIdxs[0],
			}, tt.parties[0])

		// tests that only entries of tt.party[0] were modified on the ledger.
		for asset, holders := range ledger.Bals() {
			for addr, bal := range holders {
				if addr.Equals(tt.parties[0]) {
					assert.Equalf(t, tt.wantLedger[assetMap[asset]][0].Int64(), bal.Int64(),
						"player %d should have %d funds on the ledger for asset %d",
						0, tt.wantLedger[assetMap[asset]][0].Int64(), assetMap[asset])
				} else if addr.Equals((tt.parties[1])) {
					assert.Equal(t, tt.initLedgerBals[assetMap[asset]][1].Int64(), bal.Int64(),
						"player %d should have %d funds for asset %d on the ledger",
						1, tt.wantLedger[assetMap[asset]][1].Int64(), assetMap[asset])
				}
			}
		}

		// tests that only entries of tt.party[0] were modified for the channels.
		for assetIdx, asset := range ledger.ChannelDepots(params.ID()) {
			for balIdx, bal := range asset {
				if balIdx == 0 {
					assert.Equal(t, tt.wantChan[assetIdx][balIdx].Int64(), bal.Int64(),
						"player %d should have %d funds in the channel for asset %d",
						balIdx, tt.wantChan[assetIdx][balIdx].Int64(), assetIdx)
				} else {
					assert.Equal(t, tt.initChanBals[assetIdx][balIdx].Int64(), bal.Int64(),
						"player %d should have %d funds in the channel for asset %d",
						balIdx, tt.wantChan[assetIdx][balIdx].Int64(), assetIdx)
				}
			}
		}

		ledger.Deposit(ctx,
			channel.FundingReq{
				Params: params,
				State:  chanState,
				Idx:    tt.recipientIdxs[1],
			}, tt.parties[1])

		// tests if expected channel allocation is setup and ledger balances
		// were subtracted.
		assertLedgerBalsFor(t, tt.name, tt.wantLedger, tt.parties[1], ledger, assetMap)
		assertChannelBalsFor(t, tt.name, tt.wantChan, params.ID(), ledger)
	}
}

func setupTwoPartyTC(rng *rand.Rand, tc twoPartyTestcase) (
	[]channel.Asset,
	*simchannel.Ledger,
	*channel.Params,
	*channel.State,
) {
	assets := test.NewRandomAssets(rng, test.WithNumAssets(tc.numAssets))
	alloc := test.NewRandomAllocation(
		rng, test.WithAssets(assets...),
		test.WithBalances(tc.initLedgerBals...),
	)
	ledger, _ := simchannel.NewPrefundedLedger(tc.parties, alloc)
	params, chanState := test.NewRandomParamsAndState(
		rng,
		test.WithAssets(assets...),
		test.WithBalances(tc.proposedChan...),
	)
	return assets, ledger, params, chanState
}

// assertLedgerBalsFor allows to assert for a two party channel if the allocation
// of funds for all addresses are as expected.
func assertLedgerBalsFor(
	t *testing.T,
	name string,
	expected [][]channel.Bal,
	otherParty wallet.Address,
	l *simchannel.Ledger,
	assetMap map[io.Encoder]int,
) {
	for asset, holders := range l.Bals() {
		for addr, bal := range holders {
			addrIdx := 0
			if addr.Equals(otherParty) {
				addrIdx = 1
			}
			assert.Equal(t, expected[assetMap[asset]][addrIdx].Int64(), bal.Int64(),
				"%v Player %d should have %d funds on the ledger for asset %d",
				name, addrIdx, expected[assetMap[asset]][addrIdx].Int64(), assetMap[asset])
		}
	}
}

func assertChannelBalsFor(
	t *testing.T,
	name string,
	expected [][]channel.Bal,
	id channel.ID,
	l *simchannel.Ledger,
) {
	for assetIdx, asset := range l.ChannelDepots(id) {
		for balIdx, bal := range asset {
			assert.Equal(t, expected[assetIdx][balIdx].Int64(), bal.Int64(),
				"%v Player %d should have %d funds in the channel for asset %d",
				name, balIdx, expected[assetIdx][balIdx].Int64(), assetIdx)
		}
	}
}

func getAssetIdxs(assets []channel.Asset) map[channel.Asset]int {
	indexes := make(map[channel.Asset]int)
	for idx, asset := range assets {
		indexes[*asset.(*simchannel.Asset)] = idx
	}
	return indexes
}
