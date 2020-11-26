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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"perun.network/go-perun/channel"
)

// Carol is a Responder. She accepts an incoming channel proposal.
type Carol struct {
	Responder
	registered chan *channel.RegisteredEvent
}

// NewCarol creates a new Responder that executes the Carol protocol.
func NewCarol(setup RoleSetup, t *testing.T) *Carol {
	return &Carol{
		Responder:  *NewResponder(setup, t, 3),
		registered: make(chan *channel.RegisteredEvent),
	}
}

// HandleRegistered is the callback for RegisteredEvent.
func (r *Carol) HandleRegistered(e *channel.RegisteredEvent) {
	r.log.Infof("HandleRegistered: %v", e)
	r.registered <- e
}

// HandleProgressed is the callback for ProgressedEvent.
func (r *Carol) HandleProgressed(e *channel.ProgressedEvent) {
	r.log.Infof("HandleProgressed: %v", e)
}

// Execute executes the Carol protocol.
func (r *Carol) Execute(cfg ExecConfig) {
	r.Responder.Execute(cfg, r.exec)
}

func (r *Carol) exec(_cfg ExecConfig, ch *paymentChannel, propHandler *acceptNextPropHandler) {
	cfg := _cfg.(*MalloryCarolExecConfig)
	assert := assert.New(r.t)
	_, them := r.Idxs(cfg.Peers())

	// start watcher
	go func() {
		r.log.Info("Starting channel watcher.")
		assert.NoError(ch.Watch(r))
		r.log.Debug("Channel watcher returned.")
	}()

	// 1st stage - channel controller set up
	r.waitStage()

	// Carol receives some updates from Mallory
	for i := 0; i < cfg.NumPayments[them]; i++ {
		ch.recvTransfer(cfg.TxAmounts[them], fmt.Sprintf("Mallory#%d", i))
	}
	// 2nd stage - txs received
	r.waitStage()

	r.log.Debug("Waiting for registered event")
	e := <-r.registered

	r.log.Debug("Waiting until ready to conclude")
	assert.NoError(e.Timeout().Wait(r.Ctx())) // wait until ready to conclude

	r.log.Debug("Settle")
	ch.settle() // conclude and withdraw

	// 3rd stage - channel settled
	r.waitStage()
}
