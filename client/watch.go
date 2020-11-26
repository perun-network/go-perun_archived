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

package client

import (
	"context"

	"github.com/pkg/errors"

	"perun.network/go-perun/channel"
)

// WatchEventHandler represents an interface for handling adjudicator events.
type WatchEventHandler interface {
	HandleRegistered(*channel.RegisteredEvent)
	HandleProgressed(*channel.ProgressedEvent)
}

// Watch watches the adjudicator for channel events and responds accordingly.
func (c *Channel) Watch(h WatchEventHandler) error {
	log := c.Log().WithField("proc", "watcher")
	defer log.Info("Watcher returned.")

	// Subscribe to state changes
	ctx := c.Ctx()
	sub, err := c.adjudicator.Subscribe(ctx, c.Params().ID())
	if err != nil {
		return errors.WithMessage(err, "subscribing to adjudicator state changes")
	}
	// nolint:errcheck
	defer sub.Close()
	// nolint:errcheck,gosec
	c.OnCloseAlways(func() { sub.Close() })

	// Wait for state changed event
	for e := sub.Next(); e != nil; e = sub.Next() {
		log.Infof("event %v", e)

		switch e := e.(type) {
		case *channel.RegisteredEvent:
			// Assert backend version not greater than local version.
			if e.Version > c.State().Version {
				// If the implementation works as intended, this should never happen.
				log.Panicf("watch: registered: expected version less than or equal to %d, got version %d", c.machine.State().Version, e.Version)
			}

			// If local version greater than backend version, register local state.
			if e.Version < c.State().Version {
				if err := c.Register(ctx); err != nil {
					return errors.WithMessage(err, "registering")
				}
			}

			go h.HandleRegistered(e)

		case *channel.ProgressedEvent:
			func() {
				c.machMtx.Lock()
				defer c.machMtx.Unlock()
				c.machine.SetProgressed(e)
			}()
			go h.HandleProgressed(e)

		}
	}

	log.Debugf("Subscription closed: %v", sub.Err())

	if err := sub.Err(); err != nil {
		return errors.WithMessage(err, "subscription closed")
	}

	return nil
}

// Register registers the channel on the adjudicator.
func (c *Channel) Register(ctx context.Context) error {
	// Lock channel machine.
	if !c.machMtx.TryLockCtx(ctx) {
		return errors.WithMessage(ctx.Err(), "locking machine")
	}
	defer c.machMtx.Unlock()

	return c.register(ctx)
}

// ProgressBy progresses the channel state in the adjudicator backend.
func (c *Channel) ProgressBy(ctx context.Context, update func(*channel.State)) error {
	// Lock machine
	if !c.machMtx.TryLockCtx(ctx) {
		return errors.Errorf("locking machine mutex in time: %v", ctx.Err())
	}
	defer c.machMtx.Unlock()

	// Store current state
	ar := c.machine.AdjudicatorReq()

	// Update state
	state := c.machine.State().Clone()
	state.Version++
	update(state)

	// Apply state in machine and generate signature
	c.machine.SetProgressing(state)
	sig, err := c.machine.Sig(ctx)
	if err != nil {
		return errors.WithMessage(err, "signing")
	}

	// Create and send request
	pr := channel.MakeProgressRequest(ar, state, sig)
	if err := c.adjudicator.Progress(ctx, pr); err != nil {
		return errors.WithMessage(err, "progressing")
	}

	return nil
}

// Withdraw concludes a registered channel and withdraws the funds.
func (c *Channel) Withdraw(ctx context.Context) error {
	return c.WithdrawWithSubchannels(ctx, nil)
}

// WithdrawWithSubchannels concludes a registered channel with registered
// subchannels and withdraws the funds.
func (c *Channel) WithdrawWithSubchannels(ctx context.Context, subStates map[channel.ID]*channel.State) error {
	// Lock channel machine.
	if !c.machMtx.TryLockCtx(ctx) {
		return errors.WithMessage(ctx.Err(), "locking machine")
	}
	defer c.machMtx.Unlock()

	if err := c.machine.SetWithdrawing(ctx); err != nil {
		return errors.WithMessage(err, "setting machine to withdrawing phase")
	}

	switch {
	case c.IsLedgerChannel():
		req := c.machine.AdjudicatorReq()
		if err := c.adjudicator.Withdraw(ctx, req, subStates); err != nil {
			return errors.WithMessage(err, "calling Withdraw")
		}
	case c.IsSubChannel():
		if c.hasLockedFunds() {
			return errors.New("cannot settle off-chain with locked funds")
		}
		if err := c.withdrawIntoParent(ctx); err != nil {
			return errors.WithMessage(err, "withdrawing into parent channel")
		}
	default:
		panic("invalid channel type")
	}

	if err := c.machine.SetWithdrawn(ctx); err != nil {
		return errors.WithMessage(err, "setting machine phase")
	}

	return nil
}

// register calls Register on the adjudicator with the current channel state and
// progresses the machine phases. When successful, the resulting RegisteredEvent
// is saved to the phase machine.
//
// The caller is expected to have locked the channel mutex.
func (c *Channel) register(ctx context.Context) error {
	if err := c.machine.SetRegistering(ctx); err != nil {
		return err
	}

	if err := c.adjudicator.Register(ctx, c.machine.AdjudicatorReq()); err != nil {
		return errors.WithMessage(err, "calling Register")
	}

	return c.machine.SetRegistered(ctx)
}
