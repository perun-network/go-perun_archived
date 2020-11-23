// Copyright 2020 - See NOTICE file for copyright holders.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package test

import (
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"perun.network/go-perun/channel"
	"perun.network/go-perun/client"
)

// MalloryCarolExecConfig contains config parameters for Mallory and Carol test.
type MalloryCarolExecConfig struct {
	BaseExecConfig
	NumPayments [2]int      // how many payments each role sends
	TxAmounts   [2]*big.Int // amounts that are to be sent/requested by each role
}

// Mallory is a test client role. She proposes the new channel.
type Mallory struct {
	Proposer
}

// NewMallory creates a new party that executes the Mallory protocol.
func NewMallory(setup RoleSetup, t *testing.T) *Mallory {
	return &Mallory{Proposer: *NewProposer(setup, t, 3)}
}

// HandleRegistered is the callback for RegisteredEvent.
func (r *Mallory) HandleRegistered(e *channel.RegisteredEvent) {
	r.log.Infof("HandleRegistered: %v", e)
}

// HandleProgressed is the callback for ProgressedEvent.
func (r *Mallory) HandleProgressed(e *channel.ProgressedEvent) {
	r.log.Infof("HandleProgressed: %v", e)
}

// Execute executes the Mallory protocol.
func (r *Mallory) Execute(cfg ExecConfig) {
	r.Proposer.Execute(cfg, r.exec)
}

func (r *Mallory) exec(_cfg ExecConfig, ch *paymentChannel) {
	cfg := _cfg.(*MalloryCarolExecConfig)
	assert := assert.New(r.t)
	we, _ := r.Idxs(cfg.Peers())
	// AdjudicatorReq for version 0
	req0 := client.NewTestChannel(ch.Channel).AdjudicatorReq()

	// start watcher
	go func() {
		r.log.Info("Starting channel watcher.")
		ch.Watch(r)
		r.log.Debug("Channel watcher returned.")
	}()

	// 1st stage - channel controller set up
	r.waitStage()

	// Mallory sends some updates to Carol
	for i := 0; i < cfg.NumPayments[we]; i++ {
		ch.sendTransfer(cfg.TxAmounts[we], fmt.Sprintf("Mallory#%d", i))
	}
	// 2nd stage - txs sent
	r.waitStage()

	// Register version 0 AdjudicatorReq
	challengeDuration := time.Duration(ch.Channel.Params().ChallengeDuration) * time.Second
	regCtx, regCancel := context.WithTimeout(context.Background(), r.timeout)
	defer regCancel()
	r.log.Debug("Registering version 0 state.")
	reg0, err := r.setup.Adjudicator.Register(regCtx, req0)
	assert.NoError(err)
	assert.NotNil(reg0)
	r.log.Debugln("<Registered> ver 0: ", reg0)

	// within the challenge duration, Carol should refute.
	subCtx, subCancel := context.WithTimeout(context.Background(), r.timeout+challengeDuration)
	defer subCancel()
	sub, err := r.setup.Adjudicator.Subscribe(subCtx, ch.Params())
	assert.NoError(err)

	// 3rd stage - wait until Carol has refuted
	r.waitStage()

	assert.True(reg0.Timeout().IsElapsed(subCtx),
		"Carol's refutation should already have progressed past the timeout.")
	reg := sub.Next().(*channel.RegisteredEvent) // should be event caused by Mallory's refutation.
	reg = sub.Next().(*channel.RegisteredEvent)  // should be event caused by Carol's refutation.
	assert.NoError(sub.Close())
	assert.NoError(sub.Err())
	assert.NotNil(reg)
	r.log.Debugln("<Registered> refuted: ", reg)
	if reg != nil {
		assert.Equal(ch.State().Version, reg.Version, "expected refutation with current version")
		waitCtx, waitCancel := context.WithTimeout(context.Background(), r.timeout+challengeDuration)
		defer waitCancel()
		// refutation increased the timeout.
		assert.NoError(reg.Timeout().Wait(waitCtx))
	}

	wdCtx, wdCancel := context.WithTimeout(context.Background(), r.timeout)
	defer wdCancel()
	err = r.setup.Adjudicator.Withdraw(wdCtx, req0, nil)
	assert.Error(err, "withdrawing should fail because Carol should have refuted.")

	// settling current version should work
	ch.settle()
}
