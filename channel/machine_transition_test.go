// Copyright (c) 2020 Chair of Applied Cryptography, Technische Universität
// Darmstadt, Germany. All rights reserved. This file is part of go-perun. Use
// of this source code is governed by a MIT-style license that can be found in
// the LICENSE file.

package channel_test

import (
	"math/big"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"perun.network/go-perun/channel"
	"perun.network/go-perun/channel/test"
	pkgtest "perun.network/go-perun/pkg/test"
	"perun.network/go-perun/wallet"
	wtest "perun.network/go-perun/wallet/test"
)

func TestMachine(t *testing.T) {
	rng := rand.New(rand.NewSource(0xDDDDD))
	app := test.NewRandomApp(rng)
	params := test.NewRandomParams(rng, app.Def())
	accs := make([]wallet.Account, len(params.Parts))
	for i := range accs {
		accs[i] = wtest.NewRandomAccount(rng)
		params.Parts[i] = accs[i].Address()
	}
	m, err := channel.NewStateMachine(accs[0], *params)
	require.NoError(t, err)

	t.Run("clone", func(t *testing.T) {
		testMachineClone(*params, accs[0], t)
	})
	t.Run("transitions", func(t *testing.T) {
		testStateMachineTransitions(t, params, accs, m)
	})
}

func TestValidTransition(t *testing.T) {
	rng := rand.New(rand.NewSource(0xDDDDD))
	app := test.NewRandomApp(rng)
	params := test.NewRandomParams(rng, app.Def())
	accs := make([]wallet.Account, len(params.Parts))
	for i := range accs {
		accs[i] = wtest.NewRandomAccount(rng)
		params.Parts[i] = accs[i].Address()
	}
	m, err := channel.NewStateMachine(accs[0], *params)
	require.NoError(t, err)
	initAlloc := test.NewRandomAllocation(rng, len(params.Parts))
	initData := channel.NewMockOp(channel.OpValid)

	require.NoError(t, m.Init(*initAlloc, initData))
	state := &channel.State{
		ID:         params.ID(),
		Version:    0,
		App:        params.App,
		Allocation: *initAlloc,
		Data:       initData,
	}
	newState := state.Clone()
	newState.Version++

	for i := range params.Parts {
		sig, err := channel.Sign(accs[i], params, state)
		require.NoError(t, err)
		require.NoError(t, m.AddSig(channel.Index(i), sig))
	}
	require.NoError(t, m.EnableInit())

	// Invalid IDx
	{
		err := m.CheckUpdate(state, channel.Index(len(params.Parts)+1), nil, 0)
		require.Error(t, err)
		assert.Equal(t, "actor index is out of range", err.Error())
	}
	// Invalid channel ID
	{
		newState := newState.Clone()
		newState.ID[0] ^= 1
		err := m.CheckUpdate(newState, 0, nil, 0)
		require.Error(t, err)
		assert.Equal(t, "new state's ID doesn't match", err.Error())
	}
	// Invalid AppDef
	{
		newState := newState.Clone()
		newState.App = test.NewRandomApp(rng)
		err := m.CheckUpdate(newState, 0, nil, 0)
		require.Error(t, err)
		assert.Equal(t, "new state's App dosen't match", err.Error())
	}
	// Transition from final
	/*{
		newState := state.Clone()
		newState.IsFinal = true
		err := m.CheckUpdate(newState, 0, nil, 0)
		require.True(t, channel.IsStateTransitionError(err))
		assert.Equal(t, "cannot advance final state", err.Error())
	}*/
	// Version not incremented by one
	{
		err := m.CheckUpdate(state, 0, nil, 0)
		require.True(t, channel.IsStateTransitionError(err))
		assert.True(t, strings.Contains(err.Error(), "version must increase by one"))
	}
	// Invalid Allocation
	{
		newState := newState.Clone()
		newState.Allocation.Assets = nil
		err := m.CheckUpdate(newState, 0, nil, 0)
		require.True(t, channel.IsStateTransitionError(err))
		assert.True(t, strings.Contains(err.Error(), "invalid allocation"))
	}
	// Inequal Allocation Sum
	{
		newState := newState.Clone()
		newState.Allocation.OfParts[0][0].Add(big.NewInt(1), newState.Allocation.OfParts[0][0])
		err := m.CheckUpdate(newState, 0, nil, 0)
		require.True(t, channel.IsStateTransitionError(err))
		assert.True(t, strings.Contains(err.Error(), "allocations must be preserved"))
	}
}

func testMachineClone(params channel.Params, acc wallet.Account, t *testing.T) {
	sm, err := channel.NewStateMachine(acc, params)
	require.NoError(t, err)
	pkgtest.VerifyClone(t, sm)

	am, err := channel.NewActionMachine(acc, params)
	require.NoError(t, err)
	pkgtest.VerifyClone(t, am)
}

