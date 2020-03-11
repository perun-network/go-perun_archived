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

func TestPhase(t *testing.T) {
	ps := []channel.Phase{channel.InitActing, channel.InitSigning, channel.Funding, channel.Acting, channel.Signing, channel.Final, channel.Settled}

	t.Run("String()", func(t *testing.T) {
		concat := ""
		for _, p := range ps {
			concat = concat + p.String() + " "
		}
		assert.Equal(t, concat, "InitActing InitSigning Funding Acting Signing Final Settled ")
	})

	t.Run("PhaseTransition", func(t *testing.T) {
		for _, i := range ps {
			for _, j := range ps {
				ts := channel.PhaseTransition{i, j}
				str := i.String() + "->" + j.String()

				assert.Equal(t, str, ts.String())
			}
		}
	})
}

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

	t.Run("get/set", func(t *testing.T) {
		testStateMachineGetSet(t, params, accs, m.Clone())
	})

	t.Run("transitions", func(t *testing.T) {
		testStateMachineTransitions(t, params, accs, m)
	})
}

func testStateMachineGetSet(t *testing.T, params *channel.Params, accs []wallet.Account, m *channel.StateMachine) {
	assert := assert.New(t)

	assert.Equal(params.ID(), m.ID())
	assert.Equal(accs[0].Address(), m.Account().Address())
	assert.Equal(channel.Index(0), m.Idx())
	assert.Equal(params, m.Params())
	assert.Equal(len(params.Parts), int(m.N()))
	assert.Equal(channel.InitActing, m.Phase())
	assert.Equal((*channel.State)(nil), m.State())
}

func testStateMachineTransitions(t *testing.T, params *channel.Params, accs []wallet.Account, m *channel.StateMachine) {
	rng := rand.New(rand.NewSource(0xDDDDD))

	initAlloc := test.NewRandomAllocation(rng, len(params.Parts))
	initData := channel.NewMockOp(channel.OpValid)
	state, err := channel.NewState(params, *initAlloc, initData)
	state.Version = 0
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
	depthReached, err := checkInitActing(r, m, 100)
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

type State func(*round, *channel.StateMachine, uint) (bool, error)

var transitions []Transition

func init() {
	transitions = []Transition{
		goActingToSigning,
		goFinalToSettled,
		goFundingToActing,
		goInitActToInitSig,
		goInitSigToFunding,
		goSigningToActing,
		goSigningToFinal,
		goSigningToSigning,
	}
}

func callAllExcept(expect []Transition, r *round, m *channel.StateMachine) error {
outer:
	for _, ts := range transitions {
		t := functionName(ts)
		for _, ex := range expect {
			e := functionName(ex)
			if t == e {
				continue outer
			}
		}
		r.t.Logf("try   %s", t)
		r = r.Clone()
		m = m.Clone()
		if err := ts(r, m); err == nil {
			return errors.Errorf("transition '%s' from phase %s did not abort or produce error", t, m.Phase())
		}
	}

	return nil
}

func transitionTo(arrow Transition, state State, r *round, m *channel.StateMachine, depth uint) (bool, error) {
	if depth == 0 {
		return true, nil
	}
	if err := callAllExcept([]Transition{arrow}, r, m); err != nil {
		return false, err
	}
	r.t.Logf("take  %s -> %s\n\n", functionName(arrow), functionName(state))

	clonedR := r.Clone()
	cloned := m.Clone()

	if err := arrow(clonedR, cloned); err != nil {
		return false, err
	}

	if _, err := state(clonedR, cloned, depth-1); err != nil {
		return false, err
	}

	return false, nil
}

func functionName(f interface{}) string {
	return filepath.Ext(runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name())[1:]
}

func checkInitActing(r *round, m *channel.StateMachine, depth uint) (bool, error) {
	if m.Phase() != channel.InitActing {
		return false, errors.New("Started in wrong phase")
	}

	// Machine that will go to 'InitSigning' phase
	r.InitAlloc = test.NewRandomAllocation(r.rng, len(r.Params.Parts))
	return transitionTo(goInitActToInitSig, checkInitSigning, r, m, depth)
}

func checkInitSigning(r *round, m *channel.StateMachine, depth uint) (bool, error) {
	if m.Phase() != channel.InitSigning {
		return false, errors.New("Started in wrong phase")
	}

	if r.SigningIdx < len(r.Params.Parts) {
		// Machine that will self transition only when signatures are missing
		return transitionTo(goSigningToSigning, checkInitSigning, r, m, depth)
	} else {
		// Machine that will go to 'Funding' phase only when all signatures are added
		return transitionTo(goInitSigToFunding, checkFunding, r, m, depth)
	}
}

func checkFunding(r *round, m *channel.StateMachine, depth uint) (bool, error) {
	if m.Phase() != channel.Funding {
		return false, errors.New("Started in wrong phase")
	}
	r.SigningIdx = 0

	return transitionTo(goFundingToActing, checkActing, r, m, depth)
}

func checkActing(r *round, m *channel.StateMachine, depth uint) (bool, error) {
	if m.Phase() != channel.Acting {
		return false, errors.New("Started in wrong phase")
	}
	r.SigningIdx = 0

	return transitionTo(goActingToSigning, checkSigning, r, m, depth)
}

func checkSigning(r *round, m *channel.StateMachine, depth uint) (bool, error) {
	if m.Phase() != channel.Signing {
		return false, errors.New("Started in wrong phase")
	}

	if r.SigningIdx < len(r.Params.Parts) {
		// Machine that will self transition only when signatures are missing
		return transitionTo(goSigningToSigning, checkSigning, r, m, depth)
	} else if r.State.IsFinal {
		// Machine that will go to 'Final' phase only for Final states
		return transitionTo(goSigningToFinal, checkFinal, r, m, depth)
	} else {
		// Machine that will go to 'Acting' phase with non-Final state
		if d, err := transitionTo(goSigningToActing, checkActing, r, m, depth); err != nil {
			return d, err
		}
		// Machine that will go to 'Acting' phase with Final state
		r.State.IsFinal = true
		return transitionTo(goSigningToActing, checkActing, r, m, depth)
	}
}

func checkFinal(r *round, m *channel.StateMachine, depth uint) (bool, error) {
	if m.Phase() != channel.Final {
		return false, errors.New("Started in wrong phase")
	}

	// Machine that will go to 'Settled' phase
	return transitionTo(goFinalToSettled, checkSettled, r, m, depth)
}

func checkSettled(r *round, m *channel.StateMachine, depth uint) (bool, error) {
	if m.Phase() != channel.Settled {
		return true, errors.New("Started in wrong phase")
	}

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
	r.State.Version++
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
