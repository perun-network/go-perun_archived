// Copyright 2019 - See NOTICE file for copyright holders.
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

package client

import (
	"context"
	"math/big"

	"github.com/pkg/errors"

	"perun.network/go-perun/channel"
	"perun.network/go-perun/log"
	"perun.network/go-perun/pkg/io"
	"perun.network/go-perun/pkg/sync/atomic"
	"perun.network/go-perun/wallet"
	"perun.network/go-perun/wire"
)

const proposerIdx, proposeeIdx = 0, 1

type (
	// A ProposalHandler decides how to handle incoming channel proposals from
	// other channel network peers.
	ProposalHandler interface {
		// HandleProposal is the user callback called by the Client on an incoming channel
		// proposal.
		HandleProposal(ChannelProposal, *ProposalResponder)
	}

	// ProposalHandlerFunc is an adapter type to allow the use of functions as
	// proposal handlers. ProposalHandlerFunc(f) is a ProposalHandler that calls
	// f when HandleProposal is called.
	ProposalHandlerFunc func(ChannelProposal, *ProposalResponder)

	// ProposalResponder lets the user respond to a channel proposal. If the user
	// wants to accept the proposal, they should call Accept(), otherwise Reject().
	// Only a single function must be called and every further call causes a
	// panic.
	ProposalResponder struct {
		client *Client
		peer   wire.Address
		req    ChannelProposal
		called atomic.Bool
	}

	// ProposalAcc is the proposal acceptance struct that the user passes to
	// ProposalResponder.Accept() when they want to accept an incoming channel
	// proposal.
	ProposalAcc struct {
		Participant wallet.Address
	}
)

// HandleProposal calls the proposal handler function.
func (f ProposalHandlerFunc) HandleProposal(p ChannelProposal, r *ProposalResponder) { f(p, r) }

// Accept lets the user signal that they want to accept the channel proposal.
// Returns the newly created channel controller if the channel was successfully
// created and funded. Panics if the proposal was already accepted or rejected.
//
// After the channel controller got successfully set up, it is passed to the
// callback registered with Client.OnNewChannel. Accept returns after this
// callback has run.
//
// It is important that the passed context does not cancel before twice the
// ChallengeDuration has passed (at least for real blockchain backends with wall
// time), or the channel cannot be settled if a peer times out funding.
//
// After the channel got successfully created, the user is required to start the
// channel watcher with Channel.Watch() on the returned channel controller.
func (r *ProposalResponder) Accept(ctx context.Context, acc ProposalAcc) (*Channel, error) {
	if ctx == nil {
		return nil, errors.New("context must not be nil")
	}

	if !r.called.TrySet() {
		log.Panic("multiple calls on proposal responder")
	}

	return r.client.handleChannelProposalAcc(ctx, r.peer, r.req, acc)
}

// Reject lets the user signal that they reject the channel proposal.
// Returns whether the rejection message was successfully sent. Panics if the
// proposal was already accepted or rejected.
func (r *ProposalResponder) Reject(ctx context.Context, reason string) error {
	if !r.called.TrySet() {
		log.Panic("multiple calls on proposal responder")
	}
	return r.client.handleChannelProposalRej(ctx, r.peer, r.req, reason)
}

// ProposeChannel attempts to open a channel with the parameters and peers from
// ChannelProposal prop:
// - the proposal is sent to the peers and if all peers accept,
// - the channel is funded. If successful,
// - the channel controller is returned.
//
// After the channel controller got successfully set up, it is passed to the
// callback registered with Client.OnNewChannel. Accept returns after this
// callback has run.
//
// It is important that the passed context does not cancel before twice the
// ChallengeDuration has passed (at least for real blockchain backends with wall
// time), or the channel cannot be settled if a peer times out funding.
//
// After the channel got successfully created, the user is required to start the
// channel watcher with Channel.Watch() on the returned channel
// controller.
func (c *Client) ProposeChannel(ctx context.Context, prop ChannelProposal) (*Channel, error) {
	if ctx == nil || prop == nil {
		c.log.Panic("invalid nil argument")
	}

	// 1. validate input
	peer := prop.Proposal().PeerAddrs[proposeeIdx]
	if err := c.validTwoPartyProposal(prop, proposerIdx, peer); err != nil {
		return nil, errors.WithMessage(err, "invalid channel proposal")
	}

	// 2. send proposal, wait for response, create channel object
	ch, err := c.proposeTwoPartyChannel(ctx, prop)
	if err != nil {
		return nil, errors.WithMessage(err, "channel proposal")
	}

	// 3. fund
	return ch, c.fundChannel(ctx, ch, prop)
}

