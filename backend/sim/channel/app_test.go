// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package channel // import "perun.network/go-perun/backend/sim/channel"

import (
	"math/rand"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"perun.network/go-perun/backend/sim/wallet"
	"perun.network/go-perun/channel"
	chtest "perun.network/go-perun/channel/test"
	"perun.network/go-perun/pkg/io"
	pkgtest "perun.network/go-perun/pkg/io/test"
)

// actionAppSetup contains all data to run TestActionApp1.
type actionAppSetup struct {
	// ValidActParams `params` argument of ValidAction which should succeed.
	ValidActParams *channel.Params
	// ValidActState `state` argument of ValidAction which should succeed.
	ValidActState *channel.State
	// ValidActPart `part` argument of ValidAction which should succeed.
	ValidActPart uint
	// ValidActAction `action` argument of ValidAction which should succeed.
	ValidActAction channel.Action

	// ValidActParams `params` argument of ValidAction which should fail.
	InvalidActParams *channel.Params
	// ValidActState `state` argument of ValidAction which should fail.
	InvalidActState *channel.State
	// ValidActPart `part` argument of ValidAction which should fail.
	InvalidActPart uint
	// ValidActAction `action` argument of ValidAction which should fail.
	InvalidActAction channel.Action

	// ValidApplyParams `params` argument of ApplyActions which should succeed.
	ValidApplyParams *channel.Params
	// ValidApplyState `state` argument of ApplyActions which should succeed.
	ValidApplyState *channel.State
	// ValidApplyActions `[]Action` argument of ApplyActions which should succeed.
	ValidApplyActions []channel.Action

	// InvalidApplyParams `params` argument of ApplyActions which should fail.
	InvalidApplyParams *channel.Params
	// InvalidApplyState `state` argument of ApplyActions which should fail.
	InvalidApplyState *channel.State
	// InvalidApplyActions `[]Action` argument of ApplyActions which should fail.
	InvalidApplyActions []channel.Action

	// ValidInitParams `params` argument of InitState which should succeed.
	ValidInitParams *channel.Params
	// ValidInitActions `action` argument of InitState which should succeed.
	ValidInitActions []channel.Action

	// InvalidInitActions `action` argument of InitState which should succeed.
	InvalidInitActions []channel.Action

	// ActionApp the app on which the tests should be performed.
	ActionApp channel.ActionApp
}

// stateAppSetup contains all data to run TestStateApp1.
type stateAppSetup struct {
	// ValidTransParams `parameters` argument of ValidTransition which should succeed.
	ValidTransParams *channel.Params
	// ValidTransFrom `from` argument of ValidTransition which should succeed.
	ValidTransFrom *channel.State
	// ValidTransTo `to` argument of ValidTransition which should succeed.
	ValidTransTo *channel.State

	// InvalidTransParams `parameters` argument of ValidTransition which should fail.
	InvalidTransParams *channel.Params
	// InvalidTransFrom `from` argument of ValidTransition which should fail.
	InvalidTransFrom *channel.State
	// InvalidTransTo `to` argument of ValidTransition which should fail.
	InvalidTransTo *channel.State

	// ValidInitParams `parameters` argument of ValidInit which should succeed.
	ValidInitParams *channel.Params
	// ValidInitState `state` argument of ValidInit which should succeed.
	ValidInitState *channel.State

	// InvalidInitParams `parameters` argument of ValidInit which should succeed.
	InvalidInitParams *channel.Params
	// InvalidInitState `state` argument of ValidInit which should succeed.
	InvalidInitState *channel.State

	// StateApp the app on which the tests should be performed.
	StateApp channel.StateApp
}

const challengeDuration = 60

