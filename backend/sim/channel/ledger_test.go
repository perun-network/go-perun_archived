// Copyright (c) 2020 Chair of Applied Cryptography, Technische Universität
// Darmstadt, Germany. All rights reserved. This file is part of go-perun. Use
// of this source code is governed by the Apache 2.0 license that can be found
// in the LICENSE file.

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

func TestLedgerAdd(t *testing.T) {
	rng := pkgtest.Prng(t)
	tests := []struct {
		name    string
		account simwallet.Address
		alloc   channel.Allocation
		amount  *big.Int
		want    *big.Int
		valid   bool
	}{
		{
			"single asset/one part/add positive",
			*simwallet.NewRandomAddress(rng),
			*test.NewRandomAllocation(rng, test.WithNumAssets(1), test.WithBalances([]channel.Bal{big.NewInt(0)})),
			big.NewInt(10),
			big.NewInt(10),
			true,
		},
		{
			"single asset/one part/invalid add/insufficient funds",
			*simwallet.NewRandomAddress(rng),
			*test.NewRandomAllocation(rng, test.WithNumAssets(1), test.WithBalances([]channel.Bal{big.NewInt(0)})),
			big.NewInt(-10),
			big.NewInt(0),
			false,
		},
	}
	for _, tt := range tests {
		ledger, _ := simchannel.NewPrefundedLedger([]wallet.Address{&tt.account}, &tt.alloc)
		asset := tt.alloc.Assets[0].(*simchannel.Asset)
		if err := ledger.Add(asset, &tt.account, tt.amount); (err == nil) != tt.valid {
			t.Errorf("test %v : got %v, want valid = %v", tt.name, err, tt.valid)
		}
		if got := ledger.Bal(asset, &tt.account); got.Cmp(tt.want) != 0 {
			t.Errorf("ledger.Add(%v) = %v, want %v ", tt.amount, got, tt.want)
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
			*test.NewRandomAllocation(rng, test.WithNumAssets(1), test.WithBalances([]channel.Bal{big.NewInt(10), big.NewInt(0)})),
			big.NewInt(5),
			true,
		},
		{
			"single asset/two parts/invalid transfer/excessive positive amount",
			*simwallet.NewRandomAddress(rng),
			*simwallet.NewRandomAddress(rng),
			*test.NewRandomAllocation(rng, test.WithNumAssets(1), test.WithBalances([]channel.Bal{big.NewInt(10), big.NewInt(0)})),
			big.NewInt(11),
			false,
		},
		{
			"single asset/two parts/negative amount",
			*simwallet.NewRandomAddress(rng),
			*simwallet.NewRandomAddress(rng),
			*test.NewRandomAllocation(rng, test.WithNumAssets(1), test.WithBalances([]channel.Bal{big.NewInt(0), big.NewInt(10)})),
			big.NewInt(-5),
			false,
		},
		{
			"single asset/two parts/invalid transfer/excessive negative amount",
			*simwallet.NewRandomAddress(rng),
			*simwallet.NewRandomAddress(rng),
			*test.NewRandomAllocation(rng, test.WithNumAssets(1), test.WithBalances([]channel.Bal{big.NewInt(0), big.NewInt(10)})),
			big.NewInt(-11),
			false,
		},
	}

	for _, tt := range tests {
		ledger, _ := simchannel.NewPrefundedLedger([]wallet.Address{&tt.from, &tt.to}, &tt.alloc)
		asset := tt.alloc.Assets[0].(*simchannel.Asset)

		initFrom, initTo := new(big.Int).Set(ledger.Bal(asset, &tt.from)), new(big.Int).Set(ledger.Bal(asset, &tt.to))
		if err := ledger.Transfer(asset, &tt.from, &tt.to, tt.amount); (err == nil) != tt.valid {
			t.Errorf("test %v : got %v, want %v", tt.name, err, tt.valid)
		}
		if tt.valid {
			gotFrom, gotTo := ledger.Bal(asset, &tt.from), ledger.Bal(asset, &tt.to)

			if wantFrom, wantTo := initFrom.Sub(initFrom, tt.amount), initTo.Add(initTo, tt.amount); gotFrom.Cmp(wantFrom) != 0 || gotTo.Cmp(wantTo) != 0 {
				t.Errorf("ledger.Transfer(%v) = got From %v and To %v, want %v and %v ", tt.amount, gotFrom, gotTo, wantFrom, wantTo)
			}
		}
	}
}

