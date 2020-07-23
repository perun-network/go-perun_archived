// Copyright (c) 2019 Chair of Applied Cryptography, Technische Universität
// Darmstadt, Germany. All rights reserved. This file is part of go-perun. Use
// of this source code is governed by the Apache 2.0 license that can be found
// in the LICENSE file.

package test

import (
	"context"
	"net"
	"sync/atomic"

	"github.com/pkg/errors"

	"perun.network/go-perun/pkg/sync"
	"perun.network/go-perun/wire"
	wirenet "perun.network/go-perun/wire/net"
)

// Dialer is a test dialer that can dial connections to Listeners via a ConnHub.
type Dialer struct {
	hub    *ConnHub
	dialed int32

	sync.Closer
}

var _ wirenet.Dialer = (*Dialer)(nil)

// Dial tries to connect to a wire.
func (d *Dialer) Dial(ctx context.Context, address wire.Address) (wirenet.Conn, error) {
	if d.IsClosed() {
		return nil, errors.New("dialer closed")
	}

	select {
	case <-ctx.Done():
		return nil, errors.New("manually aborted")
	default:
	}

	l, ok := d.hub.find(address)
	if !ok {
		return nil, errors.Errorf("peer with address %v not found", address)
	}

	local, remote := net.Pipe()
	if !l.Put(ctx, wirenet.NewIoConn(remote)) {
		local.Close()
		remote.Close()
		return nil, errors.New("Put() failed")
	}
	atomic.AddInt32(&d.dialed, 1)
	return wirenet.NewIoConn(local), nil
}

// Close closes a connection.
func (d *Dialer) Close() error {
	return errors.WithMessage(d.Closer.Close(), "dialer was already closed")
}

// NumDialed returns how many peers have been dialed.
func (d *Dialer) NumDialed() int {
	return int(atomic.LoadInt32(&d.dialed))
}
