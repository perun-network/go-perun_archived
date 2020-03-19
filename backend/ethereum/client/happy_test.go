// Copyright (c) 2019 Chair of Applied Cryptography, Technische Universität
// Darmstadt, Germany. All rights reserved. This file is part of go-perun. Use
// of this source code is governed by a MIT-style license that can be found in
// the LICENSE file.

package client_test

import (
	"context"
	"math/big"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"perun.network/go-perun/backend/ethereum/channel/test"
	"perun.network/go-perun/backend/ethereum/wallet"
	clienttest "perun.network/go-perun/client/test"
	"perun.network/go-perun/log"
	"perun.network/go-perun/peer"
	peertest "perun.network/go-perun/peer/test"
)

var defaultTimeout = 5 * time.Second

func TestHappyAliceBob(t *testing.T) {
	log.Info("Starting happy test")
	rng := rand.New(rand.NewSource(0x1337))

	const A, B = 0, 1 // Indices of Alice and Bob
	var (
		name  = [2]string{"Alice", "Bob"}
		hub   peertest.ConnHub
		setup [2]clienttest.RoleSetup
		role  [2]clienttest.Executer
	)

	s := test.NewSetup(t, rng, 2)
	for i := 0; i < 2; i++ {
		setup[i] = clienttest.RoleSetup{
			Name:        name[i],
			Identity:    s.Accs[i],
			Dialer:      hub.NewDialer(),
			Listener:    hub.NewListener(s.Accs[i].Address()),
			Funder:      s.Funders[i],
			Adjudicator: s.Adjs[i],
			Timeout:     defaultTimeout,
		}
	}

	role[A] = clienttest.NewAlice(setup[A], t)
	role[B] = clienttest.NewBob(setup[B], t)
	// enable stages synchronization
	stages := role[A].EnableStages()
	role[B].SetStages(stages)

	execConfig := clienttest.ExecConfig{
		PeerAddrs:  [2]peer.Address{s.Accs[A].Address(), s.Accs[B].Address()},
		InitBals:   [2]*big.Int{big.NewInt(100), big.NewInt(100)},
		Asset:      &wallet.Address{Address: s.Asset},
		NumUpdates: [2]int{2, 2},
		TxAmounts:  [2]*big.Int{big.NewInt(5), big.NewInt(3)},
	}

	var wg sync.WaitGroup
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func(i int) {
			defer wg.Done()
			log.Infof("Starting %s.Execute", name[i])
			role[i].Execute(execConfig)
		}(i)
	}

	wg.Wait()

	// Assert correct final balances
	aliceToBob := big.NewInt(int64(execConfig.NumUpdates[A])*execConfig.TxAmounts[A].Int64() -
		int64(execConfig.NumUpdates[B])*execConfig.TxAmounts[B].Int64())
	finalBalAlice := new(big.Int).Sub(execConfig.InitBals[A], aliceToBob)
	finalBalBob := new(big.Int).Add(execConfig.InitBals[B], aliceToBob)
	// reset context timeout
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	assertBal := func(addr common.Address, bal *big.Int) {
		b, err := s.SimBackend.BalanceAt(ctx, addr, nil)
		require.NoError(t, err)
		assert.Zero(t, bal.Cmp(b), "ETH balance mismatch")
	}

	assertBal(s.Recvs[A].Address, finalBalAlice)
	assertBal(s.Recvs[B].Address, finalBalBob)

	log.Info("Happy test done")
}