func TestLedger1PartyDeposit(t *testing.T) {
	rng := pkgtest.Prng(t)
	tests := []struct {
		name      string
		funder    simwallet.Address
		recipient channel.Index
		numAssets int
		initBals  [][]channel.Bal
		chBals    [][]channel.Bal
	}{
		{
			"1party/three assets/valid deposit",
			*simwallet.NewRandomAddress(rng),
			channel.Index(0),
			3,
			[][]channel.Bal{
				{big.NewInt(1)},
				{big.NewInt(2)},
				{big.NewInt(3)},
			},
			[][]channel.Bal{
				{big.NewInt(0)},
				{big.NewInt(0)},
				{big.NewInt(0)},
			},
		},
		{
			"1party/three assets/invalid deposit/zero balance",
			*simwallet.NewRandomAddress(rng),
			channel.Index(0),
			3,
			[][]channel.Bal{
				{big.NewInt(0)},
				{big.NewInt(0)},
				{big.NewInt(0)},
			},
			[][]channel.Bal{
				{big.NewInt(0)},
				{big.NewInt(0)},
				{big.NewInt(0)},
			},
		},
	}

	for _, tt := range tests {
		assets := test.NewRandomAssets(rng, test.WithNumAssets(tt.numAssets))
		ledger, _ := simchannel.NewPrefundedLedger([]wallet.Address{&tt.funder}, test.NewRandomAllocation(rng, test.WithAssets(assets...), test.WithBalances(tt.initBals...)))
		params, initState := test.NewRandomParamsAndState(rng, test.WithAssets(assets...), test.WithBalances(tt.chBals...))

		ledger.Deposit(channel.FundingReq{
			Params: params,
			State:  initState,
			Idx:    tt.recipient,
		}, &tt.funder)

		for asset, userBals := range ledger.Bals() {
			for user, bal := range userBals {
				if user.Equals(&tt.funder) && bal.Sign() != 0 {
					t.Errorf("Deposit(): funder balance not empty for asset %v: want 0, got %v", asset, bal)
				}

			}
		}

		for i, asset := range ledger.ChannelDepots(params.ID()) {
			if asset[tt.recipient].Cmp(tt.initBals[i][0]) != 0 {
				t.Errorf("Deposit(): recipient balance %v, want %v", asset[tt.recipient], tt.initBals[i][0])
			}
		}
	}
}

func TestLedger2PartiesDeposit(t *testing.T) {
	rng := pkgtest.Prng(t)
	tests := []struct {
		name          string
		parties       []wallet.Address
		recipientIdxs []channel.Index
		numAssets     int
		initBals      [][]channel.Bal
		chParts       int
		chBals        [][]channel.Bal
	}{
		{
			"2parties/three assets/valid deposit",
			[]wallet.Address{simwallet.NewRandomAddress(rng), simwallet.NewRandomAddress(rng)},
			[]channel.Index{channel.Index(0), channel.Index(1)},
			3,
			[][]channel.Bal{
				{big.NewInt(1), big.NewInt(4)},
				{big.NewInt(2), big.NewInt(5)},
				{big.NewInt(3), big.NewInt(6)},
			},
			1,
			[][]channel.Bal{
				{big.NewInt(0), big.NewInt(0)},
				{big.NewInt(0), big.NewInt(0)},
				{big.NewInt(0), big.NewInt(0)},
			},
		},
	}

	for _, tt := range tests {
		assets := test.NewRandomAssets(rng, test.WithNumAssets(tt.numAssets))
		ledger, _ := simchannel.NewPrefundedLedger(tt.parties, test.NewRandomAllocation(rng, test.WithAssets(assets...), test.WithBalances(tt.initBals...)))
		params, initState := test.NewRandomParamsAndState(rng, test.WithAssets(assets...), test.WithBalances(tt.chBals...))

		ledger.Deposit(channel.FundingReq{
			Params: params,
			State:  initState,
			Idx:    tt.recipientIdxs[0],
		}, tt.parties[0])

		assetMap := getAssetIdxs(assets)

		for asset, holders := range ledger.Bals() {
			for addr, bal := range holders {
				if addr.Equals(tt.parties[0]) {
					assert.Equalf(t, int64(0), bal.Int64(), "p1 should have no more funds on the ledger for asset %d", assetMap[asset])
				} else if addr.Equals((tt.parties[1])) {
					assert.Equal(t, tt.initBals[assetMap[asset]][1].Int64(), bal.Int64(), "p2 should have all funds for asset %d on the ledger", assetMap[asset])
				}
			}
		}

		for assetIdx, asset := range ledger.ChannelDepots(params.ID()) {
			for balIdx, bal := range asset {
				if balIdx == 0 {
					assert.Equal(t, tt.initBals[assetIdx][balIdx].Int64(), bal.Int64(), "p1 should have all funds in the channel for asset %d", assetIdx)
				} else if balIdx == 1 {
					assert.Equal(t, bal.Int64(), int64(0), "p2 should have no fund in the channel for asset %d", assetIdx)
				}
			}
		}

		ledger.Deposit(channel.FundingReq{
			Params: params,
			State:  initState,
			Idx:    tt.recipientIdxs[1],
		}, tt.parties[1])

		for asset, holders := range ledger.Bals() {
			for addr, bal := range holders {
				addrIdx := 0
				if addr.Equals(tt.parties[1]) {
					addrIdx = 1
				}
				assert.Equal(t, int64(0), bal.Int64(), "Player %d should have no more funds on the ledger for asset %d", addrIdx, assetMap[asset])
			}
		}

		for assetIdx, asset := range ledger.ChannelDepots(params.ID()) {
			for balIdx, bal := range asset {
				assert.Equal(t, tt.initBals[assetIdx][balIdx].Int64(), bal.Int64(), "Player %d should have all funds in the channel for asset %d", balIdx, assetIdx)
			}
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