// handleChannelProposal implements the receiving side of the (currently)
// two-party channel proposal protocol.
// The proposer is expected to be the first peer in the participant list.
//
// This handler is dispatched from the Client.Handle routine.
func (c *Client) handleChannelProposal(
	handler ProposalHandler, p wire.Address, req ChannelProposal) {
	if err := c.validTwoPartyProposal(req, proposeeIdx, p); err != nil {
		c.logPeer(p).Debugf("received invalid channel proposal: %v", err)
		return
	}

	c.logPeer(p).Trace("calling proposal handler")
	responder := &ProposalResponder{client: c, peer: p, req: req}
	handler.HandleProposal(req, responder)
	// control flow continues in responder.Accept/Reject
}

func (c *Client) handleChannelProposalAcc(
	ctx context.Context, p wire.Address,
	prop ChannelProposal, acc ProposalAcc,
) (ch *Channel, err error) {
	if ch, err = c.acceptChannelProposal(ctx, prop, p, acc); err != nil {
		return ch, errors.WithMessage(err, "accept channel proposal")
	}

	return ch, c.fundChannel(ctx, ch, prop)
}

func (c *Client) acceptChannelProposal(
	ctx context.Context,
	prop ChannelProposal,
	p wire.Address,
	acc ProposalAcc,
) (*Channel, error) {
	if acc.Participant == nil {
		c.logPeer(p).Error("user returned nil Participant in ProposalAcc")
		return nil, errors.New("nil Participant in ProposalAcc")
	}

	// enables caching of incoming version 0 signatures before sending any message
	// that might trigger a fast peer to send those. We don't know the channel id
	// yet so the cache predicate is coarser than the later subscription.
	enableVer0Cache(ctx, c.conn)

	msgAccept := prop.Proposal().NewChannelProposalAcc(acc.Participant, WithRandomNonce())
	if err := c.conn.pubMsg(ctx, msgAccept, p); err != nil {
		c.logPeer(p).Errorf("error sending proposal acceptance: %v", err)
		return nil, errors.WithMessage(err, "sending proposal acceptance")
	}

	return c.completeCPP(ctx, prop, msgAccept, proposeeIdx)
}

func (c *Client) handleChannelProposalRej(
	ctx context.Context, p wire.Address,
	req ChannelProposal, reason string,
) error {
	msgReject := &ChannelProposalRej{
		ProposalID: req.Proposal().ProposalID(),
		Reason:     reason,
	}
	if err := c.conn.pubMsg(ctx, msgReject, p); err != nil {
		c.logPeer(p).Warn("error sending proposal rejection")
		return err
	}
	return nil
}

// proposeTwoPartyChannel implements the multi-party channel proposal
// protocol for the two-party case. It returns the agreed upon channel
// parameters.
func (c *Client) proposeTwoPartyChannel( //comment: rename to `twoPartyProposal`?
	ctx context.Context,
	proposal ChannelProposal,
) (*Channel, error) {
	propBase := proposal.Proposal()
	peer := propBase.PeerAddrs[proposeeIdx]

	// enables caching of incoming version 0 signatures before sending any message
	// that might trigger a fast peer to send those. We don't know the channel id
	// yet so the cache predicate is coarser than the later subscription.
	enableVer0Cache(ctx, c.conn)

	proposalID := propBase.ProposalID()
	isResponse := func(e *wire.Envelope) bool {
		return (e.Msg.Type() == wire.ChannelProposalAcc &&
			e.Msg.(*ChannelProposalAcc).ProposalID == proposalID) ||
			(e.Msg.Type() == wire.ChannelProposalRej &&
				e.Msg.(*ChannelProposalRej).ProposalID == proposalID)
	}
	/* comment:
	* - rename `isResponse` to `filter`?
	* - allowing an arbitrary function as subscription filter doesn't seem good practice to me
	* -- risk of performance degradation
	* -- probably hard or impossible to map to standard pub/sub technology like apache kafka (see https://kafka.apache.org/26/javadoc/org/apache/kafka/clients/consumer/KafkaConsumer.html#subscribe-java.util.Collection-)
	 */
	receiver := wire.NewReceiver()
	// nolint:errcheck
	defer receiver.Close()

	if err := c.conn.Subscribe(receiver, isResponse); err != nil {
		return nil, errors.WithMessage(err, "subscribing proposal response recv")
	}

	if err := c.conn.pubMsg(ctx, proposal, peer); err != nil {
		return nil, errors.WithMessage(err, "publishing channel proposal")
	}

	env, err := receiver.Next(ctx)
	if err != nil {
		return nil, errors.WithMessage(err, "receiving proposal response")
	}
	if rej, ok := env.Msg.(*ChannelProposalRej); ok {
		return nil, errors.Errorf("channel proposal rejected: %v", rej.Reason)
	}

	acc := env.Msg.(*ChannelProposalAcc) // this is safe because of predicate isResponse
	return c.completeCPP(ctx, proposal, acc, proposerIdx)
}

