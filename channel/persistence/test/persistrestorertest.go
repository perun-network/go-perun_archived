// Copyright 2020 - See NOTICE file for copyright holders.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package test

import (
	"context"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"perun.network/go-perun/channel"
	"perun.network/go-perun/channel/persistence"
	"perun.network/go-perun/log"
	pkgtest "perun.network/go-perun/pkg/test"
	wtest "perun.network/go-perun/wallet/test"
	"perun.network/go-perun/wire"
)

// Client is a mock client that can be used to create channels.
type Client struct {
	addr wire.Address

	rng *rand.Rand
	pr  persistence.PersistRestorer
	ctx context.Context
}

// NewClient creates a client.
func NewClient(ctx context.Context, t *testing.T, rng *rand.Rand, pr persistence.PersistRestorer) *Client {
	return &Client{
		addr: wtest.NewRandomAddress(rng),
		rng:  rng,
		pr:   pr,
		ctx:  ctx,
	}
}

// NewChannel creates a new channel with the supplied peer as the other
// participant. The client's participant index is randomly chosen.
func (c *Client) NewChannel(t require.TestingT, p wire.Address) *Channel {
	idx := c.rng.Intn(2)
	peers := make([]wire.Address, 2)
	peers[idx] = c.addr
	peers[idx^1] = p

	return NewRandomChannel(
		c.ctx,
		t,
		c.pr,
		channel.Index(idx),
		peers,
		c.rng)
}

// GenericPersistRestorerTest tests a PersistRestorer by persisting 2-party
// channels and then asserting equality of the restored channels. pr must be
// fresh and not contain any previous channels. The parameter numChans controls
// the channels created per wire. numPeers is the number of separate peers to
// generate.
func GenericPersistRestorerTest(
	ctx context.Context,
	t *testing.T,
	rng *rand.Rand,
	pr persistence.PersistRestorer,
	numPeers int,
	numChans int) {
	t.Run("RestoreChannel error", func(t *testing.T) {
		var id channel.ID
		ch, err := pr.RestoreChannel(context.Background(), id)
		assert.Error(t, err)
		assert.Nil(t, ch)
	})

	ct := pkgtest.NewConcurrent(t)
	c := NewClient(ctx, t, rng, pr)
	peers := wtest.NewRandomAddresses(rng, numPeers)

	channels := make([]map[channel.ID]*Channel, numPeers)
	for p := 0; p < numPeers; p++ {
		channels[p] = make(map[channel.ID]*Channel)
		for i := 0; i < numChans; i++ {
			ch := c.NewChannel(t, peers[p])
			channels[p][ch.ID()] = ch
			t.Logf("created channel %d for peer %d", i, p)
		}
	}

	subSeed := rng.Int63()
	iterIdx := 0
	for idx := range peers {
		idx := idx
		for _, ch := range channels[idx] {
			ch := ch
			iterIdx++
			iterIdx := iterIdx
			go ct.StageN("testing", numChans*numPeers, func(t pkgtest.ConcT) {
				chIndex := iterIdx
				log.Error(subSeed)
				seed := pkgtest.Seed("", subSeed, numChans, numPeers, chIndex, ch.ID())
				rng := rand.New(rand.NewSource(seed))

				ch.Init(t, rng)
				ch.SignAll(t)
				ch.EnableInit(t)

				ch.SetFunded(t)

				// Update state
				state1 := ch.State().Clone()
				state1.Version++
				ch.Update(t, state1, ch.Idx())
				ch.DiscardUpdate(t)
				ch.Update(t, state1, ch.Idx())
				ch.SignAll(t)
				ch.EnableUpdate(t)

				// Final state
				statef := ch.State().Clone()
				statef.Version++
				statef.IsFinal = true
				ch.Update(t, statef, ch.Idx()^1)
				ch.SignAll(t)
				ch.EnableFinal(t)

				ch.SetRegistering(t)

				ch.SetRegistered(t)

				ch.SetWithdrawing(t)

				t.BarrierN("withdrawing", numChans*numPeers)
				t.Wait("assertedPeers")

				ch.SetWithdrawn(t)
			})
		}
	}
	ct.Wait("withdrawing")
	for pIdx, peer := range peers {
		it, err := pr.RestorePeer(peer)
		require.NoError(t, err)

		for it.Next(ctx) {
			ch := it.Channel()
			cached := channels[pIdx][ch.ID()]
			cached.RequireEqual(t, ch)
		}
	}

	// Test ActivePeers
	persistedPeers, err := pr.ActivePeers(ctx)
	require.NoError(t, err)
	require.Len(t, persistedPeers, numPeers+1) // + local client
peerLoop:
	for idx, addr := range peers {
		for _, paddr := range persistedPeers {
			if addr.Equals(paddr) {
				continue peerLoop // found, next address
			}
		}
		t.Errorf("Peer[%d] not found in persisted peers", idx)
	}
	ct.Barrier("assertedPeers")
	ct.Wait("testing")

	ps, err := pr.ActivePeers(ctx)
	require.NoError(t, err)
	require.Len(t, ps, 0)
}
