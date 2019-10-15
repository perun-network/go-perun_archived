// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package channel // import "perun.network/go-perun/backend/sim/channel"

import (
	"fmt"
	"io"

	"github.com/pkg/errors"
	"perun.network/go-perun/backend/sim/wallet"
	"perun.network/go-perun/channel"
	"perun.network/go-perun/log"
	perunio "perun.network/go-perun/pkg/io"
	perun "perun.network/go-perun/wallet"
	"perun.network/go-perun/wire"
)

// ActionApp1 starts in state `Init` where the `Counter` of `data` is set to 0.
// Each round all participants send a `Delta` value between -10 and 10 together with `ActionType`=`Mod`.
// Participant with the id=version % len(participants) is excluded in each round to test the concept of invalid partakers.
// An excluded participant has to send `ActionType`=`NOP`.
// `Counter` is then updated by adding all `Delta`s to `Counter`.
type ActionApp1 struct {
	definition wallet.Address
	data       stateData
}

var _ channel.ActionApp = new(ActionApp1)

// ActionType type of the possible actions for an ActionApp1
type ActionType = int64

const (
	// Init is the initial agreement phase
	Init ActionType = iota
	// Mod A parts wants to modify the `Counter`
	Mod
	//NOP A parts does not want to modify the `Counter` (No-OP)
	NOP
)

// app1Action the action each part sends each round
type app1Action struct {
	Type ActionType

	// Delta [-10,10]
	Delta int64
}

var _ perunio.Serializable = new(app1Action)

// Encode encodes an `ActionApp1` into an `io.Writer`
func (a *app1Action) Encode(w io.Writer) error {
	return wire.Encode(w, a.Type, a.Delta)
}

// Decode decodes an `ActionApp1` from an `io.Reader`
func (a *app1Action) Decode(r io.Reader) error {
	return wire.Decode(r, &a.Type, &a.Delta)
}

// NewActionApp1 create a new `ActionApp1` with the specified definition
func NewActionApp1(definition wallet.Address) *ActionApp1 {
	return &ActionApp1{definition, stateData{1}}
}

// Def returns the definition of an `ActionApp1`
func (a ActionApp1) Def() perun.Address {
	return &a.definition
}

// DecodeData returns a decoded stateData or an error
func (a ActionApp1) DecodeData(r io.Reader) (channel.Data, error) {
	var data stateData
	return &data, data.Decode(r)
}

// DecodeAction returns a decoded app1Action or an error
func (a ActionApp1) DecodeAction(r io.Reader) (channel.Action, error) {
	var act app1Action
	return &act, act.Decode(r)
}

// ValidAction checks the passed actions for validity and returns an `ActionError` otherwise
func (a ActionApp1) ValidAction(params *channel.Params, state *channel.State, part uint, rawAction channel.Action) error {
	log.WithField("part", part).Trace("ValidAction")
	isPartaker := uint64(part) != (state.Version % uint64(len(params.Parts)))

	if params == nil || state == nil || rawAction == nil || part >= uint(len(params.Parts)) {
		log.Panic("Nil parameter")
	}
	if channel.ChannelID(params) != state.ID || params.ID() != state.ID {
		log.Panic("Wrong params or state id")
	}

	action := rawAction.(*app1Action)
	if state.IsFinal {
		log.Panic("Can't transition from a final state")
	}

	if isPartaker {
		if action.Type != Mod {
			return channel.NewActionError(params.ID(), fmt.Sprintf("Partaker %d refused to turn", part))
		}

		if action.Delta > 10 {
			return channel.NewActionError(params.ID(), "Participants delta >  10")
		} else if action.Delta < -10 {
			return channel.NewActionError(params.ID(), "Participants delta < -10")
		}
	} else if action.Type != NOP {
		return channel.NewActionError(params.ID(), fmt.Sprintf("%d is not a partaker", part))
	}

	return nil
}

// ApplyActions applies the actions unto a copy of `state` and returns the result or an error.
func (a ActionApp1) ApplyActions(params *channel.Params, state *channel.State, rawActions []channel.Action) (*channel.State, error) {
	log.Trace("ApplyActions")
	newState := state.Clone()
	newState.Version++
	newData := newState.Data.(*stateData)

	if rawActions == nil || len(rawActions) != len(params.Parts) {
		log.Panic("Wrong number of actions")
	}

	for i := range rawActions {
		action := rawActions[i].(*app1Action)

		if a.ValidAction(params, state, uint(i), action) != nil {
			log.Panic("Invalid action should not be passed to ApplyActions")
		}
		newData.Counter += action.Delta
	}

	return newState, nil
}

// InitState checks for the validity of the passed actions to form the initial state.
func (a ActionApp1) InitState(params *channel.Params, rawActions []channel.Action) (channel.Allocation, channel.Data, error) {
	if len(rawActions) != len(params.Parts) {
		log.Panic("wrong number of actions")
	}

	for part, rawAction := range rawActions {
		action, ok := rawAction.(*app1Action)
		if !ok {
			return channel.Allocation{}, nil, errors.New("Invalid Action type")
		}

		if action.Type != Init {
			return channel.Allocation{}, nil, errors.Errorf("Invalid action from part %d, expected: Init", part)
		}
	}

	return channel.Allocation{}, &stateData{0}, nil
}