// validTwoPartyProposal checks that the proposal is valid in the two-party
// setting, where the proposer is expected to have index 0 in the peer list and
// the receiver to have index 1. The generic validity of the proposal is also
// checked.
func (c *Client) validTwoPartyProposal(
	proposal ChannelProposal,
	ourIdx int,
	peerAddr wallet.Address,
) error {
	base := proposal.Proposal()
	if err := base.Valid(); err != nil {
		return err
	}

	if len(base.PeerAddrs) != 2 {
		return errors.Errorf("exptected 2 peers, got %d", len(proposal.Proposal().PeerAddrs))
	}

	peerIdx := ourIdx ^ 1
	// In the 2PCPP, the proposer is expected to have index 0
	if !base.PeerAddrs[peerIdx].Equals(peerAddr) {
		return errors.Errorf("remote peer doesn't have peer index %d", peerIdx)
	}

	// In the 2PCPP, the receiver is expected to have index 1
	if !base.PeerAddrs[ourIdx].Equals(c.address) {
		return errors.Errorf("we don't have peer index %d", ourIdx)
	}

	if proposal.Type() == wire.SubChannelProposal {
		/*
			comment:
			- anything specific we need to check for `SubChannelProposal`?
			-- e.g., should we check that parent channel exists? and has sufficient funds? uses same assets? has same participants? ...
			- should we change `validTwoPartyProposal` to receive type `ChannelProposal`?
			-- a similar comment can be made for `exchangeTwoPartyProposal`, `setupChannel`. it seems like the abstraction `ChannelProposal` was implemented only half-way
		*/
		return errors.New("not implemented yet")
	}

	return nil
}

func (c *Client) completeCPP(
	ctx context.Context,
	prop ChannelProposal,
	acc *ChannelProposalAcc,
	partIdx channel.Index,
) (*Channel, error) {
	nonce := calcNonce(nonceShares(prop.Proposal().NonceShare, acc.NonceShare))
	parts := participants(prop.Proposal().ParticipantAddr, acc.ParticipantAddr)
	params := channel.NewParamsUnsafe(prop.Proposal().ChallengeDuration, parts, prop.Proposal().App, nonce)

	return c.completeChannelInit(ctx, prop, params, partIdx) //comment: move function content to here?
}

func participants(proposer, proposee wallet.Address) []wallet.Address {
	parts := make([]wallet.Address, 2)
	parts[proposerIdx] = proposer
	parts[proposeeIdx] = proposee
	return parts
}

func nonceShares(proposer, proposee NonceShare) []NonceShare {
	shares := make([]NonceShare, 2)
	shares[proposerIdx] = proposer
	shares[proposeeIdx] = proposee
	return shares
}

// calcNonce calculates a nonce from its shares. The order of the shares must
// correspond to the participant indices.
func calcNonce(nonceShares []NonceShare) channel.Nonce {
	hasher := newHasher()
	for i, share := range nonceShares {
		if err := io.Encode(hasher, share); err != nil {
			log.Panicf("Failed to encode nonce share %d for hashing", i)
		}
	}
	return channel.NonceFromBytes(hasher.Sum(nil))
}

// completeChannelInit sets up a new channel controller for the given proposal and
// params, using the wallet to unlock the account for our participant.
//
// The initial state with signatures is exchanged. The channel will be funded
// and if successful, the channel controller is returned.
//
// It does not perform a validity check on the proposal, so make sure to only
// pass valid proposals.
//
// It is important that the passed context does not cancel before twice the
// ChallengeDuration has passed (at least for real blockchain backends with wall
// time), or the channel cannot be settled if a peer times out funding.
func (c *Client) completeChannelInit( //comment: rename from `setupChannel`, update docu
	ctx context.Context,
	prop ChannelProposal,
	params *channel.Params,
	idx channel.Index, // our index
) (*Channel, error) {
	if c.channels.Has(params.ID()) {
		return nil, errors.New("channel already exists")
	}

	acc, err := c.wallet.Unlock(params.Parts[idx])
	if err != nil {
		return nil, errors.WithMessage(err, "unlocking account")
	}

	ch, err := c.newChannel(acc, prop.Proposal().PeerAddrs, *params)
	if err != nil {
		return nil, err
	}

	// if subchannel proposal receiver, setup register funding update
	if prop.Type() == wire.SubChannelProposal && idx == proposeeIdx {
		parentChannelID := prop.(*SubChannelProposal).ParentChannelID
		parentChannel, ok := c.channels.Get(parentChannelID)
		if !ok {
			return ch, errors.New("referenced parent channel not found")
		}
		subAlloc := &channel.SubAlloc{ID: ch.ID(), Bals: ch.State().Allocation.Sum()}
		parentChannel.registerSubchannelFunding(newSubchannelFunding(ctx, subAlloc))
	}

	if err := c.pr.ChannelCreated(ctx, ch.machine, prop.Proposal().PeerAddrs); err != nil {
		return ch, errors.WithMessage(err, "persisting new channel")
	}

	if err := ch.init(ctx, prop.Proposal().InitBals, prop.Proposal().InitData); err != nil {
		return ch, errors.WithMessage(err, "setting initial bals and data")
	}
	if err := ch.initExchangeSigsAndEnable(ctx); err != nil { //comment: this is a bit weird. we are sending messages on the channel connection before the channel proposal protocol has been completed
		return ch, errors.WithMessage(err, "exchanging initial sigs and enabling state")
	}

	//comment: funding removed, update function documentation

	return ch, nil
}

