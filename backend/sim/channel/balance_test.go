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
	"testing"

	"github.com/stretchr/testify/assert"
	sim "perun.network/go-perun/backend/sim/channel"
	"perun.network/go-perun/channel"
	pkgtest "perun.network/go-perun/pkg/test"
	"perun.network/go-perun/wallet"
	"perun.network/go-perun/wire/test"
)

func TestBalancesClone(t *testing.T) {
	rng := pkgtest.Prng(t)
	bals := newRandomBalances(rng, 4)
	clone := bals.Clone()

	for asset, holdersClone := range clone {
		oHolders, ok := bals[asset]
		assert.True(t, ok, "wrong asset contained in clone")
		for holder, balClone := range holdersClone {
			bal, ok := oHolders[holder]
			assert.True(t, ok, "wrong user contained in clone")
			assert.Zero(t, bal.Cmp(balClone), "wrong account balances for clone")
		}
	}
}

func newRandomBalances(rng *rand.Rand, n int) sim.Balances {
	bals := make(sim.Balances, n)
	for i := 0; i < n; i++ {
		asset := *sim.NewRandomAsset(rng)
		bals[asset] = make(map[wallet.AddrKey]channel.Bal, n)
		for j := 0; j < n; j++ {
			bals[asset][wallet.Key(test.NewRandomAddress(rng))] = big.NewInt(rng.Int63())
		}
	}
	return bals
}
