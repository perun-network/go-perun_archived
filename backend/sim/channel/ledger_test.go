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
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	simchannel "perun.network/go-perun/backend/sim/channel"
	simwallet "perun.network/go-perun/backend/sim/wallet"
	"perun.network/go-perun/channel"
	"perun.network/go-perun/channel/test"
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
		got, err := ledger.Bal(*asset, &tt.account)
		assert.NoError(t, err)
		if got.Cmp(tt.wantAdd) != 0 {
			t.Errorf("ledger.Add(%v) = %v, want %v ", tt.amountAdd, got, tt.wantAdd)
		}

		if err := ledger.Sub(asset, &tt.account, tt.amountSub); (err == nil) != tt.validSub {
			t.Errorf("test %v : got %v, want valid = %v", tt.name, err, tt.validSub)
		}
		got, err = ledger.Bal(*asset, &tt.account)
		assert.NoError(t, err)
		if got.Cmp(tt.wantSub) != 0 {
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

		initFrom, err := ledger.Bal(*asset, &tt.from)
		assert.NoError(t, err)
		initTo, err := ledger.Bal(*asset, &tt.to)
		assert.NoError(t, err)

		if tt.valid {
			assert.NoError(t, ledger.Transfer(asset, &tt.from, &tt.to, tt.amount),
				"%v", tt.name)

			gotFrom, err := ledger.Bal(*asset, &tt.from)
			assert.NoError(t, err)
			gotTo, err := ledger.Bal(*asset, &tt.to)
			assert.NoError(t, err)
			wantFrom, wantTo := initFrom.Sub(initFrom, tt.amount), initTo.Add(initTo, tt.amount)
			if gotFrom.Cmp(wantFrom) != 0 || gotTo.Cmp(wantTo) != 0 {
				t.Errorf("ledger.Transfer(%v) = got From %v and To %v, want %v and %v ",
					tt.amount, gotFrom, gotTo, wantFrom, wantTo)
			}
		} else {
			assert.Error(t, ledger.Transfer(asset, &tt.from, &tt.to, tt.amount),
				tt.name)
		}
	}
}
