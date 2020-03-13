// Copyright (c) 2020 Chair of Applied Cryptography, Technische Universität
// Darmstadt, Germany. All rights reserved. This file is part of go-perun. Use
// of this source code is governed by a MIT-style license that can be found in
// the LICENSE file.

package channel_test

import (
	"math/rand"
	"reflect"
	"testing"

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

func testMachineClone(params channel.Params, acc wallet.Account, t *testing.T) {
	for i := 0; i < 100; i++ {
		m, err := channel.NewMachine(acc, params)
		require.NoError(t, err)
		pkgtest.VerifyClone(t, m)

		sm, err := channel.NewStateMachine(acc, params)
		require.NoError(t, err)
		pkgtest.VerifyClone(t, sm)

		am, err := channel.NewActionMachine(acc, params)
		require.NoError(t, err)
		pkgtest.VerifyClone(t, am)
	}
}

func testStateMachineTransitions(t *testing.T, params *channel.Params, accs []wallet.Account, m *channel.StateMachine) {
	rng := rand.New(rand.NewSource(0xDDDDD))
	initAlloc := test.NewRandomAllocation(rng, len(params.Parts))
	initData := channel.NewMockOp(channel.OpValid)
	/*state, err := channel.NewState(params, *initAlloc, initData)
	require.NoError(t, err)
	state.Version = 0*/

	r := &round{
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
func verifyStep(t *testing.T, r *round, m *channel.StateMachine, phase channel.Phase) {
	require := assert.New(t)

	require.Equal(r.Params.ID(), m.ID())
	require.Equal(r.Accs[0].Address(), m.Account().Address())
	require.Equal(channel.Index(0), m.Idx())
	require.Equal(r.Params, m.Params())
	require.Equal(len(r.Params.Parts), int(m.N()))
	require.Equal(phase, m.Phase())

	// Check correct state
	if phase == channel.InitSigning || phase == channel.Signing {
		require.Equal(r.State, m.StagingState(), "Wrong stating state")
	} else {
		require.Equal(r.State, m.State(), "Wrong current state")
	}
	// TODO Check AdjudicatorReq, the call does not always make sense
}

func checkInitActing(r *round, m *channel.StateMachine, depth uint) (bool, error) {
	verifyStep(r.t, r, m, channel.InitActing)

	// Machine that will go to 'InitSigning' phase
	r.InitAlloc = test.NewRandomAllocation(r.rng, len(r.Params.Parts))
	return transitionTo(goInitActToInitSig, checkInitSigning, r, m, depth)
}

func checkInitSigning(r *round, m *channel.StateMachine, depth uint) (bool, error) {
	verifyStep(r.t, r, m, channel.InitSigning)

	if r.SigningIdx < len(r.Params.Parts) {
		// Machine that will self transition only when signatures are missing
		return transitionTo(goSigningToSigning, checkInitSigning, r, m, depth)
	} else {
		// Machine that will go to 'Funding' phase only when all signatures are added
		return transitionTo(goInitSigToFunding, checkFunding, r, m, depth)
	}
}

func checkFunding(r *round, m *channel.StateMachine, depth uint) (bool, error) {
	verifyStep(r.t, r, m, channel.Funding)
	r.SigningIdx = 0

	return transitionTo(goFundingToActing, checkActing, r, m, depth)
}

func checkActing(r *round, m *channel.StateMachine, depth uint) (bool, error) {
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

func checkSigning(r *round, m *channel.StateMachine, depth uint) (bool, error) {
	verifyStep(r.t, r, m, channel.Signing)

	if r.SigningIdx < len(r.Params.Parts) {
		// Machine that will self transition only when signatures are missing
		return transitionTo(goSigningToSigning, checkSigning, r, m, depth)
	} else if r.State.IsFinal {
		// Machine that will go to 'Final' phase only for Final states
		return transitionTo(goSigningToFinal, checkFinal, r, m, depth)
	} else {
		// Machine that will go to 'Acting' phase with non-Final state
		return transitionTo(goSigningToActing, checkActing, r, m, depth)
	}
}

func checkFinal(r *round, m *channel.StateMachine, depth uint) (bool, error) {
	verifyStep(r.t, r, m, channel.Final)

	// Machine that will go to 'Settled' phase
	return transitionTo(goFinalToSettled, checkSettled, r, m, depth)
}

func checkSettled(r *round, m *channel.StateMachine, depth uint) (bool, error) {
	verifyStep(r.t, r, m, channel.Settled)

	// Check that all invalid transitions are invalid
	if err := callAllExcept([]Transition{}, r, m); err != nil {
		return true, errors.WithMessagef(err, "now in phase %v", m.Phase())
	}

	// This is an accepting state
	return true, nil
}

/*func checkRegistering(r *round, m *channel.StateMachine, depth uint) (bool, error) {
	// Phase not yet implemented
	return true, nil
}*/

// goInitActToInitSig transition InitActing->InitSigning
func goInitActToInitSig(r *round, m *channel.StateMachine) error {
	if err := m.Init(*r.InitAlloc, r.InitData); err != nil {
		return err
	}
	// Create the new state and check that the statemachine staged it correctly
	state, err := channel.NewState(r.Params, *r.InitAlloc, r.InitData)
	require.NoError(r.t, err)
	r.State = state
	require.True(r.t, reflect.DeepEqual(m.StagingState(), r.State))

	return nil
}

// goInitSigToFunding transition InitSigning->Funding
func goInitSigToFunding(r *round, m *channel.StateMachine) error {
	return m.EnableInit()
}

// goFundingToActing transition InitSigning->Acting
func goFundingToActing(r *round, m *channel.StateMachine) error {
	return m.SetFunded()
}

// goFundingToRegistering transition Funding->Registering
//func goFundingToRegistering(r *round, m *channel.StateMachine) error {
//	return nil
//}

// goActingToRegistering transition Acting->Registering
//var goActingToRegistering = goFundingToRegistering

// goActingToSigning transition Acting->Signing
func goActingToSigning(r *round, m *channel.StateMachine) error {
	if r.State == nil {
		return errors.New("Needs state")
	}
	r.State.Version++
	return m.Update(r.State, m.Idx())
}

// goActingToRegistering transition Signing->Registering
//var goSigningToRegistering = goFundingToRegistering

// goSigningToActing transition Signing->Acting
func goSigningToActing(r *round, m *channel.StateMachine) error {
	return m.EnableUpdate()
}

// goSigningToSigning modells the transitions Signing->Signing
// AND InitSigning->InitSigning
func goSigningToSigning(r *round, m *channel.StateMachine) error {
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

// goFinalToRegistering transition Final->Registering
//var goFinalToRegistering = goFundingToRegistering

// goSigningToFinal transition Signing->Final
func goSigningToFinal(r *round, m *channel.StateMachine) error {
	return m.EnableFinal()
}

// goFinalToSettled transition Final->Settled
func goFinalToSettled(r *round, m *channel.StateMachine) error {
	return m.SetSettled()
}
