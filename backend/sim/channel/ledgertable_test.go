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
	"math/rand"

	simwallet "perun.network/go-perun/backend/sim/wallet"
	"perun.network/go-perun/channel"
	"perun.network/go-perun/wallet"
)

// Large tabletests.
// Two constant addresses, since randomness is not required.
var (
	part1 = simwallet.NewRandomAddress(rand.New(rand.NewSource(420)))
	part2 = simwallet.NewRandomAddress(rand.New(rand.NewSource(421)))
)

type singlePartyTest struct {
	name           string
	funder         simwallet.Address
	recipient      channel.Index
	numAssets      int
	initLedgerBals [][]channel.Bal // init ledger balances for each addr
	proposedChan   [][]channel.Bal // intention of fundingReq
	wantLedger     [][]channel.Bal // reality for ledger after deposit
	wantChan       [][]channel.Bal // reality for chan after deposit
}

var singlePartyTable = []singlePartyTest{
	{
		"1party/three assets/valid deposit",
		*part1,
		channel.Index(0),
		3,
		[][]channel.Bal{
			{big.NewInt(1)},
			{big.NewInt(2)},
			{big.NewInt(3)},
		},
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
		[][]channel.Bal{
			{big.NewInt(1)},
			{big.NewInt(2)},
			{big.NewInt(3)},
		},
	},
	{
		"1party/three assets/single invalid deposit/remaining balance",
		*part1,
		channel.Index(0),
		3,
		[][]channel.Bal{
			{big.NewInt(3)},
			{big.NewInt(4)},
			{big.NewInt(5)},
		},
		[][]channel.Bal{
			{big.NewInt(3)},
			{big.NewInt(5)},
			{big.NewInt(5)},
		},
		[][]channel.Bal{
			{big.NewInt(3)},
			{big.NewInt(4)},
			{big.NewInt(5)},
		},
		[][]channel.Bal{
			{big.NewInt(0)},
			{big.NewInt(0)},
			{big.NewInt(0)},
		},
	},
	{
		"1party/three assets/all invalid deposit/remaining balance",
		*part1,
		channel.Index(0),
		3,
		[][]channel.Bal{
			{big.NewInt(3)},
			{big.NewInt(4)},
			{big.NewInt(5)},
		},
		[][]channel.Bal{
			{big.NewInt(420)},
			{big.NewInt(420)},
			{big.NewInt(420)},
		},
		[][]channel.Bal{
			{big.NewInt(3)},
			{big.NewInt(4)},
			{big.NewInt(5)},
		},
		[][]channel.Bal{
			{big.NewInt(0)},
			{big.NewInt(0)},
			{big.NewInt(0)},
		},
	},
	{
		"1party/three assets/valid deposit/remaining balance",
		*part1,
		channel.Index(0),
		3,
		[][]channel.Bal{
			{big.NewInt(3)},
			{big.NewInt(4)},
			{big.NewInt(5)},
		},
		[][]channel.Bal{
			{big.NewInt(2)},
			{big.NewInt(2)},
			{big.NewInt(2)},
		},
		[][]channel.Bal{
			{big.NewInt(1)},
			{big.NewInt(2)},
			{big.NewInt(3)},
		},
		[][]channel.Bal{
			{big.NewInt(2)},
			{big.NewInt(2)},
			{big.NewInt(2)},
		},
	},
	{
		"1party/three assets/invalid deposit/zero balance",
		*part1,
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

type twoPartyTestcase struct {
	name           string
	parties        []wallet.Address
	recipientIdxs  []channel.Index
	numAssets      int
	initLedgerBals [][]channel.Bal // init ledger balances for each addr
	initChanBals   [][]channel.Bal // init ledger balances for each addr
	proposedChan   [][]channel.Bal // intention of fundingReq
	wantLedger     [][]channel.Bal // reality for ledger after deposit
	wantChan       [][]channel.Bal // reality for chan after deposit
}

//nolint:dupl // explicitly stated table tests are easier for debugging.
var twoPartyTable = []twoPartyTestcase{
	{
		"2parties/three assets/valid & no deposit",
		[]wallet.Address{part1, part2},
		[]channel.Index{channel.Index(0), channel.Index(1)},
		3,
		[][]channel.Bal{
			{big.NewInt(1), big.NewInt(4)},
			{big.NewInt(2), big.NewInt(5)},
			{big.NewInt(3), big.NewInt(6)},
		},
		[][]channel.Bal{
			{big.NewInt(0), big.NewInt(0)},
			{big.NewInt(0), big.NewInt(0)},
			{big.NewInt(0), big.NewInt(0)},
		},
		[][]channel.Bal{
			{big.NewInt(1), big.NewInt(0)},
			{big.NewInt(2), big.NewInt(0)},
			{big.NewInt(3), big.NewInt(0)},
		},
		[][]channel.Bal{
			{big.NewInt(0), big.NewInt(4)},
			{big.NewInt(0), big.NewInt(5)},
			{big.NewInt(0), big.NewInt(6)},
		},
		[][]channel.Bal{
			{big.NewInt(1), big.NewInt(0)},
			{big.NewInt(2), big.NewInt(0)},
			{big.NewInt(3), big.NewInt(0)},
		},
	},
	{
		"2parties/three assets/no & valid deposit",
		[]wallet.Address{part1, part2},
		[]channel.Index{channel.Index(0), channel.Index(1)},
		3,
		[][]channel.Bal{
			{big.NewInt(1), big.NewInt(4)},
			{big.NewInt(2), big.NewInt(5)},
			{big.NewInt(3), big.NewInt(6)},
		},
		[][]channel.Bal{
			{big.NewInt(0), big.NewInt(0)},
			{big.NewInt(0), big.NewInt(0)},
			{big.NewInt(0), big.NewInt(0)},
		},
		[][]channel.Bal{
			{big.NewInt(0), big.NewInt(4)},
			{big.NewInt(0), big.NewInt(5)},
			{big.NewInt(0), big.NewInt(6)},
		},
		[][]channel.Bal{
			{big.NewInt(1), big.NewInt(0)},
			{big.NewInt(2), big.NewInt(0)},
			{big.NewInt(3), big.NewInt(0)},
		},
		[][]channel.Bal{
			{big.NewInt(0), big.NewInt(4)},
			{big.NewInt(0), big.NewInt(5)},
			{big.NewInt(0), big.NewInt(6)},
		},
	},
	{
		"2parties/three assets/valid & invalid deposit",
		[]wallet.Address{part1, part2},
		[]channel.Index{channel.Index(0), channel.Index(1)},
		3,
		[][]channel.Bal{
			{big.NewInt(1), big.NewInt(4)},
			{big.NewInt(2), big.NewInt(5)},
			{big.NewInt(3), big.NewInt(6)},
		},
		[][]channel.Bal{
			{big.NewInt(0), big.NewInt(0)},
			{big.NewInt(0), big.NewInt(0)},
			{big.NewInt(0), big.NewInt(0)},
		},
		[][]channel.Bal{
			{big.NewInt(1), big.NewInt(420)},
			{big.NewInt(2), big.NewInt(420)},
			{big.NewInt(3), big.NewInt(420)},
		},
		[][]channel.Bal{
			{big.NewInt(0), big.NewInt(4)},
			{big.NewInt(0), big.NewInt(5)},
			{big.NewInt(0), big.NewInt(6)},
		},
		[][]channel.Bal{
			{big.NewInt(1), big.NewInt(0)},
			{big.NewInt(2), big.NewInt(0)},
			{big.NewInt(3), big.NewInt(0)},
		},
	},
	{
		"2parties/three assets/invalid & valid deposit",
		[]wallet.Address{part1, part2},
		[]channel.Index{channel.Index(0), channel.Index(1)},
		3,
		[][]channel.Bal{
			{big.NewInt(1), big.NewInt(4)},
			{big.NewInt(2), big.NewInt(5)},
			{big.NewInt(3), big.NewInt(6)},
		},
		[][]channel.Bal{
			{big.NewInt(0), big.NewInt(0)},
			{big.NewInt(0), big.NewInt(0)},
			{big.NewInt(0), big.NewInt(0)},
		},
		[][]channel.Bal{
			{big.NewInt(420), big.NewInt(4)},
			{big.NewInt(420), big.NewInt(5)},
			{big.NewInt(420), big.NewInt(6)},
		},
		[][]channel.Bal{
			{big.NewInt(1), big.NewInt(0)},
			{big.NewInt(2), big.NewInt(0)},
			{big.NewInt(3), big.NewInt(0)},
		},
		[][]channel.Bal{
			{big.NewInt(0), big.NewInt(4)},
			{big.NewInt(0), big.NewInt(5)},
			{big.NewInt(0), big.NewInt(6)},
		},
	},
	{
		"2parties/three assets/valid & valid deposit",
		[]wallet.Address{part1, part2},
		[]channel.Index{channel.Index(0), channel.Index(1)},
		3,
		[][]channel.Bal{
			{big.NewInt(1), big.NewInt(4)},
			{big.NewInt(2), big.NewInt(5)},
			{big.NewInt(3), big.NewInt(6)},
		},
		[][]channel.Bal{
			{big.NewInt(0), big.NewInt(0)},
			{big.NewInt(0), big.NewInt(0)},
			{big.NewInt(0), big.NewInt(0)},
		},
		[][]channel.Bal{
			{big.NewInt(1), big.NewInt(4)},
			{big.NewInt(2), big.NewInt(5)},
			{big.NewInt(3), big.NewInt(6)},
		},
		[][]channel.Bal{
			{big.NewInt(0), big.NewInt(0)},
			{big.NewInt(0), big.NewInt(0)},
			{big.NewInt(0), big.NewInt(0)},
		},
		[][]channel.Bal{
			{big.NewInt(1), big.NewInt(4)},
			{big.NewInt(2), big.NewInt(5)},
			{big.NewInt(3), big.NewInt(6)},
		},
	},
	{
		"2parties/three assets/invalid & invalid deposit",
		[]wallet.Address{part1, part2},
		[]channel.Index{channel.Index(0), channel.Index(1)},
		3,
		[][]channel.Bal{
			{big.NewInt(1), big.NewInt(4)},
			{big.NewInt(2), big.NewInt(5)},
			{big.NewInt(3), big.NewInt(6)},
		},
		[][]channel.Bal{
			{big.NewInt(0), big.NewInt(0)},
			{big.NewInt(0), big.NewInt(0)},
			{big.NewInt(0), big.NewInt(0)},
		},
		[][]channel.Bal{
			{big.NewInt(420), big.NewInt(420)},
			{big.NewInt(420), big.NewInt(420)},
			{big.NewInt(420), big.NewInt(420)},
		},
		[][]channel.Bal{
			{big.NewInt(1), big.NewInt(4)},
			{big.NewInt(2), big.NewInt(5)},
			{big.NewInt(3), big.NewInt(6)},
		},
		[][]channel.Bal{
			{big.NewInt(0), big.NewInt(0)},
			{big.NewInt(0), big.NewInt(0)},
			{big.NewInt(0), big.NewInt(0)},
		},
	},
}
