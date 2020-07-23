// Copyright (c) 2020 Chair of Applied Cryptography, Technische Universität
// Darmstadt, Germany. All rights reserved. This file is part of go-perun. Use
// of this source code is governed by the Apache 2.0 license that can be found
// in the LICENSE file.

package net

import (
	"context"
	"math/rand"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	pkgtest "perun.network/go-perun/pkg/test"
	wallettest "perun.network/go-perun/wallet/test"
	"perun.network/go-perun/wire"
	wiretest "perun.network/go-perun/wire/test"
)

func TestExchangeAddrs_ConnFail(t *testing.T) {
	rng := rand.New(rand.NewSource(0xDDDDDEDE))
	a, _ := newPipeConnPair()
	a.Close()
	addr, err := ExchangeAddrsPassive(context.Background(), wallettest.NewRandomAccount(rng), a)
	assert.Nil(t, addr)
	assert.Error(t, err)
}

func TestExchangeAddrs_Success(t *testing.T) {
	rng := rand.New(rand.NewSource(0xfedd))
	conn0, conn1 := newPipeConnPair()
	defer conn0.Close()
	account0, account1 := wallettest.NewRandomAccount(rng), wallettest.NewRandomAccount(rng)
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		defer conn1.Close()

		recvAddr0, err := ExchangeAddrsPassive(context.Background(), account1, conn1)
		assert.NoError(t, err)
		assert.True(t, recvAddr0.Equals(account0.Address()))
	}()

	err := ExchangeAddrsActive(context.Background(), account0, account1.Address(), conn0)
	assert.NoError(t, err)

	wg.Wait()
}

func TestExchangeAddrs_Timeout(t *testing.T) {
	rng := rand.New(rand.NewSource(0xDDDDDeDe))
	a, _ := newPipeConnPair()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	pkgtest.AssertTerminates(t, 2*timeout, func() {
		addr, err := ExchangeAddrsPassive(ctx, wallettest.NewRandomAccount(rng), a)
		assert.Nil(t, addr)
		assert.Error(t, err)
	})
}

func TestExchangeAddrs_BogusMsg(t *testing.T) {
	rng := rand.New(rand.NewSource(0xcafe))
	acc := wallettest.NewRandomAccount(rng)
	conn := newMockConn(nil)
	conn.recvQueue <- wiretest.NewRandomEnvelope(rng, wire.NewPingMsg())
	addr, err := ExchangeAddrsPassive(context.Background(), acc, conn)

	assert.Error(t, err, "ExchangeAddrs should error when peer sends a non-AuthResponseMsg")
	assert.Nil(t, addr)
}
