// Copyright (c) 2020 Chair of Applied Cryptography, Technische Universität
// Darmstadt, Germany. All rights reserved. This file is part of go-perun. Use
// of this source code is governed by a MIT-style license that can be found in
// the LICENSE file.

package channel

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// finalPhase is the last value of the Phase enum.
const finalPhase = Settled

func Test_machine_expect(t *testing.T) {
	t.Run("wrong from", func(t *testing.T) {
		m := &machine{phase: InitActing}
		assert.Error(t, m.expect(PhaseTransition{Funding, Funding}))
	})

	tests := map[Phase][]Phase{
		InitActing:  []Phase{InitSigning},
		InitSigning: []Phase{Funding},
		Funding:     []Phase{Acting},
		Acting:      []Phase{Signing},
		Signing:     []Phase{Acting, Final},
		Final:       []Phase{Settled},
		Settled:     nil,
	}

	// Helper function.
	var contains = func(slice []Phase, x Phase) bool {
		for _, y := range slice {
			if x == y {
				return true
			}
		}
		return false
	}

	for phase := Phase(0); phase <= finalPhase; phase++ {
		test, ok := tests[phase]
		if !ok {
			t.Fatalf("phase %v has no test case", phase)
		}

		for next := Phase(0); next <= finalPhase; next++ {
			m := machine{phase: phase}
			err := m.expect(PhaseTransition{phase, next})
			if contains(test, next) {
				assert.NoError(t, err, "phase transition should be allowed")
			} else {
				assert.Error(t, err, "phase transition should be rejected")
			}
		}
	}
}
