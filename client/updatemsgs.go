// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package client

import (
	//"io"

	//"github.com/pkg/errors"

	"perun.network/go-perun/channel"
	//perunio "perun.network/go-perun/pkg/io"
	//"perun.network/go-perun/wire"
	//"perun.network/go-perun/wire/msg"
)

type (
	// ChannelMsg are all messages that can be routed to a particular channel controller
	ChannelMsg interface {
		ID() channel.ID
	}

	// ChannelUpdate is a channel update proposal
	ChannelUpdate struct {
		State    *channel.State
		ActorIdx uint16
	}

	ChannelUpdateAcc struct {
		ID      channel.ID
		Version uint64
		Sig     channel.Sig
	}

	ChannelUpdateRej struct {
		Alt    *channel.State
		Sig    channel.Sig
		Reason string
	}
)

// TODO:
// - implement interface io.Serializable (methods Encode, Decode)
//   - use new channel.DecodeSig to decode the signature
// - implement interface msg.Msg (method Type() msg.Type)
//   - add msg types to wire.Type enum
//   - register decoders, see init() of proposalmsgs.go
// - implement interface ChannelMsg (method ID() channel.ID) on the three channel messages
//   - can be used to define the predicate when creating the channel receiver
