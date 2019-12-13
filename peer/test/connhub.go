// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package test

import (
	gosync "sync"

	"github.com/pkg/errors"

	"perun.network/go-perun/log"
	"perun.network/go-perun/peer"
	"perun.network/go-perun/pkg/sync"
)

// ConnHub is a factory for creating and connecting test dialers and listeners.
type ConnHub struct {
	mutex gosync.RWMutex
	listenerMap
	dialers dialerList

	sync.Closer
}

// NewListener creates a new test listener for the given address.
// Registers the new listener in the hub. Panics if the address was already
// entered or the hub is closed.
func (h *ConnHub) NewListener(addr peer.Address) *Listener {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	if h.IsClosed() {
		panic("ConnHub already closed")
	}

	listener := NewListener()
	if err := h.insert(addr, listener); err != nil {
		panic("double registration")
	}

	// Remove the listener from the hub after it's closed.
	if err := listener.OnClose(func() { h.erase(addr) }); err != nil {
		log.Fatalf(
			"when registering Listener.OnClose() handler for address %v: %v", addr, err)
	}

	return listener
}

// NewDialer creates a new test dialer.
// Registers the new dialer in the hub. Panics if the hub is closed.
func (h *ConnHub) NewDialer() *Dialer {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	if h.IsClosed() {
		panic("ConnHub already closed")
	}

	dialer := &Dialer{hub: h}
	h.dialers.insert(dialer)
	if err := dialer.OnClose(func() { h.dialers.erase(dialer) }); err != nil {
		log.Fatalf(
			"when registering Dialer.OnClose() handler: %v", err)
	}

	return dialer
}

// Close closes the ConnHub and all its listeners.
func (h *ConnHub) Close() (err error) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if err := h.Closer.Close(); err != nil {
		return errors.WithMessage(err, "ConnHub already closed")
	}

	for _, l := range h.clear() {
		if cerr := l.value.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}

	for _, d := range h.dialers.clear() {
		if cerr := d.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}

	return
}
