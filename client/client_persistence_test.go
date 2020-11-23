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

package client_test

import (
	"math/big"
	"math/rand"
	"testing"

	"perun.network/go-perun/apps/payment"
	chprtest "perun.network/go-perun/channel/persistence/test"
	chtest "perun.network/go-perun/channel/test"
	"perun.network/go-perun/client"
	ctest "perun.network/go-perun/client/test"
	"perun.network/go-perun/pkg/test"
	"perun.network/go-perun/wire"
)

func TestPersistencePetraRobert(t *testing.T) {
	rng := test.Prng(t)
	for i := 0; i < 2; i++ {
		setups := NewSetupsPersistence(t, rng, []string{"Petra", "Robert"})
		roles := [2]ctest.Executer{
			ctest.NewPetra(setups[0], t),
			ctest.NewRobert(setups[1], t),
		}

		app := client.WithoutApp()
		if i == 1 {
			app = client.WithApp(
				chtest.NewRandomAppAndData(rng, chtest.WithAppRandomizer(new(payment.Randomizer))),
			)
		}

		cfg := &ctest.PetraRobertExecConfig{
			BaseExecConfig: ctest.MakeBaseExecConfig(
				[2]wire.Address{setups[0].Identity.Address(), setups[1].Identity.Address()},
				chtest.NewRandomAsset(rng),
				[2]*big.Int{big.NewInt(100), big.NewInt(100)},
				app,
			),
			NumPayments: [2]int{2, 2},
			TxAmounts:   [2]*big.Int{big.NewInt(5), big.NewInt(3)},
		}

		ctest.ExecuteTwoPartyTest(t, roles, cfg)
	}
}

func NewSetupsPersistence(t *testing.T, rng *rand.Rand, names []string) []ctest.RoleSetup {
	setups := NewSetups(rng, names)
	for i := range names {
		setups[i].PR = chprtest.NewPersistRestorer(t)
	}
	return setups
}
