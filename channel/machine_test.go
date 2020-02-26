// Copyright (c) 2020 Chair of Applied Cryptography, Technische Universität
// Darmstadt, Germany. All rights reserved. This file is part of go-perun. Use
// of this source code is governed by a MIT-style license that can be found in
// the LICENSE file.

package channel_test

import (
	"math/rand"
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

type round struct {
	Params    *channel.Params
	Accs      []wallet.Account `cloneable:"shallow"`
	InitAlloc *channel.Allocation
	InitData  channel.Data

	InitSigningIdx int
	InitSigningSig wallet.Sig
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
		InitSigningSig: r.InitSigningSig,
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
		gotoInitSigningInit,
		goToInitSigningAddSig,
		goToFundingEnableInit,
	}
}

func callAllExcept(expect []Transition, r *round, m *channel.StateMachine) error {
outer:
	for i, ts := range transitions {
		for _, ex := range expect {
			if runtime.FuncForPC(reflect.ValueOf(ex).Pointer()).Name() ==
				runtime.FuncForPC(reflect.ValueOf(ts).Pointer()).Name() {
				continue outer
			}
		}
		r.t.Logf("now calling %s", runtime.FuncForPC(reflect.ValueOf(ts).Pointer()).Name())
		if err := ts(r, m); err == nil {
			return errors.Errorf("transition #%d did not abort or produce error", i)
		}
	}

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
		if err := goInitActToInitSig(r, m); err != nil {
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
	if err := callAllExcept([]Transition{goInitSigToInitSig, goInitSigToFunding}, r, m); err != nil {
		return false, err
	}
	// Machine that will self transition
	{
		cloned := m.Clone()
		if err := goInitSigToInitSig(r, cloned); err != nil {
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
	if err := callAllExcept([]Transition{goFundingToActing, goFundingToRegistering}, r, m); err != nil {
		return false, errors.WithMessagef(err, "now in phase %v", m.Phase())
	}
	// Machine that will go to 'Acting' phase
	{
		cloned := m.Clone()
		if err := goFundingToActing(r, m); err != nil {
			return false, err
		}
		if _, err := checkActing(r, cloned, depth-1); err != nil {
			return false, err
		}
	}
	// Machine that will go to 'Registering' phase
	{
		cloned := m.Clone()
		if err := goFundingToRegistering(r, m); err != nil {
			return false, err
		}
		if _, err := checkRegistering(r, cloned, depth-1); err != nil {
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
	if err := callAllExcept([]Transition{goA, goFundingToRegistering}, r, m); err != nil {
		return false, errors.WithMessagef(err, "now in phase %v", m.Phase())
	}
	// Machine that will go to 'Acting' phase
	{
		cloned := m.Clone()
		if err := goFundingToActing(r, m); err != nil {
			return false, err
		}
		if _, err := checkActing(r, cloned, depth-1); err != nil {
			return false, err
		}
	}
	// Machine that will go to 'Registering' phase
	{
		cloned := m.Clone()
		if err := goFundingToRegistering(r, m); err != nil {
			return false, err
		}
		if _, err := checkRegistering(r, cloned, depth-1); err != nil {
			return false, err
		}
	}

	return false, nil
}

func checkRegistering(r *round, m *channel.StateMachine, depth uint) (bool, error) {
	// Phase not yet implemented
	return true, nil
}

// goInitActToInitSig transition InitActing->InitSigning
func goInitActToInitSig(r *round, m *channel.StateMachine) error {
	if err := m.Init(*r.InitAlloc, r.InitData); err != nil {
		return err
	}

	// The newState creation should be unit tested somewhere else
	r.State = m.StagingState()
	require.NotNil(r.t, r.State)
	r.InitSigningIdx = 0
	sig, err := channel.Sign(r.Accs[0], r.Params, r.State)
	require.NoError(r.t, err)
	r.InitSigningSig = sig

	return nil
}

// goInitSigToInitSig transition InitSigning->InitSigning
func goInitSigToInitSig(r *round, m *channel.StateMachine) error {
	if err := m.AddSig(channel.Index(r.InitSigningIdx), r.InitSigningSig); err != nil {
		return err
	}

	if r.InitSigningIdx+1 < len(r.Params.Parts) {
		r.InitSigningIdx++
		sig, err := channel.Sign(r.Accs[r.InitSigningIdx], r.Params, r.State)
		require.NoError(r.t, err)
		r.InitSigningSig = sig
	}

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
func goFundingToRegistering(r *round, m *channel.StateMachine) error {
	return nil
}

// goActingToRegistering transition Acting->Registering
var goActingToRegistering = goFundingToRegistering

// goActingToSigning transition Acting->Signing
func goActingToSigning(r *round, m *channel.StateMachine) error {
	r.State = test.NewRandomState(r.rng, r.Params)
	return m.Update(r.State, m.Idx())
}

// goActingToRegistering transition Signing->Registering
var goSigningToRegistering = goFundingToRegistering

// goSigningToActing transition Signing->Acting
func goSigningToActing(r *round, m *channel.StateMachine) error {
	return m.EnableUpdate()
}

// goSigningToActing transition Signing->Acting
func goSigningToSigning(r *round, m *channel.StateMachine) error {
	sig, err := m.Sig()
	require.NoError(r.t, err)
	return m.AddSig(m.Idx(), sig)
}

// goFinalToRegistering transition Final->Registering
var goFinalToRegistering = goFundingToRegistering

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

	r := &round{
		Params:    params,
		Accs:      accs,
		InitAlloc: test.NewRandomAllocation(rng, len(params.Parts)),
		InitData:  channel.NewMockOp(channel.OpValid),
		State:     test.NewRandomState(rng, params),
		rng:       rng,
		t:         t,
	}

	pkgtest.VerifyClone(t, r)
	assert.NoError(t, checkInitActing(r, m, 10))
}
