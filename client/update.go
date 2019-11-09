// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package client

import (
	"context"
	"sync/atomic"

	"perun.network/go-perun/channel"
	"perun.network/go-perun/log"
)

type (
	UpdateHandler interface {
		Handle(ChannelUpdate, *UpdateResponder)
	}

	UpdateResponder struct {
		accept chan ctxUpdateAcc
		reject chan ctxUpdateRej
		err    chan error // return error
		called int32      // atomically accessed state
	}

	// The following type is only needed to bundle the ctx and sig of
	// UpdateResponder.Accept() into a single struct so that they can be sent over
	// a channel
	ctxUpdateAcc struct {
		ctx context.Context
		sig channel.Sig
	}

	// The following type is only needed to bundle the ctx and channel update
	// rejection of UpdateResponder.Reject() into a single struct so that they can
	// be sent over a channel
	ctxUpdateRej struct {
		ctx context.Context
		ChannelUpdateRej
	}
)

func newUpdateResponder() *UpdateResponder {
	return &UpdateResponder{
		accept: make(chan ctxUpdateAcc),
		reject: make(chan ctxUpdateRej),
		err:    make(chan error),
	}
}

// Accept lets the user signal that they want to accept the channel update.
func (r *UpdateResponder) Accept(ctx context.Context, sig channel.Sig) error {
	if !atomic.CompareAndSwapInt32(&r.called, 0, 1) {
		log.Panic("multiple calls on channel update responder")
	}
	defer r.close()
	r.accept <- ctxUpdateAcc{ctx, sig}
	return <-r.err
}

// Reject lets the user signal that they reject the channel update.
func (r *UpdateResponder) Reject(ctx context.Context, rej ChannelUpdateRej) error {
	if !atomic.CompareAndSwapInt32(&r.called, 0, 1) {
		log.Panic("multiple calls on channel update responder")
	}
	defer r.close()
	r.reject <- ctxUpdateRej{ctx, rej}
	return <-r.err
}

// called by Accept or Reject once one of them has returned
func (r *UpdateResponder) close() {
	close(r.accept)
	close(r.reject)
	close(r.err)
}
