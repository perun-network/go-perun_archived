// Copyright (c) 2020 Chair of Applied Cryptography, Technische Universität
// Darmstadt, Germany. All rights reserved. This file is part of go-perun. Use
// of this source code is governed by the Apache 2.0 license that can be found
// in the LICENSE file.

package client

import (
	"context"

	"github.com/pkg/errors"

	"perun.network/go-perun/log"
	"perun.network/go-perun/wire"
)

// A clientConn bundles all the messaging infrastructure for a Client.
type clientConn struct {
	*wire.Relay // Client relay, subscribed to the bus. Embedded for methods Subscribe and Cache.
	bus         wire.Bus
	reqRecv     *wire.Receiver // subscription to incoming requests
	sender      wire.Address
	log.Embedding
}

func makeClientConn(address wire.Address, bus wire.Bus) (c clientConn, err error) {
	c.Embedding = log.MakeEmbedding(log.WithField("id", address))
	c.sender = address
	c.bus = bus
	c.Relay = wire.NewRelay()
	defer func() {
		if err != nil {
			if cerr := c.Relay.Close(); cerr != nil {
				err = errors.WithMessagef(err, "(error closing bus: %v)", cerr)
			}
		}
	}()

	c.Relay.SetDefaultMsgHandler(func(m *wire.Envelope) {
		log.Debugf("Received %T message without subscription: %v", m.Msg, m)
	})
	if err := bus.SubscribeClient(c, c.sender); err != nil {
		return c, errors.WithMessage(err, "subscribing client on bus")
	}

	c.reqRecv = wire.NewReceiver()
	if err := c.Subscribe(c.reqRecv, isReqMsg); err != nil {
		return c, errors.WithMessage(err, "subscribing request receiver")
	}

	return c, nil
}

func isReqMsg(m *wire.Envelope) bool {
	return m.Msg.Type() == wire.ChannelProposal ||
		m.Msg.Type() == wire.ChannelUpdate ||
		m.Msg.Type() == wire.ChannelSync
}

func (c clientConn) nextReq(ctx context.Context) (*wire.Envelope, error) {
	return c.reqRecv.Next(ctx)
}

// pubMsg publishes the given message on the wire bus, setting the own client as
// the sender.
func (c *clientConn) pubMsg(ctx context.Context, msg wire.Msg, rec wire.Address) error {
	c.Log().WithField("peer", rec).Debugf("Publishing message: %v", msg)
	return c.bus.Publish(ctx, &wire.Envelope{
		Sender:    c.sender,
		Recipient: rec,
		Msg:       msg,
	})
}

// Publish publishes the message on the bus. Makes clientConn implement the
// wire.Publisher interface.
func (c *clientConn) Publish(ctx context.Context, env *wire.Envelope) error {
	return c.bus.Publish(ctx, env)
}

func (c *clientConn) Close() error {
	err := c.Relay.Close()
	if rerr := c.reqRecv.Close(); err == nil {
		err = errors.WithMessage(rerr, "closing proposal receiver")
	}
	return err
}
