// Copyright (c) 2020 Chair of Applied Cryptography, Technische Universität
// Darmstadt, Germany. All rights reserved. This file is part of go-perun. Use
// of this source code is governed by the Apache 2.0 license that can be found
// in the LICENSE file.

package simple

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	simwallet "perun.network/go-perun/backend/sim/wallet"
	"perun.network/go-perun/pkg/test"
	"perun.network/go-perun/wallet"
	"perun.network/go-perun/wire"
)

func TestNewTCPDialer(t *testing.T) {
	d := NewTCPDialer(0)
	assert.Equal(t, d.network, "tcp")
}

func TestNewUnixDialer(t *testing.T) {
	d := NewUnixDialer(0)
	assert.Equal(t, d.network, "unix")
}

func TestDialer_Register(t *testing.T) {
	rng := rand.New(rand.NewSource(0xDDDDdede))
	addr := simwallet.NewRandomAddress(rng)
	key := wallet.Key(addr)
	d := NewTCPDialer(0)

	_, ok := d.get(key)
	require.False(t, ok)

	d.Register(addr, "host")

	host, ok := d.get(key)
	assert.True(t, ok)
	assert.Equal(t, host, "host")
}

func TestDialer_Dial(t *testing.T) {
	timeout := 100 * time.Millisecond
	rng := rand.New(rand.NewSource(0xDDDDdede))
	lhost := "127.0.0.1:7357"
	laddr := simwallet.NewRandomAddress(rng)

	l, err := NewTCPListener(lhost)
	require.NoError(t, err)
	defer l.Close()

	d := NewTCPDialer(timeout)
	d.Register(laddr, lhost)
	daddr := simwallet.NewRandomAddress(rng)
	defer d.Close()

	t.Run("happy", func(t *testing.T) {
		e := &wire.Envelope{
			Sender:    daddr,
			Recipient: laddr,
			Msg:       wire.NewPingMsg()}
		ct := test.NewConcurrent(t)
		go ct.Stage("accept", func(rt require.TestingT) {
			conn, err := l.Accept()
			assert.NoError(t, err)
			require.NotNil(rt, conn)

			re, err := conn.Recv()
			assert.NoError(t, err)
			assert.Equal(t, re, e)
		})

		ct.Stage("dial", func(rt require.TestingT) {
			test.AssertTerminates(t, timeout, func() {
				conn, err := d.Dial(context.Background(), laddr)
				assert.NoError(t, err)
				require.NotNil(rt, conn)

				assert.NoError(t, conn.Send(e))
			})
		})

		ct.Wait("dial", "accept")
	})

	t.Run("aborted context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		test.AssertTerminates(t, timeout, func() {
			conn, err := d.Dial(ctx, laddr)
			assert.Nil(t, conn)
			assert.Error(t, err)
		})
	})

	t.Run("unknown host", func(t *testing.T) {
		noHostAddr := simwallet.NewRandomAddress(rng)
		d.Register(noHostAddr, "no such host")

		test.AssertTerminates(t, timeout, func() {
			conn, err := d.Dial(context.Background(), noHostAddr)
			assert.Nil(t, conn)
			assert.Error(t, err)
		})
	})

	t.Run("unknown address", func(t *testing.T) {
		test.AssertTerminates(t, timeout, func() {
			unkownAddr := simwallet.NewRandomAddress(rng)
			conn, err := d.Dial(context.Background(), unkownAddr)
			assert.Error(t, err)
			assert.Nil(t, conn)
		})
	})
}
