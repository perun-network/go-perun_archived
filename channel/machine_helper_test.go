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
	"perun.network/go-perun/channel"
	"perun.network/go-perun/wallet"
)

func TestPhase(t *testing.T) {
	ps := []channel.Phase{channel.InitActing, channel.InitSigning, channel.Funding, channel.Acting, channel.Signing, channel.Final, channel.Settled}

	t.Run("Phase.String()", func(t *testing.T) {
		concat := ""
		for _, p := range ps {
			concat = concat + p.String() + " "
		}
		assert.Equal(t, concat, "InitActing InitSigning Funding Acting Signing Final Settled ")
	})
	t.Run("PhaseTransition.String()", func(t *testing.T) {
		for _, i := range ps {
			for _, j := range ps {
				ts := channel.PhaseTransition{i, j}
				str := i.String() + "->" + j.String()
				assert.Equal(t, str, ts.String())
			}
		}
	})
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
	var state *channel.State
	if r.State != nil {
		state = r.State.Clone()
	}

	return &round{
		Params:         r.Params.Clone(),
		Accs:           r.Accs,
		InitAlloc:      &a,
		InitData:       r.InitData.Clone(),
		InitSigningIdx: r.InitSigningIdx,
		SigningIdx:     r.SigningIdx,
		State:          state,
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

		r.t.Logf("try %s", t)
		r = r.Clone()
		clonedM := m.Clone()
		if err := ts(r, clonedM); err == nil {
			return errors.Errorf("transition '%s' from phase %v did not abort or produce error", functionName(ts), m.Phase())
		} /*else if !reflect.DeepEqual(m, clonedM) {
			return errors.Errorf("transition '%s' changed the machine state", functionName(ts))
		}*/
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
	r.t.Logf("take (%d) %s -> %s\n\n", depth, functionName(arrow), functionName(state))

	r = r.Clone()
	m = m.Clone()
	if err := arrow(r, m); err != nil {
		return false, err
	}
	if _, err := state(r, m, depth-1); err != nil {
		return false, err
	}

	return false, nil
}

func functionName(f interface{}) string {
	return filepath.Ext(runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name())[1:]
}