func testStateMachineTransitions(t *testing.T, params *channel.Params, accs []wallet.Account, m *channel.StateMachine) {
	rng := rand.New(rand.NewSource(0xDDDDD))
	initAlloc := test.NewRandomAllocation(rng, len(params.Parts))
	initData := channel.NewMockOp(channel.OpValid)

	r := &setup{
		Params:    params,
		Accs:      accs,
		InitAlloc: initAlloc,
		InitData:  initData,
		State:     nil,
		rng:       rng,
		t:         t,
	}

	pkgtest.VerifyClone(t, r)
	depthReached, err := checkInitActing(r, m, 100)
	if depthReached {
		t.Log("Search depth reached")
	}
	assert.NoError(t, err)
}

// verifyStep checks that the induction step holds.
func verifyStep(t *testing.T, r *setup, m *channel.StateMachine, phase channel.Phase) {
	require := assert.New(t)

	require.Equal(r.Params.ID(), m.ID())
	require.Equal(r.Accs[0].Address(), m.Account().Address())
	require.Equal(channel.Index(0), m.Idx())
	require.Equal(r.Params, m.Params())
	require.Equal(len(r.Params.Parts), int(m.N()))
	require.Equal(phase, m.Phase())

	// Check correct state
	if phase == channel.InitSigning || phase == channel.Signing {
		require.Equal(r.State, m.StagingState(), "Wrong staging state")
	} else {
		require.Equal(r.State, m.State(), "Wrong current state")
	}

	// Test the generated AdjudicatorReq
	req := m.AdjudicatorReq()
	require.Equal(r.Params, req.Params)
	require.Equal(req.Acc.Address(), r.Accs[0].Address())
	require.Equal(channel.Index(0), req.Idx)
	require.Equal(req.Tx.State, m.State())
	// Check that we get a full signed state in phases where it is important.
	if phase == channel.Funding || phase == channel.Acting || phase >= channel.Final {
		require.Equal(len(r.Params.Parts), len(req.Tx.Sigs))
		for i := range req.Tx.Sigs {
			require.True(channel.Verify(r.Accs[i].Address(), r.Params, req.Tx.State, req.Tx.Sigs[i]))
		}
	}
	// Check that the machine is cloneable in every phases
	pkgtest.VerifyClone(t, m)
}

func checkInitActing(r *setup, m *channel.StateMachine, depth uint) (bool, error) {
	verifyStep(r.t, r, m, channel.InitActing)

	// Machine that will go to 'InitSigning' phase
	r.InitAlloc = test.NewRandomAllocation(r.rng, len(r.Params.Parts))
	return transitionTo(goInitActToInitSig, checkInitSigning, r, m, depth)
}

func checkInitSigning(r *setup, m *channel.StateMachine, depth uint) (bool, error) {
	verifyStep(r.t, r, m, channel.InitSigning)

	if r.SigningIdx < len(r.Params.Parts) {
		// Machine that will self transition only when signatures are missing
		return transitionTo(goSigningToSigning, checkInitSigning, r, m, depth)
	} else {
		// Machine that will go to 'Funding' phase only when all signatures are added
		return transitionTo(goInitSigToFunding, checkFunding, r, m, depth)
	}
}

func checkFunding(r *setup, m *channel.StateMachine, depth uint) (bool, error) {
	verifyStep(r.t, r, m, channel.Funding)
	r.SigningIdx = 0

	return transitionTos([]Transition{goFundingToActing, goToRegistering, goToRegistered}, checkActing, r, m, depth)
}

func checkActing(r *setup, m *channel.StateMachine, depth uint) (bool, error) {
	verifyStep(r.t, r, m, channel.Acting)
	r.SigningIdx = 0

	// Machine that will go to 'Signing' phase with non-Final state
	{
		r := r.Clone()
		m := m.Clone()
		if d, err := transitionTo(goActingToSigning, checkSigning, r, m, depth); err != nil {
			return d, err
		}
	}
	// Machine that will go to 'Signing' phase with Final state
	r.State.IsFinal = true
	return transitionTo(goActingToSigning, checkSigning, r, m, depth)
}

func checkSigning(r *setup, m *channel.StateMachine, depth uint) (bool, error) {
	verifyStep(r.t, r, m, channel.Signing)

	if r.SigningIdx < len(r.Params.Parts) {
		// Machine that will self transition only when signatures are missing
		return transitionTo(goSigningToSigning, checkSigning, r, m, depth)
	} else if r.State.IsFinal {
		// Machine that will go to 'Final' phase only for Final states
		return transitionTo(goToFinal, checkFinal, r, m, depth)
	} else {
		// Machine that will go to 'Acting' phase with non-Final state
		return transitionTo(goSigningToActing, checkActing, r, m, depth)
	}
}

func checkFinal(r *setup, m *channel.StateMachine, depth uint) (bool, error) {
	verifyStep(r.t, r, m, channel.Final)

	// Machine that will go to 'Registering' phase
	return transitionTo(goToRegistering, checkRegistering, r, m, depth)
}

