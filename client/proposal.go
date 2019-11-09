// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package client

import (
	"context"
	"sync/atomic"

	"perun.network/go-perun/log"
	"perun.network/go-perun/peer"
	"perun.network/go-perun/wallet"
	wire "perun.network/go-perun/wire/msg"
)

type (
	ProposalHandler interface {
		Handle(*ChannelProposal, *ProposalResponder)
	}

	ProposalResponder struct {
		accept chan ctxProposalAcc
		reject chan ctxProposalRej
		err    chan error // return error
		called int32      // atomically accessed state
	}

	ProposalAcc struct {
		Participant wallet.Account
		// TODO add Funder
		// TODO add UpdateHandler
	}

	// The following type is only needed to bundle the ctx and res of
	// ProposalResponder.Accept() into a single struct so that they can be sent
	// over a channel
	ctxProposalAcc struct {
		ProposalAcc
		ctx context.Context
	}

	// The following type is only needed to bundle the ctx and reason of
	// ProposalResponder.Reject() into a single struct so that they can be sent
	// over a channel
	ctxProposalRej struct {
		ctx    context.Context
		reason string
	}
)

func newProposalResponder() *ProposalResponder {
	return &ProposalResponder{
		accept: make(chan ctxProposalAcc),
		reject: make(chan ctxProposalRej),
		err:    make(chan error),
	}
}

// Accept lets the user signal that they want to accept the channel proposal.
//
// TODO Add channel controller to return values
func (r *ProposalResponder) Accept(ctx context.Context, res ProposalAcc) error {
	if !atomic.CompareAndSwapInt32(&r.called, 0, 1) {
		log.Panic("multiple calls on proposal responder")
	}
	defer r.close()
	r.accept <- ctxProposalAcc{res, ctx}
	// TODO return (*Channel, error) when first version of channel controller is present
	return <-r.err
}

// Reject lets the user signal that they reject the channel proposal.
func (r *ProposalResponder) Reject(ctx context.Context, reason string) error {
	if !atomic.CompareAndSwapInt32(&r.called, 0, 1) {
		log.Panic("multiple calls on proposal responder")
	}
	defer r.close()
	r.reject <- ctxProposalRej{ctx, reason}
	return <-r.err
}

// called by Accept or Reject once one of them has returned
func (r *ProposalResponder) close() {
	close(r.accept)
	close(r.reject)
	close(r.err)
}

func (c *Client) subChannelProposals(p *peer.Peer) {
	proposalReceiver, err := peer.Subscribe(p,
		func(m wire.Msg) bool { return m.Type() == wire.ChannelProposal },
	)
	if err != nil {
		c.logPeer(p).Warnf("failed to subscribe to channel proposals on new peer")
		return
	}

	go func() { <-c.quit; proposalReceiver.Close() }()

	// proposal handler loop
	go func() {
		for {
			_p, m := proposalReceiver.Next(context.Background())
			if _p == nil {
				c.logPeer(p).Debugf("proposal subscription closed")
				return
			}
			proposal := m.(*ChannelProposal) // safe because that's the predicate
			go c.handleChannelProposal(p, proposal)
		}
	}()
}

func (c *Client) handleChannelProposal(p *peer.Peer, proposal *ChannelProposal) {
	responder := newProposalResponder()
	go c.propHandler.Handle(proposal, responder)

	// wait for user response
	select {
	case acc := <-responder.accept:
		msgAccept := &ChannelProposalAcc{
			//			SessID:          proposal.SessID(), // TODO uncomment when !102 is merged
			ParticipantAddr: acc.Participant.Address(),
		}
		if err := p.Send(acc.ctx, msgAccept); err != nil {
			c.logPeer(p).Warn("error sending proposal acceptance")
			responder.err <- err
			return
		}
		// TODO setup channel controller and start it

	case rej := <-responder.reject:
		msgReject := &ChannelProposalRej{
			//			SessID: proposal.SessID(), // TODO uncomment when !102 is merged
			Reason: rej.reason,
		}
		if err := p.Send(rej.ctx, msgReject); err != nil {
			c.logPeer(p).Warn("error sending proposal rejection")
			responder.err <- err
			return
		}
	}
	responder.err <- nil
}
