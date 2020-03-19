// Copyright (c) 2019 Chair of Applied Cryptography, Technische Universität
// Darmstadt, Germany. All rights reserved. This file is part of go-perun. Use
// of this source code is governed by a MIT-style license that can be found in
// the LICENSE file.

package channel

import (
	"bytes"

	"github.com/pkg/errors"

	"perun.network/go-perun/wallet"
)

// An ActionMachine is the channel pushdown automaton around an ActionApp.
// It implements the state transitions specific for ActionApps: AddAction, Init
// and Update.
type ActionMachine struct {
	*machine

	app            ActionApp `cloneable:"shallow"`
	stagingActions []Action
}

// NewActionMachine creates a new ActionMachine.
func NewActionMachine(acc wallet.Account, params Params) (*ActionMachine, error) {
	app, ok := params.App.(ActionApp)
	if !ok {
		return nil, errors.New("app must be ActionApp")
	}

	m, err := NewMachine(acc, params)
	if err != nil {
		return nil, err
	}

	return &ActionMachine{
		machine:        m,
		app:            app,
		stagingActions: make([]Action, m.N()),
	}, nil
}

var actionPhases = []Phase{InitActing, Acting}

// AddAction adds the action of participant idx to the staging actions.
// It is checked that the action of that participant is not already set as it
// should not happen that an action is overwritten.
// The validity of the action applied to the current state is also checked as
// specified by the application.
// If the index is out of bounds, a panic occurs as this is an invalid usage of
// the machine.
func (m *ActionMachine) AddAction(idx Index, a Action) error {
	if !inPhase(m.phase, actionPhases) {
		return m.phaseErrorf(m.selfTransition(), "can only set action in an action phase")
	}

	if m.stagingActions[idx] != nil {
		return errors.Errorf("action for idx %d already set (ID: %x)", idx, m.params.id)
	}

	if err := m.app.ValidAction(&m.params, m.currentTX.State, idx, a); IsActionError(err) {
		return err
	} else if err != nil {
		return errors.WithMessagef(err, "runtime error in application's ValidAction() (ID: %x)", m.params.id)
	}

	m.stagingActions[idx] = a
	return nil
}

// Init creates the initial state as the combination of all initial actions.
func (m *ActionMachine) Init() error {
	if err := m.expect(PhaseTransition{InitActing, InitSigning}); err != nil {
		return err
	}

	initBals, initData, err := m.app.InitState(&m.params, m.stagingActions)
	if err != nil {
		return err
	}
	initState, err := newState(&m.params, initBals, initData)
	if err != nil {
		return err
	}

	m.setStaging(InitSigning, initState)
	return nil
}

// Update applies all staged actions to the current state to create the new
// staging state for signing.
func (m *ActionMachine) Update() error {
	if err := m.expect(PhaseTransition{Acting, Signing}); err != nil {
		return err
	}

	stagingState, err := m.app.ApplyActions(&m.params, m.currentTX.State, m.stagingActions)
	if err != nil {
		return err
	}

	m.setStaging(Signing, stagingState)
	return nil
}

// setStaging sets the current staging phase and state and additionally clears
// the staging actions
func (m *ActionMachine) setStaging(phase Phase, state *State) {
	m.stagingActions = make([]Action, m.N())
	m.machine.setStaging(phase, state)
}

// Clone returns a deep copy of ActionMachine
func (m *ActionMachine) Clone() *ActionMachine {
	clonedActions := make([]Action, m.N())
	for i := range m.stagingActions {
		if m.stagingActions[i] != nil {
			var buff bytes.Buffer
			m.stagingActions[i].Encode(&buff)
			clonedAction, err := m.app.DecodeAction(&buff)
			if err != nil {
				panic("App could not decode Action")
			}
			clonedActions[i] = clonedAction
		}
	}

	return &ActionMachine{
		machine:        m.machine.Clone(),
		app:            m.app,
		stagingActions: clonedActions,
	}
}