func TestStateApp1(t *testing.T) {
	rng := rand.New(rand.NewSource(1337))
	app := NewStateApp1(*wallet.NewRandomAddress(rng))

	t.Run("Create machine", func(t *testing.T) {
		randParams := chtest.NewRandomParams(rng, app)
		params, err := channel.NewParams(challengeDuration, randParams.Parts, app, randParams.Nonce)
		require.NoError(t, err)
		alloc := chtest.NewRandomAllocation(rng, params)
		initData := &stateData{stateApp1InitValue}

		acc := wallet.NewRandomAccount(rng)
		params.Parts[0] = acc.Address()

		machine, err := channel.NewStateMachine(acc, *params)
		assert.NoError(t, err)

		err = machine.Init(*alloc, initData)
		assert.NoError(t, err)
	})

	t.Run("StateApp1Test", func(t *testing.T) {
		setup := newStateAppSetup(app, rng, t)
		StateApp1Test(t, setup)
		pkgtest.GenericSerializableTest(t, setup.ValidInitState.Data)
	})
}

// StateApp1Test tests the `StateApp` interface functions of `StateApp1`.
func StateApp1Test(t *testing.T, s stateAppSetup) {
	assert.True(t, channel.IsStateApp(s.StateApp), "StateApp1 should be detected as StateApp")
	assert.False(t, channel.IsActionApp(s.StateApp), "StateApp1 should not be detected as ActionApp")

	t.Run("ValidTransition", func(t *testing.T) {
		assert.NoError(t, s.StateApp.ValidTransition(s.ValidTransParams, s.ValidTransFrom, s.ValidTransTo), "ValidTransition with valid arguments should succeed")

		// Set one parameter at a time to nil and check for panic.
		assert.Panics(t, func() { s.StateApp.ValidTransition(nil, s.ValidTransFrom, s.ValidTransTo) }, "ValidTransition with nil params should panic")
		assert.Panics(t, func() { s.StateApp.ValidTransition(s.ValidTransParams, nil, s.ValidTransTo) }, "ValidTransition with nil from-state should panic")
		assert.Panics(t, func() { s.StateApp.ValidTransition(s.ValidTransParams, s.ValidTransFrom, nil) }, "ValidTransition with nil to-state should panic")

		// Set one parameter at a time to invalid and check for error.
		err := s.StateApp.ValidTransition(s.InvalidTransParams, s.ValidTransFrom, s.ValidTransTo)
		assert.True(t, channel.IsStateTransitionError(err), "ValidTransition with invalid params should return StateTransitionError")
		assert.Equal(t, errors.Cause(err).(*channel.StateTransitionError).ID, s.InvalidTransParams.ID())

		err = s.StateApp.ValidTransition(s.ValidTransParams, s.InvalidTransFrom, s.ValidTransTo)
		assert.True(t, channel.IsStateTransitionError(err), "ValidTransition with invalid from-state should return StateTransitionError")
		assert.Equal(t, errors.Cause(err).(*channel.StateTransitionError).ID, s.ValidTransParams.ID())

		err = s.StateApp.ValidTransition(s.ValidTransParams, s.ValidTransFrom, s.InvalidTransTo)
		assert.True(t, channel.IsStateTransitionError(err), "ValidTransition with invalid to-state should return StateTransitionError")
		assert.Equal(t, errors.Cause(err).(*channel.StateTransitionError).ID, s.ValidTransParams.ID())
	})

	t.Run("ValidInit", func(t *testing.T) {
		assert.NoError(t, s.StateApp.ValidInit(s.ValidInitParams, s.ValidInitState), "ValidInit with valid arguments should succeed")

		// Set one parameter at a time to nil and check for panic.
		assert.Panics(t, func() { s.StateApp.ValidInit(nil, s.ValidInitState) }, "ValidInit with nil params should panic")
		assert.Panics(t, func() { s.StateApp.ValidInit(s.ValidInitParams, nil) }, "ValidInit with nil state should panic")

		// Set one parameter at a time to invalid and check for error
		//assert.Error(t, s.StateApp.ValidInit(s.InvalidInitParams, s.ValidInitState), "ValidInit with invalid params should return error").
		assert.Error(t, s.StateApp.ValidInit(s.ValidInitParams, s.InvalidInitState), "ValidInit with invalid state should return error")
	})
}

