// Copyright (c) 2020 Chair of Applied Cryptography, Technische Universität
// Darmstadt, Germany. All rights reserved. This file is part of go-perun. Use
// of this source code is governed by the Apache 2.0 license that can be found
// in the LICENSE file.

package net // import "perun.network/go-perun/wire/net"

import (
	"context"

	"perun.network/go-perun/wire"
)

// Dialer is an interface that allows creating a connection to a peer via its
// Perun address. The established connections are not authenticated yet.
type Dialer interface {
	// Dial creates a connection to a peer.
	// The passed context is used to abort the dialing process. The returned
	// connection might not belong to the requested address.
	//
	// Dial needs to be reentrant, and concurrent calls to Close() must abort
	// any ongoing Dial() calls.
	Dial(ctx context.Context, addr wire.Address) (Conn, error)
	// Close aborts any ongoing calls to Dial().
	//
	// Close() needs to be reentrant, and repeated calls to Close() need to
	// return an error.
	Close() error
}
