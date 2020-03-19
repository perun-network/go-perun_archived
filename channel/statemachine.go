// Copyright (c) 2019 Chair of Applied Cryptography, Technische Universität
// Darmstadt, Germany. All rights reserved. This file is part of go-perun. Use
// of this source code is governed by a MIT-style license that can be found in
// the LICENSE file.

package channel

import (
	"github.com/pkg/errors"

	"perun.network/go-perun/wallet"
)

// A StateMachine is the channel pushdown automaton around a StateApp.
// It implements the state transitions specific for StateApps: Init and Update.
type StateMachine struct {
	*machine

	app StateApp `cloneable:"shallow"`
}

// NewStateMachine creates a new StateMachine.
func NewStateMachine(acc wallet.Account, params Params) (*StateMachine, error) {
	app, ok := params.App.(StateApp)
	if !ok {
		return nil, errors.New("app must be StateApp")
	}

	m, err := NewMachine(acc, params)
	if err != nil {
		return nil, err
	}

	return &StateMachine{
		machine: m,
		app:     app,
	}, nil
}

// Init sets the initial staging state to the given balance and data.
// It returns the initial state and own signature on it.
func (m *StateMachine) Init(initBals Allocation, initData Data) error {
	if err := m.expect(PhaseTransition{InitActing, InitSigning}); err != nil {
		return err
	}

	// we start with the initial state being the staging state
	initState, err := newState(&m.params, initBals, initData)
	if err != nil {
		return err
	}
	if err := m.app.ValidInit(&m.params, initState); err != nil {
		return err
	}

	m.setStaging(InitSigning, initState)
	return nil
}

// Update makes the provided state the staging state.
// It is checked whether this is a valid state transition.
func (m *StateMachine) Update(stagingState *State, actor Index) error {
	if err := m.expect(PhaseTransition{Acting, Signing}); err != nil {
		return err
	}

	if err := m.validTransition(stagingState, actor); err != nil {
		return err
	}

	m.setStaging(Signing, stagingState)
	return nil
}

// CheckUpdate checks if the given state is a valid transition from the current
// state and if the given signature is valid. It is a read-only operation that
// does not advance the state machine.
func (m *StateMachine) CheckUpdate(
	state *State, actor Index,
	sig wallet.Sig, sigIdx Index,
) error {
	if err := m.validTransition(state, actor); err != nil {
		return err
	}

	if ok, err := Verify(m.params.Parts[sigIdx], &m.params, state, sig); err != nil {
		return errors.WithMessagef(err, "verifying signature[%d]", sigIdx)
	} else if !ok {
		return errors.Errorf("invalid signature[%d]", sigIdx)
	}
	return nil
}

// validTransition makes all the default transition checks and additionally
// checks for a valid application specific transition.
// This is where a StateMachine and ActionMachine differ. In an ActionMachine,
// every action is checked as being a valid action by the application definition
// and the resulting state by applying all actions to the old state is by
// definition a valid new state.
func (m *StateMachine) validTransition(to *State, actor Index) (err error) {
	if actor >= m.N() {
		return errors.New("actor index is out of range")
	}
	if err := m.machine.validTransition(to); err != nil {
		return err
	}

	if err = m.app.ValidTransition(&m.params, m.currentTX.State, to, actor); IsStateTransitionError(err) {
		return err
	}
	return errors.WithMessagef(err, "runtime error in application's ValidTransition()")
}

// Clone returns a deep copy of StateMachine
func (m *StateMachine) Clone() *StateMachine {
	return &StateMachine{
		machine: m.machine.Clone(),
		app:     m.app,
	}
}
