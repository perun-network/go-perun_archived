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
	"context"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
	ethchannel "perun.network/go-perun/backend/ethereum/channel"
	"perun.network/go-perun/backend/ethereum/channel/test"
	ethwallet "perun.network/go-perun/backend/ethereum/wallet"
	ethwtest "perun.network/go-perun/backend/ethereum/wallet/test"
	"perun.network/go-perun/channel"
	"perun.network/go-perun/client"
	clienttest "perun.network/go-perun/client/test"
	pkgtest "perun.network/go-perun/pkg/test"
	"perun.network/go-perun/wallet"
	"perun.network/go-perun/wire"
)

func TestProgression(t *testing.T) {
	rng := pkgtest.Prng(t)

	names := [2]string{"Paul", "Paula"}
	backendSetup := test.NewSetup(t, rng, 2)
	roleSetups := makeRoleSetups(backendSetup, names)
	clients := [2]clienttest.Executer{
		clienttest.NewPaul(t, roleSetups[0]),
		clienttest.NewPaula(t, roleSetups[1]),
	}

	appAddress := deployMockApp(t, backendSetup)
	app := channel.NewMockApp(appAddress)
	channel.RegisterApp(app)

	execConfig := &clienttest.ProgressionExecConfig{
		BaseExecConfig: clienttest.MakeBaseExecConfig(
			clientAddresses(roleSetups),
			(*ethwallet.Address)(&backendSetup.Asset),
			[2]*big.Int{big.NewInt(99), big.NewInt(1)},
			client.WithApp(app, channel.NewMockOp(channel.OpValid)),
		),
	}

	clienttest.ExecuteTwoPartyTest(t, clients, execConfig)
}

func makeRoleSetups(s *test.Setup, names [2]string) (setup [2]clienttest.RoleSetup) {
	bus := wire.NewLocalBus()
	for i := 0; i < len(setup); i++ {
		setup[i] = clienttest.RoleSetup{
			Name:        names[i],
			Identity:    s.Accs[i],
			Bus:         bus,
			Funder:      s.Funders[i],
			Adjudicator: s.Adjs[i],
			Wallet:      ethwtest.NewTmpWallet(),
			Timeout:     defaultTimeout,
		}
	}
	return
}

func deployMockApp(t *testing.T, s *test.Setup) wallet.Address {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	addr, err := ethchannel.DeployTrivialApp(ctx, *s.CB, s.TxSender.Account)
	require.NoError(t, err)
	return ethwallet.AsWalletAddr(addr)
}

func clientAddresses(roleSetups [2]clienttest.RoleSetup) (addresses [2]wire.Address) {
	for i := 0; i < len(roleSetups); i++ {
		addresses[i] = roleSetups[i].Identity.Address()
	}
	return
}
