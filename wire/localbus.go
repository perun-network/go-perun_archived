// Copyright (c) 2020 Chair of Applied Cryptography, Technische Universität
// Darmstadt, Germany. All rights reserved. This file is part of go-perun. Use
// of this source code is governed by the Apache 2.0 license that can be found
// in the LICENSE file.

package wire

import (
	"context"
	"sync"

	"github.com/pkg/errors"

	"perun.network/go-perun/wallet"
)

type localBusReceiver struct {
	recv   Consumer
	exists chan struct{}
}

var _ Bus = (*LocalBus)(nil)

type LocalBus struct {
	mutex sync.RWMutex
	recvs map[wallet.AddrKey]*localBusReceiver
}

// NewLocalBus creates a new local bus, which only targets receivers that lie
// within the same process.
func NewLocalBus() *LocalBus {
	return &LocalBus{recvs: make(map[wallet.AddrKey]*localBusReceiver)}
}

// Publish implements wire.Bus.Publish. It returns only once the recipient
// received the message or the context times out.
func (h *LocalBus) Publish(ctx context.Context, e *Envelope) error {
	recv := h.ensureRecv(e.Recipient)
	select {
	case <-recv.exists:
		recv.recv.Put(e)
		return nil
	case <-ctx.Done():
		return errors.Wrap(ctx.Err(), "publishing message")
	}
}

// Subscribe implements wire.Bus.SubscribeClient. There can only be one
// subscription per receiver address.
func (h *LocalBus) SubscribeClient(c Consumer, receiver Address) error {
	recv := h.ensureRecv(receiver)
	recv.recv = c
	close(recv.exists)

	return nil
}

// ensureRecv ensures that there is an entry for a recipient address in the
// bus' receiver map, and returns it. If it creates a new receiver, it is only
// a placeholder until a subscription appears.
func (h *LocalBus) ensureRecv(a Address) *localBusReceiver {
	key := wallet.Key(a)
	// First, we only use a read lock, hoping that the receiver already exists.
	h.mutex.RLock()
	recv, ok := h.recvs[key]
	h.mutex.RUnlock()

	if ok {
		return recv
	}

	// If not, we have to insert one, so we need exclusive an lock.
	h.mutex.Lock()
	defer h.mutex.Unlock()

	// We need to re-check, because between the RUnlock() and Lock(), it could
	// have been added by another goroutine already.
	recv, ok = h.recvs[key]
	if ok {
		return recv
	}
	// Insert and return the new entry.
	recv = &localBusReceiver{
		recv:   nil,
		exists: make(chan struct{}),
	}
	h.recvs[key] = recv
	return recv
}