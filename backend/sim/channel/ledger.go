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
	"math/big"
	"sync"

	"github.com/pkg/errors"

	"perun.network/go-perun/channel"
	"perun.network/go-perun/wallet"
)

type (
	// Ledger represents a Ledger.
	Ledger struct {
		// store balances of account on global ledger per asset address.
		bals Balances

		// store balances of accounts per channel id per asset address.
		channels map[channel.ID]chanRecord

		// protect the ledger.
		mu sync.RWMutex
	}

	// chanRecord holds on-chain channel information.
	chanRecord struct {
		//nolint:structcheck,unused
		// params holds the Params of a channel.
		params *channel.Params

		// state holds the State of a channel.
		state *channel.State

		// deposits register how much, in a channel, each participant holds for each asset.
		deposits *channel.Allocation
	}
)

// NewLedger generates a new ledger.
func NewLedger() *Ledger {
	bals := make(map[Asset]map[wallet.AddrKey]channel.Bal)
	channels := make(map[channel.ID]chanRecord)
	return &Ledger{bals: bals, channels: channels}
}

// NewPrefundedLedger generates a new ledger holding the starting balances
// described in alloc for the given accounts.
func NewPrefundedLedger(accounts []wallet.Address, alloc *channel.Allocation) (*Ledger, error) {
	ledger := NewLedger()

	indexes := getAssetIdxs(alloc.Assets)
	for asset, ai := range indexes {
		ledger.bals[asset] = make(map[wallet.AddrKey]channel.Bal)
		for hi, holder := range accounts {
			amount := alloc.Balances[ai][hi]
			if amount.Sign() < 0 {
				return nil, errors.New("account balance cannot be negative")
			}
			ledger.bals[asset][wallet.Key(holder)] = new(big.Int).Set(amount)
		}
	}
	return ledger, nil
}

// Bals returns balances for all assets. Balances have to be cloned with
// `Balances.Clone()` before modifying.
func (l *Ledger) Bals() Balances {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.bals
}

// ChannelBals returns a clone of all the balances in the channel with the given
// ID if existent, else returns `nil`.
func (l *Ledger) ChannelBals(id channel.ID) channel.Balances {
	l.mu.RLock()
	defer l.mu.RUnlock()
	chanRec, ok := l.channels[id]
	if !ok {
		return nil
	}
	return chanRec.state.Allocation.Balances.Clone()
}

// ChannelDepots returns a clone of all the deposits' balances made in the
// channel with the given ID if existent, otherwise returns `nil`.
func (l *Ledger) ChannelDepots(id channel.ID) channel.Balances {
	l.mu.RLock()
	defer l.mu.RUnlock()
	channel, ok := l.channels[id]
	if !ok {
		return nil
	}
	return channel.deposits.Balances.Clone()
}

// Add adds amount to account's balance for the given asset.
func (l *Ledger) Add(asset *Asset, account wallet.Address, amount channel.Bal) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.add(asset, account, amount)
}

// add adds amount to account's balance for the given asset. This function is not thread-safe.
func (l *Ledger) add(asset *Asset, account wallet.Address, amount channel.Bal) error {
	bal, err := l.bal(asset, account)
	if err != nil {
		return errors.WithMessage(err, "ledger transfer")
	}

	if err := validateAdd(bal, amount); err != nil {
		return errors.WithMessage(err, "ledger transfer")
	}
	l.unsafeAdd(asset, account, bal, amount)

	return nil
}

// unsafeAdd adds `amount` to `asset` for `account` without performing any checks.
func (l *Ledger) unsafeAdd(asset *Asset, account wallet.Address, bal, amount channel.Bal) {
	l.bals[*asset][wallet.Key(account)].Add(bal, amount)
}

// unsafeSub subs `amount` from `asset` for `account` without performing any checks.
func (l *Ledger) unsafeSub(asset *Asset, account wallet.Address, bal, amount channel.Bal) {
	l.bals[*asset][wallet.Key(account)].Sub(bal, amount)
}

func validateTransfer(from, to, amount channel.Bal) error {
	if err := validateSub(from, amount); err != nil {
		return errors.WithMessage(err, "transferring")
	}
	return errors.WithMessage(validateAdd(to, amount), "transferring")
}

func validateAdd(bal, amount channel.Bal) error {
	if amount.Sign() < 0 && bal.CmpAbs(amount) < 0 {
		return errors.New("add: account balance becoming negative")
	}
	return nil
}

func validateSub(bal, amount channel.Bal) error {
	if amount.Sign() > 0 && bal.CmpAbs(amount) < 0 {
		return errors.New("sub: account balance becoming negative")
	}
	return nil
}

// Sub subtracts amount from account's balance for the given asset.
func (l *Ledger) Sub(asset *Asset, account wallet.Address, amount channel.Bal) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.add(asset, account, new(big.Int).Neg(amount))
}

// Bal returns a clone of the balance of an account for a certain asset.
func (l *Ledger) Bal(asset Asset, account wallet.Address) (channel.Bal, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.bal(&asset, account)
}

func (l *Ledger) bal(asset *Asset, account wallet.Address) (channel.Bal, error) {
	bals, ok := l.bals[*asset]
	if !ok {
		return nil, errors.Errorf("non-existing asset: %v", asset)
	}
	bal, ok := bals[wallet.Key(account)]
	if !ok {
		return nil, errors.Errorf("non-existing address: %v", account)
	}

	return new(big.Int).Set(bal), nil
}

// Transfer moves funds on the ledger from account `from`, to account `to`,
// errors if not sufficient funds available.
func (l *Ledger) Transfer(asset *Asset, from, to wallet.Address, amount channel.Bal) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if amount.Sign() < 0 {
		return errors.New("amount is negative")
	}

	fromBal, err := l.bal(asset, from)
	if err != nil {
		return errors.WithMessage(err, "transferring balances")
	}
	toBal, err := l.bal(asset, to)
	if err != nil {
		return errors.WithMessage(err, "transferring balances")
	}

	if err := validateTransfer(fromBal, toBal, amount); err != nil {
		return errors.WithMessagef(err, "transferring from: %v -> to: %v\nHave: %v < Want: %v",
			from, to, fromBal, amount)
	}

	l.unsafeSub(asset, from, fromBal, amount)
	l.unsafeAdd(asset, to, toBal, amount)

	return nil
}

// getAssetIdxs returns a map containing, for each asset, its index in the input slice.
func getAssetIdxs(assets []channel.Asset) map[Asset]int {
	indexes := make(map[Asset]int, len(assets))
	for idx, asset := range assets {
		indexes[*asset.(*Asset)] = idx
	}
	return indexes
}