func TestActionApp1(t *testing.T) {
	rng := rand.New(rand.NewSource(1337))
	app := NewActionApp1(*wallet.NewRandomAddress(rng))

	t.Run("Create machine", func(t *testing.T) {
		params := chtest.NewRandomParams(rng, app)
		params.ChallengeDuration = challengeDuration

		acc := wallet.NewRandomAccount(rng)
		params.Parts[0] = acc.Address()

		_, err := channel.NewActionMachine(acc, *params)
		require.NoError(t, err)
	})

	t.Run("ActionApp1Test", func(t *testing.T) {
		setup := newActionAppSetup(app, rng, t)
		ActionApp1Test(t, setup)
		pkgtest.GenericSerializableTest(t, setup.ValidActState.Data, setup.ValidActAction)
	})
}

// ActionApp1Test tests the `ActionApp` interface functions of `ActionApp1`.
func ActionApp1Test(t *testing.T, s actionAppSetup) {
	assert.False(t, channel.IsStateApp(s.ActionApp), "ActionApp1 should not be detected as StateApp")
	assert.True(t, channel.IsActionApp(s.ActionApp), "ActionApp1 should be detected as ActionApp")

	t.Run("ValidAction", func(t *testing.T) {
		assert.NoError(t, s.ActionApp.ValidAction(s.ValidActParams, s.ValidActState, s.ValidActPart, s.ValidActAction), "ValidAction with valid arguments should succeed")

		// Set one parameter at a time to nil and check for panic.
		assert.Panics(t, func() { s.ActionApp.ValidAction(nil, s.ValidActState, s.ValidActPart, s.ValidActAction) }, "ValidAction with nil params should panic")
		assert.Panics(t, func() { s.ActionApp.ValidAction(s.ValidActParams, nil, s.ValidActPart, s.ValidActAction) }, "ValidAction with nil state should panic")
		assert.Panics(t, func() {
			s.ActionApp.ValidAction(s.ValidActParams, s.ValidActState, uint(0xffffffff), s.ValidActAction)
		}, "ValidAction with invalid part should panic")
		assert.Panics(t, func() { s.ActionApp.ValidAction(s.ValidActParams, s.ValidActState, s.ValidActPart, nil) }, "ValidAction with nil actions should panic")

		// Set one parameter at a time to invalid and check for error.
		assert.Panics(t, func() { s.ActionApp.ValidAction(s.InvalidActParams, s.ValidActState, s.ValidActPart, s.ValidActAction) }, "ValidAction with invalid params should panic")
		assert.Panics(t, func() { s.ActionApp.ValidAction(s.ValidActParams, s.InvalidActState, s.ValidActPart, s.ValidActAction) }, "ValidAction with invalid state should panic")

		err := s.ActionApp.ValidAction(s.ValidActParams, s.ValidActState, s.InvalidActPart, s.ValidActAction)
		assert.True(t, channel.IsActionError(err), "ValidAction with invalid part should return ActionError")
		assert.Equal(t, errors.Cause(err).(*channel.ActionError).ID, s.ValidActParams.ID())

		err = s.ActionApp.ValidAction(s.ValidActParams, s.ValidActState, s.ValidActPart, s.InvalidActAction)
		assert.True(t, channel.IsActionError(err), "ValidAction with invalid actions should return ActionError")
		assert.Equal(t, errors.Cause(err).(*channel.ActionError).ID, s.ValidActParams.ID())
	})

	t.Run("ApplyActions", func(t *testing.T) {
		require.Equal(t, s.ValidApplyParams.ID(), s.ValidApplyState.ID)
		_, err := s.ActionApp.ApplyActions(s.ValidApplyParams, s.ValidApplyState, s.ValidApplyActions)
		assert.NoError(t, err, "ApplyActions with valid arguments should succeed")
		// DEFECT we cant compare the result state of ApplyActions because they are not compareable.

		// Set one parameter at a time to nil and check for panic.
		assert.Panics(t, func() { s.ActionApp.ApplyActions(nil, s.ValidApplyState, s.ValidApplyActions) }, "ApplyActions with nil params should panic")
		assert.Panics(t, func() { s.ActionApp.ApplyActions(s.ValidApplyParams, nil, s.ValidApplyActions) }, "ApplyActions with nil state should panic")
		assert.Panics(t, func() { s.ActionApp.ApplyActions(s.ValidApplyParams, s.ValidApplyState, nil) }, "ApplyActions with nil actions should panic")

		// Set one parameter at a time to invalid and check for panic.
		assert.Panics(t, func() { s.ActionApp.ApplyActions(s.InvalidApplyParams, s.ValidApplyState, s.ValidApplyActions) }, "ApplyActions with invalid params should panic")
		assert.Panics(t, func() { s.ActionApp.ApplyActions(s.ValidApplyParams, s.InvalidApplyState, s.ValidApplyActions) }, "ApplyActions with invalid state should panic")
		assert.Panics(t, func() { s.ActionApp.ApplyActions(s.ValidApplyParams, s.ValidApplyState, s.InvalidApplyActions) }, "ApplyActions with invalid actions should panic")
	})

	t.Run("InitState", func(t *testing.T) {
		require.NotZero(t, len(s.ValidInitActions), "ValidInitActions needs at least one element")
		_, _, err := s.ActionApp.InitState(s.ValidInitParams, s.ValidInitActions)
		assert.NoError(t, err, "InitState with valid arguments should succeed")

		// Set one parameter at a time to nil and check for panic
		// DEFECT we cant compare Allocation and Data.
		assert.Panics(t, func() { s.ActionApp.InitState(nil, s.ValidInitActions) }, "InitState with nil parameter should panic")
		assert.Panics(t, func() { s.ActionApp.InitState(s.ValidInitParams, nil) }, "InitState with nil actions should panic")
		assert.Panics(t, func() { s.ActionApp.InitState(s.ValidInitParams, s.InvalidInitActions) }, "InitState with invalid actions should return error")
	})
}

