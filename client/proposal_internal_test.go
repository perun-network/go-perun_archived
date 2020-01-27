// Copyright (c) 2019 Chair of Applied Cryptography, Technische Universität
// Darmstadt, Germany. All rights reserved. This file is part of go-perun. Use
// of this source code is governed by a MIT-style license that can be found in
// the LICENSE file.

package client

import (
	"context"
	"math/big"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	channeltest "perun.network/go-perun/channel/test"
	"perun.network/go-perun/peer"
	peertest "perun.network/go-perun/peer/test"
	"perun.network/go-perun/wallet"
	wallettest "perun.network/go-perun/wallet/test"
)

func TestClient_ProposeChannel_InvalidProposal(t *testing.T) {
	rng := rand.New(rand.NewSource(0x20200123a))
	account := wallettest.NewRandomAccount(rng)
	proposal := newRandomValidChannelProposalReq(rng, 2).AsProp(account)
	invalidProposal := proposal
	invalidProposal.ChallengeDuration = 0
	connHub := new(peertest.ConnHub)
	c := New(
		wallettest.NewRandomAccount(rng),
		connHub.NewDialer(),
		new(DummyProposalHandler),
		new(DummyFunder),
		new(DummySettler),
	)

	_, err := c.ProposeChannel(context.Background(), invalidProposal)
	assert.Error(t, err)
}

// BooleanProposalHandler is a proposal handler which can be configured to
// reject all or accept all proposals.
type BooleanProposalHandler struct {
	t               *testing.T
	ctx             context.Context
	acceptProposals bool
	partAccount     wallet.Account
	done            chan struct{}
}

var _ ProposalHandler = (*BooleanProposalHandler)(nil)

func NewBooleanHandler(
	t *testing.T, rng *rand.Rand, ctx context.Context, acceptProposals bool) *BooleanProposalHandler {
	return &BooleanProposalHandler{
		t:               t,
		ctx:             ctx,
		acceptProposals: acceptProposals,
		partAccount:     wallettest.NewRandomAccount(rng),
		done:            make(chan struct{}),
	}
}

func (h *BooleanProposalHandler) Handle(
	proposal *ChannelProposalReq, responder *ProposalResponder) {
	assert.NoError(h.t, proposal.Valid())

	if h.acceptProposals {
		msgAccept := &ChannelProposalAcc{
			SessID:          proposal.SessID(),
			ParticipantAddr: h.partAccount.Address(),
		}
		assert.NoError(h.t, responder.peer.Send(h.ctx, msgAccept))
	} else {
		assert.NoError(h.t, responder.Reject(h.ctx, "rejection reason"))
	}

	close(h.done)
}

func TestClient_exchangeTwoPartyProposal(t *testing.T) {
	rng := rand.New(rand.NewSource(0x20200123b))
	timeout := time.Duration(1 * time.Second)
	connHub := new(peertest.ConnHub)
	client0 := New(
		wallettest.NewRandomAccount(rng),
		connHub.NewDialer(),
		new(DummyProposalHandler),
		new(DummyFunder),
		new(DummySettler),
	)
	defer client0.Close()

	proposal := newRandomValidChannelProposalReq(rng, 2)
	proposal.PeerAddrs[0] = client0.id.Address()

	t.Run("accept-proposal", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		proposalHandler := NewBooleanHandler(t, rng, ctx, true)
		client1 := New(
			wallettest.NewRandomAccount(rng),
			connHub.NewDialer(),
			proposalHandler,
			new(DummyFunder),
			new(DummySettler),
		)
		defer client1.Close()

		proposal.PeerAddrs[1] = client1.id.Address()

		listener := connHub.NewListener(client1.id.Address())
		go client1.Listen(listener)

		addresses, err := client0.exchangeTwoPartyProposal(ctx, proposal)
		assert.NoError(t, err)
		require.Equal(t, len(proposal.PeerAddrs), len(addresses))
		assert.Equal(t, proposal.ParticipantAddr, addresses[0])
		assert.Equal(t, proposalHandler.partAccount.Address(), addresses[1])

		<-proposalHandler.done
	})

	t.Run("reject-proposal", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		proposalHandler := NewBooleanHandler(t, rng, ctx, false)
		client1 := New(
			wallettest.NewRandomAccount(rng),
			connHub.NewDialer(),
			proposalHandler,
			new(DummyFunder),
			new(DummySettler),
		)
		defer client1.Close()

		proposal.PeerAddrs[1] = client1.id.Address()

		listener := connHub.NewListener(client1.id.Address())
		go client1.Listen(listener)

		invalidProposal := *proposal
		invalidProposal.ChallengeDuration = 0
		addresses, err := client0.exchangeTwoPartyProposal(ctx, proposal)
		assert.Nil(t, addresses)
		assert.Error(t, err)
	})
}

func TestClient_validTwoPartyProposal(t *testing.T) {
	rng := rand.New(rand.NewSource(0xdeadbeef))

	// dummy client that only has an id
	c := &Client{
		id: wallettest.NewRandomAccount(rng),
	}
	validProp := *newRandomValidChannelProposalReq(rng, 2)
	validProp.PeerAddrs[0] = c.id.Address() // set us as the proposer
	peerAddr := validProp.PeerAddrs[1]      // peer at 1 as receiver
	require.False(t, peerAddr.Equals(c.id.Address()))
	require.Len(t, validProp.PeerAddrs, 2)

	validProp3Peers := *newRandomValidChannelProposalReq(rng, 3)
	invalidProp := validProp          // shallow copy
	invalidProp.ChallengeDuration = 0 // invalidate

	tests := []struct {
		prop     *ChannelProposalReq
		ourIdx   int
		peerAddr wallet.Address
		valid    bool
	}{
		{
			&validProp,
			0, peerAddr, true,
		},
		// test all three invalid combinations of peer address, index
		{
			&validProp,
			1, peerAddr, false, // wrong ourIdx
		},
		{
			&validProp,
			0, c.id.Address(), false, // wrong peerAddr (ours)
		},
		{
			&validProp,
			1, c.id.Address(), false, // wrong index, wrong peer address
		},
		{
			&validProp3Peers, // valid proposal but three peers
			0, peerAddr, false,
		},
		{
			&invalidProp, // invalid proposal, correct other params
			0, peerAddr, false,
		},
	}

	for i, tt := range tests {
		valid := c.validTwoPartyProposal(tt.prop, tt.ourIdx, tt.peerAddr)
		if tt.valid && valid != nil {
			t.Errorf("[%d] Exptected proposal to be valid but got: %v", i, valid)
		} else if !tt.valid && valid == nil {
			t.Errorf("[%d] Exptected proposal to be invalid", i)
		}
	}
}

func newRandomValidChannelProposalReq(rng *rand.Rand, numPeers int) *ChannelProposalReq {
	peerAddrs := make([]peer.Address, numPeers)
	for i := 0; i < numPeers; i++ {
		peerAddrs[i] = wallettest.NewRandomAddress(rng)
	}
	data := channeltest.NewRandomData(rng)
	alloc := channeltest.NewRandomAllocation(rng, numPeers)
	alloc.Locked = nil // make valid InitBals
	participantAddr := wallettest.NewRandomAddress(rng)
	return &ChannelProposalReq{
		ChallengeDuration: rng.Uint64(),
		Nonce:             big.NewInt(rng.Int63()),
		ParticipantAddr:   participantAddr,
		AppDef:            channeltest.NewRandomApp(rng).Def(),
		InitData:          data,
		InitBals:          alloc,
		PeerAddrs:         peerAddrs,
	}
}
