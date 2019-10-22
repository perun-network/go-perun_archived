// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package channel // import "perun.network/go-perun/backend/sim/channel"

import (
	"io"

	"github.com/pkg/errors"
	"perun.network/go-perun/backend/sim/wallet"
	"perun.network/go-perun/channel"
	"perun.network/go-perun/log"
	perun "perun.network/go-perun/wallet"
	"perun.network/go-perun/wire"
)

const (
	stateApp1InitValue int64 = (int64(1) << 31)
)

type StateApp1 struct {
	definition wallet.Address
	data       stateData
}

// stateData The data that is stored in channel.State.Data.
type stateData struct {
	Counter int64
}

var _ channel.StateApp = new(StateApp1)
var _ channel.Data = new(stateData)

// Encode encodes an Asset into the io.Writer `w`.
func (a *stateData) Encode(w io.Writer) error {
	return wire.Encode(w, a.Counter)
}

func (a *stateData) Decode(r io.Reader) error {
	return wire.Decode(r, &a.Counter)
}

func (a *stateData) Clone() channel.Data {
	return &stateData{Counter: a.Counter}
}

// NewStateApp1 create an App with the given definition Address.
func NewStateApp1(definition wallet.Address) *StateApp1 {
	return &StateApp1{definition, stateData{1}}
}

// Def returns the definition on the App.
func (a StateApp1) Def() perun.Address {
	return &a.definition
}

// DecodeData returns a decoded stateData or an error.
func (a StateApp1) DecodeData(r io.Reader) (channel.Data, error) {
	var data stateData
	return &data, data.Decode(r)
}

// ValidTransition checks the transition for validity.
func (a StateApp1) ValidTransition(params *channel.Params, from, to *channel.State) error {
	if from.IsFinal {
		log.Panic("Transition after final state requested")
	}
	if params == nil || from == nil || to == nil {
		log.Panic("nil argument")
	}
	if channel.ChannelID(params) != params.ID() || params.ID() != from.ID || from.ID != to.ID {
		return channel.NewStateTransitionError(params.ID(), "ChannelID should not change")
	}

	oldData, ok := from.Data.(*stateData)
	if !ok {
		log.Panic("Old data invalid")
	}
	newData, ok := to.Data.(*stateData)
	if !ok {
		return channel.NewStateTransitionError(params.ID(), "New data invalid")
	}

	if newData.Counter != oldData.Counter*2 && newData.Counter != oldData.Counter/2 {
		return channel.NewStateTransitionError(params.ID(), "Invalid counter transition")
	}

	return nil
}

// ValidInit checks the initial state for validity.
func (a StateApp1) ValidInit(params *channel.Params, state *channel.State) error {
	data, ok := state.Data.(*stateData)
	if !ok {
		return errors.New("Init data invalid")
	}
	if params == nil {
		log.Panic("Params nil")
	}

	if data.Counter != stateApp1InitValue {
		return errors.New("Invalid init state")
	}
	return nil
}