func newStateAppSetup(app *StateApp1, rng *rand.Rand, t *testing.T) stateAppSetup {
	randParams := chtest.NewRandomParams(rng, app)

	vParams, err := channel.NewParams(challengeDuration, randParams.Parts, app, randParams.Nonce)
	require.NoError(t, err)
	require.Greater(t, len(vParams.Parts), 1)
	ivParams, err := channel.NewParams(challengeDuration+1, randParams.Parts, app, randParams.Nonce)
	require.NoError(t, err)
	version := uint64(rng.Int63n(1 << 30))

	vInitParams := *vParams
	vInitState := &channel.State{ID: vInitParams.ID(), Version: version, Allocation: *chtest.NewRandomAllocation(rng, &vInitParams), Data: &stateData{stateApp1InitValue}, IsFinal: false}

	ivInitParams := *ivParams
	ivInitState := &channel.State{ID: vInitParams.ID(), Version: version, Allocation: *chtest.NewRandomAllocation(rng, &vInitParams), Data: &stateData{stateApp1InitValue + 1}, IsFinal: false}

	vTransParams := *vParams
	vTransFrom := &channel.State{ID: vInitParams.ID(), Version: version, Allocation: *chtest.NewRandomAllocation(rng, &vInitParams), Data: &stateData{stateApp1InitValue}, IsFinal: false}
	vTransTo := &channel.State{ID: vInitParams.ID(), Version: version, Allocation: *chtest.NewRandomAllocation(rng, &vInitParams), Data: &stateData{stateApp1InitValue / 2}, IsFinal: false}

	ivTransParams := *ivParams
	ivTransFrom := &channel.State{ID: vInitParams.ID(), Version: version, Allocation: *chtest.NewRandomAllocation(rng, &vInitParams), Data: &stateData{10}, IsFinal: false}
	ivTransTo := &channel.State{ID: vInitParams.ID(), Version: version, Allocation: *chtest.NewRandomAllocation(rng, &vInitParams), Data: &stateData{80}, IsFinal: false}

	return stateAppSetup{
		ValidTransParams: &vTransParams,
		ValidTransFrom:   vTransFrom,
		ValidTransTo:     vTransTo,

		InvalidTransParams: &ivTransParams,
		InvalidTransFrom:   ivTransFrom,
		InvalidTransTo:     ivTransTo,

		ValidInitParams: &vInitParams, ValidInitState: vInitState,
		InvalidInitParams: &ivInitParams, InvalidInitState: ivInitState,
		StateApp: app}
}

