// Copyright (c) 2020 Chair of Applied Cryptography, Technische Universität
// Darmstadt, Germany. All rights reserved. This file is part of go-perun. Use
// of this source code is governed by a MIT-style license that can be found in
// the LICENSE file.

package channel_test

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
	"perun.network/go-perun/channel"
	"perun.network/go-perun/channel/test"
	pkgtest "perun.network/go-perun/pkg/test"
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
	}
}
