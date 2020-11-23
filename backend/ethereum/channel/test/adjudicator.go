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
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"

	ethchannel "perun.network/go-perun/backend/ethereum/channel"
	"perun.network/go-perun/channel"
)

// A SimAdjudicator is an Adjudicator for simulated backends. Its Register
// method and subscription return a *channel.RegisteredEvent whose Timeout is a
// SimTimeout. SimTimeouts advance the clock of the simulated backend when Wait
// is called.
type SimAdjudicator struct {
	ethchannel.Adjudicator
	sb *SimulatedBackend
}

// NewSimAdjudicator returns a new SimAdjudicator for the given backend. The
// backend must be a SimulatedBackend or it panics.
func NewSimAdjudicator(backend ethchannel.ContractBackend, contract common.Address, receiver common.Address, acc accounts.Account) *SimAdjudicator {
	sb, ok := backend.ContractInterface.(*SimulatedBackend)
	if !ok {
		panic("SimAdjudicator can only be created with a SimulatedBackend.")
	}
	return &SimAdjudicator{
		Adjudicator: *ethchannel.NewAdjudicator(backend, contract, receiver, acc),
		sb:          sb,
	}
}

// Register calls Register on the Adjudicator, returning a
// *channel.RegisteredEvent with a SimTimeout or ElapsedTimeout.
func (a *SimAdjudicator) Register(ctx context.Context, req channel.AdjudicatorReq) (*channel.RegisteredEvent, error) {
	reg, err := a.Adjudicator.Register(ctx, req)
	if err != nil {
		return reg, err
	}

	switch t := reg.Timeout().(type) {
	case *ethchannel.BlockTimeout:
		reg.EventBase.TimeoutV = block2SimTimeout(a.sb, t)
	case *channel.ElapsedTimeout: // leave as is
	case nil: // leave as is
	default:
		panic("invalid timeout type from embedded Adjudicator")
	}
	return reg, nil
}

// SubscribeRegistered returns a RegisteredEvent subscription on the simulated
// blockchain backend.
func (a *SimAdjudicator) SubscribeRegistered(ctx context.Context, params *channel.Params) (channel.RegisteredSubscription, error) {
	sub, err := a.Adjudicator.SubscribeRegistered(ctx, params)
	if err != nil {
		return nil, err
	}
	return &SimRegisteredSub{
		RegisteredSub: *(sub.(*ethchannel.RegisteredSub)),
		sb:            a.sb,
	}, nil
}

// A SimRegisteredSub embeds an ethereum/channel.RegisteredSub, converting
// normal TimeTimeouts to SimTimeouts.
type SimRegisteredSub struct {
	ethchannel.RegisteredSub
	sb *SimulatedBackend
}

// Next calls Next on the underlying subscription, converting the TimeTimeout to
// a SimTimeout.
func (r *SimRegisteredSub) Next() *channel.RegisteredEvent {
	reg := r.RegisteredSub.Next()
	if reg == nil {
		return nil
	}
	reg.Timeout = block2SimTimeout(r.sb, reg.Timeout.(*ethchannel.BlockTimeout))
	return reg
}

func block2SimTimeout(sb *SimulatedBackend, t *ethchannel.BlockTimeout) *SimTimeout {
	return &SimTimeout{t.Time, sb}
}

// A SimTimeout is a timeout on a simulated blockchain. The first call to Wait
// advances the clock of the simulated blockchain past the timeout. Access to
// the blockchain by different SimTimeouts is guarded by a shared mutex.
type SimTimeout struct {
	Time uint64
	sb   *SimulatedBackend
}

// IsElapsed returns whether the timeout is higher than the current block's
// timestamp.
// Access to the blockchain by different SimTimeouts is guarded by a shared mutex.
func (t *SimTimeout) IsElapsed(ctx context.Context) bool {
	if !t.sb.clockMu.TryLockCtx(ctx) {
		return false // subsequent Wait call will expose error to caller
	}
	defer t.sb.clockMu.Unlock()

	return t.timeLeft() <= 0
}

// Wait advances the clock of the simulated blockchain past the timeout.
// Access to the blockchain by different SimTimeouts is guarded by a shared mutex.
func (t *SimTimeout) Wait(ctx context.Context) error {
	if !t.sb.clockMu.TryLockCtx(ctx) {
		return errors.New("clock mutex could not be locked")
	}
	defer t.sb.clockMu.Unlock()

	if d := t.timeLeft(); d > 0 {
		if err := t.sb.AdjustTime(time.Duration(d) * time.Second); err != nil {
			return errors.Wrap(err, "adjusting time")
		}
		t.sb.Commit()
	}
	return nil
}

func (t *SimTimeout) timeLeft() int64 {
	// context is ignored by sim blockchain anyways
	h, err := t.sb.HeaderByNumber(nil, nil)
	if err != nil { // should never happen with a sim blockchain
		panic(fmt.Sprint("Error getting latest block: ", err))
	}
	return int64(t.Time) - int64(h.Time)
}

// String returns the timeout in absolute seconds as a string.
func (t *SimTimeout) String() string {
	return fmt.Sprintf("<Sim timeout: %v>", t.Time)
}
