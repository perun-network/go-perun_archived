// Copyright (c) 2020 Chair of Applied Cryptography, Technische Universität
// Darmstadt, Germany. All rights reserved. This file is part of go-perun. Use
// of this source code is governed by a MIT-style license that can be found in
// the LICENSE file.

package channel_test

import (
	"math/rand"
	"path/filepath"
	"reflect"
	"runtime"
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

func TestMachineClone(t *testing.T) {
	rng := rand.New(rand.NewSource(0xDDDDD))

	for i := 0; i < 100; i++ {
		app := test.NewRandomApp(rng)
		params := *test.NewRandomParams(rng, app.Def())
		acc := wtest.NewRandomAccount(rng)
		params.Parts[0] = acc.Address()

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

func TestStateMachine(t *testing.T) {
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
	state, err := channel.NewState(params, *initAlloc, initData)
	require.NoError(t, err)

	r := &round{
		Params:    params,
		Accs:      accs,
		InitAlloc: initAlloc,
		InitData:  initData,
		// The constructor fot State is private
		State: state,
		rng:   rng,
		t:     t,
	}

	pkgtest.VerifyClone(t, r)
	depthReached, err := checkInitActing(r, m, 10)
	if depthReached {
		t.Log("Search depth reached")
	}
	assert.NoError(t, err)
}

type round struct {
	Params    *channel.Params
	Accs      []wallet.Account `cloneable:"shallow"`
	InitAlloc *channel.Allocation
	InitData  channel.Data

	InitSigningIdx int
	SigningIdx     int
	State          *channel.State

	rng *rand.Rand `cloneable:"shallow"`
	t   *testing.T `cloneable:"shallow"`
}

func (r *round) Clone() *round {
	a := r.InitAlloc.Clone()
	return &round{
		Params:         r.Params.Clone(),
		Accs:           r.Accs,
		InitAlloc:      &a,
		InitData:       r.InitData.Clone(),
		InitSigningIdx: r.InitSigningIdx,
		SigningIdx:     r.SigningIdx,
		State:          r.State.Clone(),
		rng:            r.rng,
		t:              r.t,
	}
}

// Return value: (depth_reached bool, err error)
type Transition func(*round, *channel.StateMachine) error

var transitions []Transition

func init() {
	transitions = []Transition{
		goActingToSigning,
		goFinalToSettled,
		goFundingToActing,
		goInitActToInitSig,
		goInitSigToFunding,
		//goInitSigToInitSig,
		goSigningToActing,
		goSigningToFinal,
		goSigningToSigning,
	}
}

func callAllExcept(expect []Transition, r *round, m *channel.StateMachine) error {
outer:
	for _, ts := range transitions {
		t := filepath.Ext(runtime.FuncForPC(reflect.ValueOf(ts).Pointer()).Name())
		for _, ex := range expect {
			e := filepath.Ext(runtime.FuncForPC(reflect.ValueOf(ex).Pointer()).Name())
			if t == e {
				r.t.Logf("skip  %s", e)
				continue outer
			}
		}
		r.t.Logf("try   %s", t)
		if err := ts(r, m); err == nil {
			return errors.Errorf("transition '%s' from phase %s did not abort or produce error", t, m.Phase())
		}
	}
	r.t.Log()

	return nil
}

func checkInitActing(r *round, m *channel.StateMachine, depth uint) (bool, error) {
	if m.Phase() != channel.InitActing {
		return false, errors.New("Started in wrong phase")
	}
	if depth == 0 {
		return true, nil
	}

	// Check that all invalid transitions are invalid
	if err := callAllExcept([]Transition{goInitActToInitSig}, r, m); err != nil {
		return false, errors.WithMessagef(err, "now in phase %v", m.Phase())
	}
	// Machine that will go to 'InitSigning' phase
	{
		cloned := m.Clone()
		r.InitAlloc = test.NewRandomAllocation(r.rng, len(r.Params.Parts))
		if err := goInitActToInitSig(r, cloned); err != nil {
			return false, err
		}
		if _, err := checkInitSigning(r, cloned, depth-1); err != nil {
			return false, err
		}
	}

	return false, nil
}

func checkInitSigning(r *round, m *channel.StateMachine, depth uint) (bool, error) {
	if m.Phase() != channel.InitSigning {
		return false, errors.New("Started in wrong phase")
	}
	if depth == 0 {
		return true, nil
	}

	// Check that all invalid transitions are invalid
	if err := callAllExcept([]Transition{goSigningToSigning, goInitSigToFunding}, r, m); err != nil {
		return false, err
	}
	// Machine that will self transition
	{
		cloned := m.Clone()
		if err := goSigningToSigning(r, cloned); err != nil {
			return false, err
		}
		if _, err := checkInitSigning(r, cloned, depth-1); err != nil {
			return false, err
		}
	}
	// Machine that will go to 'Funding' phase
	{
		cloned := m.Clone()
		if err := goInitSigToFunding(r, cloned); err == nil {
			return false, err
		}
		if _, err := checkFunding(r, cloned, depth-1); err != nil {
			return false, err
		}
	}

	return false, nil
}

func checkFunding(r *round, m *channel.StateMachine, depth uint) (bool, error) {
	if m.Phase() != channel.Funding {
		return false, errors.New("Started in wrong phase")
	}
	if depth == 0 {
		return true, nil
	}

	// Check that all invalid transitions are invalid
	if err := callAllExcept([]Transition{goFundingToActing}, r, m); err != nil {
		return false, errors.WithMessagef(err, "now in phase %v", m.Phase())
	}
	// Machine that will go to 'Acting' phase
	{
		cloned := m.Clone()
		if err := goFundingToActing(r, cloned); err != nil {
			return false, err
		}
		if _, err := checkActing(r, cloned, depth-1); err != nil {
			return false, err
		}
	}

	return false, nil
}

func checkActing(r *round, m *channel.StateMachine, depth uint) (bool, error) {
	if m.Phase() != channel.Acting {
		return false, errors.New("Started in wrong phase")
	}
	if depth == 0 {
		return true, nil
	}

	// Check that all invalid transitions are invalid
	if err := callAllExcept([]Transition{goActingToSigning}, r, m); err != nil {
		return false, errors.WithMessagef(err, "now in phase %v", m.Phase())
	}
	// Machine that will go to 'Signing' phase
	{
		cloned := m.Clone()
		if err := goActingToSigning(r, cloned); err != nil {
			return false, err
		}
		if _, err := checkSigning(r, cloned, depth-1); err != nil {
			return false, err
		}
	}

	return false, nil
}

func checkSigning(r *round, m *channel.StateMachine, depth uint) (bool, error) {
	if m.Phase() != channel.Signing {
		return false, errors.New("Started in wrong phase")
	}
	if depth == 0 {
		return true, nil
	}

	// Check that all invalid transitions are invalid
	if err := callAllExcept([]Transition{goSigningToActing, goSigningToFinal, goSigningToSigning}, r, m); err != nil {
		return false, errors.WithMessagef(err, "now in phase %v", m.Phase())
	}
	// Machine that will go to 'Acting' phase
	{
		cloned := m.Clone()
		if err := goSigningToActing(r, cloned); err != nil {
			return false, err
		}
		if _, err := checkActing(r, cloned, depth-1); err != nil {
			return false, err
		}
	}
	// Machine that will go to 'Final' phase
	{
		cloned := m.Clone()
		if err := goSigningToFinal(r, cloned); err != nil {
			return false, err
		}
		if _, err := checkFinal(r, cloned, depth-1); err != nil {
			return false, err
		}
	}
	// Machine that will go to 'Signing' phase
	{
		cloned := m.Clone()
		if err := goSigningToSigning(r, cloned); err != nil {
			return false, err
		}
		if _, err := checkSigning(r, cloned, depth-1); err != nil {
			return false, err
		}
	}

	return false, nil
}

func checkFinal(r *round, m *channel.StateMachine, depth uint) (bool, error) {
	if m.Phase() != channel.Final {
		return false, errors.New("Started in wrong phase")
	}
	if depth == 0 {
		return true, nil
	}

	// Check that all invalid transitions are invalid
	if err := callAllExcept([]Transition{goFinalToSettled}, r, m); err != nil {
		return false, errors.WithMessagef(err, "now in phase %v", m.Phase())
	}
	// Machine that will go to 'Settled' phase
	{
		cloned := m.Clone()
		if err := goFinalToSettled(r, cloned); err != nil {
			return false, err
		}
		if _, err := checkSettled(r, cloned, depth-1); err != nil {
			return false, err
		}
	}

	return false, nil
}

func checkSettled(r *round, m *channel.StateMachine, depth uint) (bool, error) {
	if m.Phase() != channel.Settled {
		return false, errors.New("Started in wrong phase")
	}
	if depth == 0 {
		return true, nil
	}

	// Check that all invalid transitions are invalid
	if err := callAllExcept([]Transition{}, r, m); err != nil {
		return false, errors.WithMessagef(err, "now in phase %v", m.Phase())
	}

	// This is an accepting state
	return false, nil
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
	//require.True(r.t, reflect.DeepEqual(m.StagingState(), r.State))

	return nil
}

// goInitSigToInitSig transition InitSigning->InitSigning
func goInitSigToInitSigs(r *round, m *channel.StateMachine) error {
	var sig wallet.Sig
	var err error

	if r.InitSigningIdx >= len(r.Params.Parts) {
		return errors.New("Already all signatures added")
	}

	if r.InitSigningIdx == 0 {
		sig, err = m.Sig()

		if err != nil {
			return errors.WithMessagef(err, "could create signature")
		}
	} else {
		sig, err = channel.Sign(r.Accs[r.InitSigningIdx], r.Params, r.State)
		require.NoError(r.t, err)

		if err := m.AddSig(channel.Index(r.InitSigningIdx), sig); err != nil {
			return errors.WithMessagef(err, "Could not add message for peer %d", r.InitSigningIdx)
		}
	}
	r.InitSigningIdx++

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
	r.State = test.NewRandomState(r.rng, r.Params)
	return m.Update(r.State, m.Idx())
}

// goActingToRegistering transition Signing->Registering
//var goSigningToRegistering = goFundingToRegistering

// goSigningToActing transition Signing->Acting
func goSigningToActing(r *round, m *channel.StateMachine) error {
	return m.EnableUpdate()
}

// goSigningToActing modells the transitions Signing->Signing
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

		if err := m.AddSig(channel.Index(r.SigningIdx), sig); err != nil {
			return errors.WithMessagef(err, "Could not add message for peer %d", r.SigningIdx)
		}
	}
	r.SigningIdx++

	return nil
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