func (c *Client) fundChannel(ctx context.Context, ch *Channel, prop ChannelProposal) (err error) {
	switch prop.Type() {
	case wire.LedgerChannelProposal:
		if err = c.fundLedgerChannel(ctx, ch); err != nil {
			return errors.WithMessage(err, "funding ledger channel")
		}

	case wire.SubChannelProposal:
		switch ch.Idx() {
		case proposerIdx:
			err = c.fundSubchannel(ctx, prop.(*SubChannelProposal), ch)
			return errors.WithMessage(err, "funding subchannel")
		case proposeeIdx:
			parentChannelID := prop.(*SubChannelProposal).ParentChannelID
			parentChannel, ok := c.channels.Get(parentChannelID)
			if !ok {
				return errors.New("referenced parent channel not found")
			}
			return parentChannel.awaitSubchannelFunded(ctx, ch.ID())
		default:
			return errors.New("invalid participant index")
		}

	default:
		return errors.New("invalid channel proposal type")
	}
	return nil
}

func (c *Client) completeFunding(ctx context.Context, ch *Channel) error {
	params := ch.Params()
	if err := ch.machine.SetFunded(ctx); err != nil {
		return errors.WithMessage(err, "error in SetFunded()")
	}
	if !c.channels.Put(params.ID(), ch) {
		return errors.New("channel already exists")
	}
	c.wallet.IncrementUsage(params.Parts[ch.machine.Idx()])
	return nil
}

func (c *Client) fundLedgerChannel(ctx context.Context, ch *Channel) (err error) {
	if err = c.funder.Fund(ctx,
		channel.FundingReq{
			Params: ch.Params(),
			State:  ch.machine.State(), // initial state
			Idx:    ch.machine.Idx(),
		}); channel.IsFundingTimeoutError(err) {
		ch.Log().Warnf("Peers timed out funding channel(%v); settling...", err)
		serr := ch.Settle(ctx)
		return errors.WithMessagef(err,
			"peers timed out funding (subsequent settlement error: %v)", serr)
	} else if err != nil { // other runtime error
		ch.Log().Warnf("error while funding channel: %v", err)
		return errors.WithMessage(err, "error while funding channel")
	}

	return c.completeFunding(ctx, ch)
}

func (c *Client) fundSubchannel(ctx context.Context, prop *SubChannelProposal, subChannel *Channel) error {
	//comment: implement this functionality in subchannel funder?

	parentChannel, ok := c.channels.Get(prop.ParentChannelID)
	if !ok {
		return errors.New("referenced channel does not exist")
	}

	//comment: lock the channel? no updates should be performed during subchannel funding
	state := parentChannel.State().Clone()

	lockedBalances := make([]*big.Int, len(prop.InitBals.Assets))
	//comment: check asset equality? or is this already checked earlier?
	//comment: check OTY's merge request !343 (#406) (funding requests), can probably help with implementing this part below
	// introduces channel.Balances = [][]Bal, which has Sum()
	for a, assetBalances := range prop.InitBals.Balances {
		var acc *big.Int = big.NewInt(0)
		for u, userBalance := range assetBalances {
			allocated := state.Allocation.Balances[a][u]
			allocated.Sub(allocated, userBalance)
			if allocated.Cmp(big.NewInt(0)) < 0 {
				return errors.New("insufficient funds")
			}
			acc.Add(acc, userBalance)
		}
		lockedBalances[a] = acc
	}

	subAlloc := channel.SubAlloc{ID: subChannel.ID(), Bals: lockedBalances}
	state.Locked = append(state.Locked, subAlloc)

	state.Version++

	cu := ChannelUpdate{state, proposerIdx}
	if err := parentChannel.Update(ctx, cu); err != nil {
		return errors.WithMessage(err, "parent channel funding update")
	}

	return c.completeFunding(ctx, subChannel)
}

// enableVer0Cache enables caching of incoming version 0 signatures.
func enableVer0Cache(ctx context.Context, c wire.Cacher) {
	c.Cache(ctx, func(m *wire.Envelope) bool {
		return m.Msg.Type() == wire.ChannelUpdateAcc &&
			m.Msg.(*msgChannelUpdateAcc).Version == 0
	})
}
