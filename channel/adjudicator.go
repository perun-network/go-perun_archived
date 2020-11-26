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

package channel

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"

	"perun.network/go-perun/wallet"
)

type (
	// An Adjudicator represents an adjudicator contract on the blockchain. It
	// has methods for state registration and withdrawal of channel funds.
	// A channel state needs to be registered before the concluded state can be
	// withdrawn after a possible timeout.
	//
	// Furthermore, it has a method for subscribing to RegisteredEvents. Those
	// events might be triggered by a Register call on the adjudicator from any
	// channel participant.
	Adjudicator interface {
		// Register should register the given channel state on-chain. It must be
		// taken into account that a peer might already have registered the same or
		// even an old state for the same channel. If registration was successful,
		// it should return the timeout when withdrawal can be initiated with
		// Withdraw.
		Register(context.Context, AdjudicatorReq) error

		// Withdraw should conclude and withdraw the registered state, so that the
		// final outcome is set on the asset holders and funds are withdrawn
		// (dependent on the architecture of the contracts). It must be taken into
		// account that a peer might already have concluded the same channel.
		// If the channel has registered subchannels, the third argument should
		// hold the registered subchannel states.
		Withdraw(context.Context, AdjudicatorReq, map[ID]*State) error

		Progress(context.Context, ProgressReq) error
		Subscribe(context.Context, ID) (AdjudicatorSubscription, error)

		DisputeState(context.Context, ID) (DisputeState, error)
	}

	DisputeState struct {
		Timeout           Timeout
		ChallengeDuration time.Duration
		Version           uint64
		HasApp            bool
		Phase             Phase
		StateHash         common.Hash
	}

	// ProgressReq constitues the request parameters for the Adjudicator's Progress function.
	ProgressReq struct {
		AdjudicatorReq
		NewState *State
		Sig      wallet.Sig
	}

	// Event represents an abstract event emitted by the Adjudicator.
	Event interface {
		ID() ID
		Timeout() Timeout
	}

	// EventBase represents the core information of an Adjudicator event.
	EventBase struct {
		IDV      ID
		TimeoutV Timeout
	}

	// ProgressedEvent is the abstract event that signals an on-chain progression.
	ProgressedEvent struct {
		EventBase        // Channel ID and ForceExec phase timeout
		State     *State // State that was progressed into
		Idx       Index  // Index of the participant who progressed
	}

	// AdjudicatorSubscription represents a subscription to Adjudicator events.
	AdjudicatorSubscription interface {
		// Next returns the newest past or next future event. If the subscription is
		// closed or any other error occurs, it should immediately return nil.
		Next() Event

		// Err returns the error status of the subscription. After Next returns nil,
		// Err should be checked for an error. If the subscription was orderly closed, Err
		// should return nil.
		Err() error

		// Close closes the subscription. Any call to Next should immediately return
		// nil.
		Close() error
	}

	// An AdjudicatorReq collects all necessary information to make calls to the
	// adjudicator.
	//
	// If the Secondary flag is set to true, it is assumed that this is an
	// on-chain request that is executed by the other channel participants as well
	// and the Adjudicator backend may run an optimized on-chain transaction
	// protocol, possibly saving unnecessary double sending of transactions.
	AdjudicatorReq struct {
		Params *Params
		Acc    wallet.Account
		Tx     Transaction
		Idx    Index
	}

	// RegisteredEvent is the abstract event that signals a successful state
	// registration on the blockchain.
	RegisteredEvent struct {
		EventBase
		Version uint64 // Registered version.
	}

	// ConcludedEvent signals channel conclusion.
	ConcludedEvent struct {
		EventBase
	}

	// A Timeout is an abstract timeout of a channel dispute. A timeout can be
	// elapsed and it can be waited on it to elapse.
	Timeout interface {
		// IsElapsed should return whether the timeout has elapsed at the time of
		// the call of this method.
		IsElapsed(context.Context) bool

		// Wait waits for the timeout to elapse. If the context is canceled, Wait
		// should return immediately with the context's error.
		Wait(context.Context) error
	}

	// A RegisteredSubscription is a subscription to RegisteredEvents for a
	// specific channel. The subscription should also return the newest past
	// RegisteredEvent, if there is any.
	//
	// The usage of the subscription should be similar to that of an iterator.
	// Next calls should block until a new event is generated (or the first past
	// event has been found). If the channel is closed or an error is produced,
	// Next should return nil and Err should tell the possible error.
	RegisteredSubscription interface {
		// Next returns the newest past or next future event. If the subscription is
		// closed or any other error occurs, it should return nil.
		Next() *RegisteredEvent

		// Err returns the error status of the subscription. After Next returns nil,
		// Err should be checked for an error.
		Err() error

		// Close closes the subscription. Any call to Next should immediately return
		// nil.
		Close() error
	}
)

// MakeProgressRequest creates a new ProgressReq object.
func MakeProgressRequest(ar AdjudicatorReq, newState *State, sig wallet.Sig) ProgressReq {
	return ProgressReq{ar, newState, sig}
}

// MakeEventBase creates a new EventBase object.
func MakeEventBase(c ID, t Timeout) EventBase {
	return EventBase{c, t}
}

// ID returns the channel identifier corresponding to the event.
func (e EventBase) ID() ID {
	return e.IDV
}

// Timeout returns the timeout associated with the current channel phase.
func (e EventBase) Timeout() Timeout {
	return e.TimeoutV
}

// ElapsedTimeout is a Timeout that is always elapsed.
type ElapsedTimeout struct{}

// IsElapsed returns true.
func (t *ElapsedTimeout) IsElapsed(context.Context) bool { return true }

// Wait immediately return nil.
func (t *ElapsedTimeout) Wait(context.Context) error { return nil }

// String says that this is an always elapsed timeout.
func (t *ElapsedTimeout) String() string { return "<Always elapsed timeout>" }

// TimeTimeout is a Timeout that elapses after a fixed time.Time.
type TimeTimeout struct{ time.Time }

// IsElapsed returns whether the current time is after the fixed timeout.
func (t *TimeTimeout) IsElapsed(context.Context) bool { return t.After(time.Now()) }

// Wait waits until the timeout has elapsed or the context is cancelled.
func (t *TimeTimeout) Wait(ctx context.Context) error {
	select {
	case <-time.After(time.Until(t.Time)):
		return nil
	case <-ctx.Done():
		return errors.Wrap(ctx.Err(), "ctx done")
	}
}

// String returns the timeout's date and time string.
func (t *TimeTimeout) String() string {
	return fmt.Sprintf("<Timeout: %v>", t.Time)
}
