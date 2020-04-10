// Copyright (c) 2020 Chair of Applied Cryptography, Technische Universität
// Darmstadt, Germany. All rights reserved. This file is part of go-perun. Use
// of this source code is governed by a MIT-style license that can be found in
// the LICENSE file.

package persistence

import (
	"context"

	"github.com/pkg/errors"

	"perun.network/go-perun/channel"
	"perun.network/go-perun/wallet"
)

// A StateMachine is a wrapper around a channel.StateMachine that forwards calls
// to it and, if successful, persists changed data using a Persister.
type StateMachine struct {
	*channel.StateMachine
	pr Persister
}

// FromStateMachine creates a persisting StateMachine wrapper around the passed
// StateMachine using the Persister pr.
func FromStateMachine(m *channel.StateMachine, pr Persister) StateMachine {
	return StateMachine{
		StateMachine: m,
		pr:           pr,
	}
}

// SetFunded calls SetFunded on the channel.StateMachine and then persists the
// changed phase.
func (m StateMachine) SetFunded(ctx context.Context) error {
	if err := m.StateMachine.SetFunded(); err != nil {
		return err
	}
	return errors.WithMessage(m.pr.PhaseChanged(ctx, m.StateMachine), "Persister.PhaseChanged")
}

// SetRegistering calls SetRegistering on the channel.StateMachine and then
// persists the changed phase.
func (m StateMachine) SetRegistering(ctx context.Context) error {
	if err := m.StateMachine.SetRegistering(); err != nil {
		return err
	}
	return errors.WithMessage(m.pr.PhaseChanged(ctx, m.StateMachine), "Persister.PhaseChanged")
}

// SetRegistered calls SetRegistered on the channel.StateMachine and then
// persists the changed phase.
func (m StateMachine) SetRegistered(ctx context.Context, reg *channel.RegisteredEvent) error {
	if err := m.StateMachine.SetRegistered(reg); err != nil {
		return err
	}
	return errors.WithMessage(m.pr.PhaseChanged(ctx, m.StateMachine), "Persister.PhaseChanged")
}

// SetWithdrawing calls SetWithdrawing on the channel.StateMachine and then
// persists the changed phase.
func (m StateMachine) SetWithdrawing(ctx context.Context) error {
	if err := m.StateMachine.SetWithdrawing(); err != nil {
		return err
	}
	return errors.WithMessage(m.pr.PhaseChanged(ctx, m.StateMachine), "Persister.PhaseChanged")
}

// SetWithdrawn calls SetWithdrawn on the channel.StateMachine and then persists
// the changed phase.
func (m StateMachine) SetWithdrawn(ctx context.Context) error {
	if err := m.StateMachine.SetWithdrawn(); err != nil {
		return err
	}
	return errors.WithMessage(m.pr.PhaseChanged(ctx, m.StateMachine), "Persister.PhaseChanged")
}

// Init calls Init on the channel.StateMachine and then persists the changed
// staging state.
func (m *StateMachine) Init(ctx context.Context, initBals channel.Allocation, initData channel.Data) error {
	if err := m.StateMachine.Init(initBals, initData); err != nil {
		return err
	}
	return errors.WithMessage(m.pr.Staged(ctx, m.StateMachine), "Persister.Staged")
}

// Update calls Update on the channel.StateMachine and then persists the changed
// staging state.
func (m StateMachine) Update(
	ctx context.Context,
	stagingState *channel.State,
	actor channel.Index,
) error {
	if err := m.StateMachine.Update(stagingState, actor); err != nil {
		return err
	}
	return errors.WithMessage(m.pr.Staged(ctx, m.StateMachine), "Persister.Staged")
}

// Sig calls Sig on the channel.StateMachine and then persists the added
// signature.
func (m StateMachine) Sig(ctx context.Context) (sig wallet.Sig, err error) {
	sig, err = m.StateMachine.Sig()
	if err != nil {
		return sig, err
	}
	return sig, errors.WithMessage(m.pr.SigAdded(ctx, m.StateMachine, m.Idx()), "Persister.SigAdded")
}

// AddSig calls AddSig on the channel.StateMachine and then persists the added
// signature.
func (m StateMachine) AddSig(ctx context.Context, idx channel.Index, sig wallet.Sig) error {
	if err := m.StateMachine.AddSig(idx, sig); err != nil {
		return err
	}
	return errors.WithMessage(m.pr.SigAdded(ctx, m.StateMachine, idx), "Persister.SigAdded")
}

// EnableInit calls EnableInit on the channel.StateMachine and then persists the
// enabled transaction.
func (m StateMachine) EnableInit(ctx context.Context) error {
	if err := m.StateMachine.EnableInit(); err != nil {
		return err
	}
	return errors.WithMessage(m.pr.Enabled(ctx, m.StateMachine), "Persister.Enabled")
}

// EnableUpdate calls EnableUpdate on the channel.StateMachine and then persists
// the enabled transaction.
func (m StateMachine) EnableUpdate(ctx context.Context) error {
	if err := m.StateMachine.EnableUpdate(); err != nil {
		return err
	}
	return errors.WithMessage(m.pr.Enabled(ctx, m.StateMachine), "Persister.Enabled")
}

// EnableFinal calls EnableFinal on the channel.StateMachine and then persists
// the enabled transaction.
func (m StateMachine) EnableFinal(ctx context.Context) error {
	if err := m.StateMachine.EnableFinal(); err != nil {
		return err
	}
	return errors.WithMessage(m.pr.Enabled(ctx, m.StateMachine), "Persister.Enabled")
}