func newActionAppSetup(app *ActionApp1, rng *rand.Rand, t *testing.T) actionAppSetup {
	randParams := chtest.NewRandomParams(rng, app)

	vParams, err := channel.NewParams(challengeDuration, randParams.Parts, app, randParams.Nonce)
	require.NoError(t, err)
	require.Greater(t, len(vParams.Parts), 1)
	ivParams, err := channel.NewParams(challengeDuration+1, randParams.Parts, app, randParams.Nonce)
	require.NoError(t, err)
	version := uint64(rng.Int63n(1 << 30))

	vActParams := *vParams
	vActState := &channel.State{ID: vActParams.ID(), Version: version, Allocation: *chtest.NewRandomAllocation(rng, &vActParams), Data: &stateData{1 << 10}, IsFinal: false}
	vActPart := version%uint64(len(randParams.Parts)) + 1
	vActAction := &app1Action{Mod, 9}

	ivActParams := *ivParams
	ivActState := &channel.State{ID: ivActParams.ID(), Version: version, Allocation: *chtest.NewRandomAllocation(rng, &vActParams), Data: &stateData{-10}, IsFinal: false}
	ivActPart := version % uint64(len(randParams.Parts))
	ivActAction := &app1Action{Mod, 11}

	vApplyParams := *vParams
	vApplyState := &channel.State{ID: vApplyParams.ID(), Version: version, Allocation: *chtest.NewRandomAllocation(rng, &vActParams), Data: &stateData{1 << 10}, IsFinal: false}
	vApplyActions := make([]channel.Action, len(randParams.Parts))
	for i := range vApplyActions {
		vApplyActions[i] = &app1Action{Mod, int64(rng.Intn(20) - 10)}
	}
	vApplyActions[ivActPart] = &app1Action{NOP, 0}

	ivApplyParams := *ivParams
	ivApplyState := &channel.State{ID: ivApplyParams.ID(), Version: uint64(rng.Int63n(1 << 30)), Allocation: *chtest.NewRandomAllocation(rng, &vActParams), Data: &stateData{-10}, IsFinal: false}
	ivApplyActions := make([]channel.Action, len(randParams.Parts))
	for i := range vApplyActions {
		ivApplyActions[i] = &app1Action{Mod, int64(rng.Intn(20) - 10)}
	}

	vInitParams := *vParams
	vInitActions := make([]io.Serializable, len(vParams.Parts))
	for i := range vInitActions {
		vInitActions[i] = &app1Action{Init, 0}
	}

	ivInitActions := make([]io.Serializable, len(vParams.Parts))
	for i := range ivInitActions {
		ivInitActions[i] = &app1Action{Init, 0}
	}
	ivInitActions[0] = &app1Action{Mod, 0}

	return actionAppSetup{
		ValidActParams: &vActParams,
		ValidActState:  vActState,
		ValidActPart:   uint(vActPart),
		ValidActAction: vActAction,

		InvalidActParams: &ivActParams,
		InvalidActState:  ivActState,
		InvalidActPart:   uint(ivActPart),
		InvalidActAction: ivActAction,

		ValidApplyParams:  &vApplyParams,
		ValidApplyState:   vApplyState,
		ValidApplyActions: vApplyActions,

		InvalidApplyParams:  &ivApplyParams,
		InvalidApplyState:   ivApplyState,
		InvalidApplyActions: ivApplyActions,

		ValidInitParams: &vInitParams, ValidInitActions: vInitActions,
		ActionApp: app}
}
