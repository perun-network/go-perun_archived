// Copyright (c) 2019 Chair of Applied Cryptography, Technische Universität
// Darmstadt, Germany. All rights reserved. This file is part of go-perun. Use
// of this source code is governed by a MIT-style license that can be found in
// the LICENSE file.

package payment

import (
	"math/big"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"

	"perun.network/go-perun/channel"
	wallettest "perun.network/go-perun/wallet/test"
)

func TestApp_Def(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	def := wallettest.NewRandomAddress(rng)
	app := &App{def}
	assert.True(t, def.Equals(app.Def()))
}

func TestApp_ValidInit(t *testing.T) {
	assert := assert.New(t)
	app := new(App)

	nildata := &channel.State{Data: nil}
	assert.Panics(func() { app.ValidInit(nil, nildata) })
	wrongdata := &channel.State{Data: new(channel.MockOp)}
	assert.Panics(func() { app.ValidInit(nil, wrongdata) })

	nodata := &channel.State{Data: new(NoData)}
	assert.Nil(app.ValidInit(nil, nodata))
}

func TestApp_ValidTransition(t *testing.T) {
	type (
		alloc = [][]int64
		to    struct {
			alloc
			valid int // the valid actor index, or -1 if there's no valid actor
		}
	)

	tests := []struct {
		from alloc
		tos  []to
		desc string
	}{
		{
			from: alloc{{10, 5}, {0, 20}},
			tos: []to{
				{alloc: alloc{{5, 10}, {5, 15}}, valid: -1}, // mixed
				{alloc: alloc{{5, 10}, {0, 20}}, valid: 0},
				{alloc: alloc{{12, 3}, {10, 10}}, valid: 1},
			},
			desc: "two-party",
		},
		{
			from: alloc{{10, 5, 20}, {10, 5, 20}},
			tos: []to{
				{alloc: alloc{{5, 8, 22}, {15, 3, 17}}, valid: -1}, // mixed
				{alloc: alloc{{5, 8, 22}, {0, 10, 25}}, valid: 0},
				{alloc: alloc{{15, 0, 20}, {10, 0, 25}}, valid: 1},
				{alloc: alloc{{15, 10, 10}, {15, 10, 10}}, valid: 2},
			},
			desc: "three-party",
		},
	}

	app := new(App)

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			assert := assert.New(t)
			from := newStateWithAlloc(tt.from)
			numParticipants := len(tt.from[0])
			for i := 0; i < numParticipants; i++ {
				// valid self-transition
				assert.NoError(app.ValidTransition(nil, from, from, channel.Index(i)))
			}

			for _, tto := range tt.tos {
				to := newStateWithAlloc(tto.alloc)
				for i := 0; i < numParticipants; i++ {
					err := app.ValidTransition(nil, from, to, channel.Index(i))
					if i == tto.valid {
						assert.NoError(err)
					} else {
						assert.Error(err)
					}
				}
			}
		})
	}

	t.Run("panic", func(t *testing.T) {
		from := newStateWithAlloc(tests[0].from)
		to := from.Clone()
		to.Data = nil
		assert.Panics(t, func() { app.ValidTransition(nil, from, to, 0) })
	})

	// Note: we don't need to test other invalid input as the framework guarantees
	// to pass valid input.
}

func newStateWithAlloc(balsv [][]int64) *channel.State {
	bigBalsv := make([][]channel.Bal, len(balsv))
	for i, asset := range balsv {
		bigBalsv[i] = make([]channel.Bal, len(asset))
		for j, bal := range asset {
			bigBalsv[i][j] = big.NewInt(bal)
		}
	}

	return &channel.State{
		Allocation: channel.Allocation{Balances: bigBalsv},
		Data:       new(NoData),
	}
}