func checkRegistering(r *setup, m *channel.StateMachine, depth uint) (bool, error) {
	verifyStep(r.t, r, m, channel.Registering)

	return transitionTo(goToRegistered, checkRegistering, r, m, depth)
}

func checkRegistered(r *setup, m *channel.StateMachine, depth uint) (bool, error) {
	verifyStep(r.t, r, m, channel.Registered)

	return transitionTo(goToWithdrawing, checkRegistering, r, m, depth)
}

func checkWithdrawing(r *setup, m *channel.StateMachine, depth uint) (bool, error) {
	verifyStep(r.t, r, m, channel.Withdrawing)

	return transitionTo(goToWithdrawn, checkRegistering, r, m, depth)
}

func checkWithdrawn(r *setup, m *channel.StateMachine, depth uint) (bool, error) {
	verifyStep(r.t, r, m, channel.Withdrawn)

	if err := callAllExcept([]Transition{}, r, m); err != nil {
		return true, errors.WithMessagef(err, "now in phase %v", m.Phase())
	}

	// This is an accepting state
	return true, nil
}

// goInitActToInitSig transition InitActing->InitSigning
func goInitActToInitSig(r *setup, m *channel.StateMachine) error {
	if err := m.Init(*r.InitAlloc, r.InitData); err != nil {
		return err
	}

	// Create the new state and check that the statemachine staged it correctly
	r.State = &channel.State{
		ID:         r.Params.ID(),
		Version:    0,
		App:        r.Params.App,
		Allocation: *r.InitAlloc,
		Data:       r.InitData,
	}
	require.True(r.t, reflect.DeepEqual(m.StagingState(), r.State))
	return nil
}

// goInitSigToFunding transition InitSigning->Funding
func goInitSigToFunding(r *setup, m *channel.StateMachine) error {
	return m.EnableInit()
}

// goFundingToActing transition InitSigning->Acting
func goFundingToActing(r *setup, m *channel.StateMachine) error {
	return m.SetFunded()
}

// goActingToSigning transition Acting->Signing
func goActingToSigning(r *setup, m *channel.StateMachine) error {
	if r.State == nil {
		return errors.New("Needs state")
	}
	r.State.Version++
	return m.Update(r.State, m.Idx())
}

// goActingToRegistering transition Signing->Registering
//var goSigningToRegistering = goFundingToRegistering

// goSigningToActing transition Signing->Acting
func goSigningToActing(r *setup, m *channel.StateMachine) error {
	return m.EnableUpdate()
}

// goSigningToSigning modells the transitions Signing->Signing
// AND InitSigning->InitSigning
func goSigningToSigning(r *setup, m *channel.StateMachine) error {
	var sig wallet.Sig
	var err error

	if r.SigningIdx >= len(r.Params.Parts) {
		return errors.New("Already all signatures added")
	}

	if r.SigningIdx == 0 {
		sig, err = m.Sig()

		if err != nil {
			return errors.WithMessagef(err, "could create signature")
		}
	} else {
		sig, err = channel.Sign(r.Accs[r.SigningIdx], r.Params, r.State)
		require.NoError(r.t, err)

		if err := verifyAddSig(m, channel.Index(r.SigningIdx), sig); err != nil {
			return errors.WithMessagef(err, "Could not add message for peer %d", r.SigningIdx)
		}
	}
	r.SigningIdx++

	return nil
}

// goFinalToRegistering transition Final->Registering
//var goFinalToRegistering = goFundingToRegistering

// goSigningToFinal transition Signing->Final
func goToFinal(r *setup, m *channel.StateMachine) error {
	return m.EnableFinal()
}

func goToRegistering(r *setup, m *channel.StateMachine) error {
	return m.SetRegistering()
}

func goToRegistered(r *setup, m *channel.StateMachine) error {
	var v uint64 = 0
	if r.State != nil {
		v = r.State.Version
	}

	e := &channel.RegisteredEvent{
		ID:      r.Params.ID(),
		Version: v,
		Timeout: time.Now().Add(time.Hour),
	}
	return m.SetRegistered(e)
}

func goToWithdrawing(r *setup, m *channel.StateMachine) error {
	return m.SetWithdrawing()
}

func goToWithdrawn(r *setup, m *channel.StateMachine) error {
	return m.SetWithdrawn()
}

func verifyAddSig(m *channel.StateMachine, i channel.Index, sig wallet.Sig) error {
	orig := m.Clone()
	wrongIdx := (i + 1) % uint16(len(m.Params().Parts))
	fakeSig := make([]byte, len(sig))
	copy(fakeSig, sig)
	// Invalidate the signature and check for error
	fakeSig[0] = ^sig[0]

	if err := m.AddSig(wrongIdx, sig); err == nil {
		return errors.New("Machine did not detect wrong IDx")
	}
	if err := m.AddSig(i, fakeSig); err == nil {
		return errors.New("Machine did not detect wrong Signature")
	}
	if !reflect.DeepEqual(orig, m) {
		return errors.New("Machine should stay the same")
	}
	return m.AddSig(i, sig)
}
