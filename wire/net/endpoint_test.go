// Copyright (c) 2019 Chair of Applied Cryptography, Technische Universität
// Darmstadt, Germany. All rights reserved. This file is part of go-perun. Use
// of this source code is governed by the Apache 2.0 license that can be found
// in the LICENSE file.

package net

import (
	"context"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	_ "perun.network/go-perun/backend/sim" // backend init
	"perun.network/go-perun/pkg/test"
	wallettest "perun.network/go-perun/wallet/test"
	"perun.network/go-perun/wire"
	wiretest "perun.network/go-perun/wire/test"
)

// setup is a test setup consisting of two connected peers.
// It is also a mock dialer.
type setup struct {
	mutex  sync.RWMutex
	closed bool
	alice  *client
	bob    *client
}

// makeSetup creates a test setup.
func makeSetup(t *testing.T) *setup {
	a, b := newPipeConnPair()
	rng := rand.New(rand.NewSource(0xb0baFEDD))
	// We need the setup address when constructing the clients.
	s := new(setup)
	*s = setup{
		alice: makeClient(t, a, rng, s),
		bob:   makeClient(t, b, rng, s),
	}
	return s
}

// Dial simulates creating a connection to a
func (s *setup) Dial(ctx context.Context, addr wire.Address) (Conn, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.closed {
		return nil, errors.New("dialer closed")
	}

	// a: Alice's end, b: Bob's end.
	a, b := newPipeConnPair()

	if addr.Equals(s.alice.endpoint.Address) { // Dialing Bob?
		s.bob.Registry.addEndpoint(s.bob.endpoint.Address, b, true) // Bob accepts connection.
		return a, nil
	} else if addr.Equals(s.bob.endpoint.Address) { // Dialing Alice?
		s.alice.Registry.addEndpoint(s.alice.endpoint.Address, a, true) // Alice accepts connection.
		return b, nil
	} else {
		return nil, errors.New("unknown peer")
	}
}

func (s *setup) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.closed {
		return errors.New("dialer closed")
	}
	s.closed = true
	return nil
}

// client is a simulated client in the test setup.
// All of the client's incoming messages can be read from its receiver.
type client struct {
	endpoint *Endpoint
	Registry *EndpointRegistry
	*wire.Receiver
}

// makeClient creates a simulated test client.
func makeClient(t *testing.T, conn Conn, rng *rand.Rand, dialer Dialer) *client {
	var receiver = wire.NewReceiver()
	var registry = NewEndpointRegistry(wallettest.NewRandomAccount(rng), func(wire.Address) wire.Consumer {
		return receiver
	}, dialer)

	return &client{
		endpoint: registry.addEndpoint(wallettest.NewRandomAddress(rng), conn, true),
		Registry: registry,
		Receiver: receiver,
	}
}

// TestEndpoint_Close tests that closing a peer will make the peer object unusable.
func TestEndpoint_Close(t *testing.T) {
	t.Parallel()
	s := makeSetup(t)
	// Remember bob's address for later, we will need it for a registry lookup.
	bobAddr := s.alice.endpoint.Address
	// The lookup needs to work because the test relies on it.
	found := s.alice.Registry.find(bobAddr)
	assert.Equal(t, s.alice.endpoint, found)
	// Close Alice's connection to Bob.
	assert.NoError(t, s.alice.endpoint.Close(), "closing a peer once must succeed")
	assert.Error(t, s.alice.endpoint.Close(), "closing peers twice must fail")

	// Sending over closed peers (not connections) must fail.
	err := s.alice.endpoint.Send(
		context.Background(),
		wiretest.NewRandomEnvelope(test.Prng(t), wire.NewPingMsg()))
	assert.Error(t, err, "sending to bob must fail", err)
}

func TestEndpoint_Send_ImmediateAbort(t *testing.T) {
	t.Parallel()
	s := makeSetup(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// This operation should abort immediately.
	assert.Error(t, s.alice.endpoint.Send(ctx,
		wiretest.NewRandomEnvelope(test.Prng(t), wire.NewPingMsg())))

	assert.Error(t, s.alice.endpoint.Close(),
		"peer must be closed after failed sending")
}

func TestEndpoint_Send_Timeout(t *testing.T) {
	t.Parallel()
	rng := test.Prng(t)
	conn, _ := newPipeConnPair()
	p := newEndpoint(nil, conn)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	assert.Error(t, p.Send(ctx, wiretest.NewRandomEnvelope(rng, wire.NewPingMsg())),
		"Send() must timeout on blocked connection")
	assert.Error(t, p.Close(),
		"peer must be closed after failed Send()")
}

func TestEndpoint_Send_Timeout_Mutex_TryLockCtx(t *testing.T) {
	t.Parallel()
	rng := test.Prng(t)
	conn, remote := newPipeConnPair()
	p := newEndpoint(nil, conn)

	go remote.Recv()
	p.sending.Lock()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	assert.Error(t, p.Send(ctx, wiretest.NewRandomEnvelope(rng, wire.NewPingMsg())),
		"Send() must timeout on locked mutex")
	assert.Error(t, p.Close(),
		"peer must be closed after failed Send()")
}

func TestEndpoint_Send_Close(t *testing.T) {
	t.Parallel()
	rng := test.Prng(t)
	conn, _ := newPipeConnPair()
	p := newEndpoint(nil, conn)

	go func() {
		<-time.NewTimer(timeout).C
		p.Close()
	}()

	assert.Error(t, p.Send(context.Background(), wiretest.NewRandomEnvelope(rng, wire.NewPingMsg())),
		"Send() must be aborted by Close()")
}

// TestEndpoint_ClosedByRecvLoopOnConnClose is a regression test for
// #181 `peer.Peer` does not handle connection termination properly
func TestEndpoint_ClosedByRecvLoopOnConnClose(t *testing.T) {
	t.Parallel()
	eofReceived := make(chan struct{})

	rng := rand.New(rand.NewSource(0xcaffe2))
	addr := wallettest.NewRandomAddress(rng)
	conn0, conn1 := newPipeConnPair()
	peer := newEndpoint(addr, conn0)

	go func() {
		peer.recvLoop(nil)
		close(eofReceived)
	}()

	conn1.Close()
	<-eofReceived

	assert.Error(t, peer.Close())
}
