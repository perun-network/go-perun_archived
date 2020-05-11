// Copyright (c) 2020 Chair of Applied Cryptography, Technische Universität
// Darmstadt, Germany. All rights reserved. This file is part of go-perun. Use
// of this source code is governed by the Apache 2.0 license that can be found
// in the LICENSE file.

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
		bals map[Asset]map[wallet.AddrKey]channel.Bal

		// store balances of accounts per channel id per asset address.
		channels map[channel.ID]chanRecord

		// protect the ledger.
		mu sync.Mutex
	}

	// chanRecord holds on-chain channel information.
	chanRecord struct {
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

// NewPrefundedLedger generates a new ledger holding the starting balances described in alloc for the given accounts.
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

// Bals returns all balances of all assets of all the users.
func (l *Ledger) Bals() map[Asset]map[wallet.AddrKey]channel.Bal {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.bals
}

// ChannelBals returns all the balances in the channel with the given ID.
func (l *Ledger) ChannelBals(ID channel.ID) [][]channel.Bal {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.channels[ID].state.Allocation.Balances
}

// ChannelDepots returns all the deposits' balances made in the channel with the given ID.
func (l *Ledger) ChannelDepots(ID channel.ID) [][]channel.Bal {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.channels[ID].deposits.Balances
}

// Add adds amount to account's balance for the given asset.
func (l *Ledger) Add(asset *Asset, account wallet.Address, amount channel.Bal) (err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.add(asset, account, amount)
}

// add adds amount to account's balance for the given asset. This function is not thread-safe.
func (l *Ledger) add(asset *Asset, account wallet.Address, amount channel.Bal) (err error) {
	bal := l.bal(asset, account)
	if bal.CmpAbs(amount) < 0 && amount.Sign() < 0 {
		return errors.New("account balance cannot become negative")
	}
	l.bals[*asset][wallet.Key(account)] = bal.Add(bal, amount)
	return nil
}

// Bal returns the balance of an account for a certain asset.
func (l *Ledger) Bal(asset *Asset, account wallet.Address) channel.Bal {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.bal(asset, account)
}

// bal returns the balance of an account for a certain asset. This function is not thread-safe.
func (l *Ledger) bal(asset *Asset, account wallet.Address) channel.Bal {
	return l.bals[*asset][wallet.Key(account)]
}

// Transfer moves funds on the ledger, errors if not sufficient funds available.
func (l *Ledger) Transfer(asset *Asset, from, to wallet.Address, amount channel.Bal) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if amount.Sign() < 0 {
		return errors.New("cannot transfer negative amount")
	}

	negAmount := new(big.Int).Neg(amount)
	if err := l.add(asset, from, negAmount); err != nil {
		return err
	}

	if err := l.add(asset, to, amount); err != nil {
		return errors.WithMessage(err, "increasing receiver's balance")
	}
	return nil
}

// Deposit moves the entire on-chain balance of the funder into the channel balance of recipientIdx, for each asset.
func (l *Ledger) Deposit(request channel.FundingReq, funder wallet.Address) {
	l.mu.Lock()
	defer l.mu.Unlock()

	chanRec, ok := l.channels[request.Params.ID()]
	if !ok {
		l.channels[request.Params.ID()] = newChanRecord(request.State, request.Params)
		chanRec = l.channels[request.Params.ID()]
	}

	indexes := getAssetIdxs(request.State.Allocation.Assets)
	movements := make(map[int]channel.Bal)
	for asset, holders := range l.bals {
		for addr, bal := range holders {
			if addr.Equals(funder) {
				movements[indexes[asset]] = bal
			}
		}
	}
	for assetIdx, bal := range movements {
		chanRec.deposits.Balances[assetIdx][request.Idx] = new(big.Int).Set(bal)
		bal.Set(big.NewInt(0))
	}
	return
}

// newChanRecord returns a new empty channel record.
func newChanRecord(state *channel.State, params *channel.Params) chanRecord {
	depots := state.Allocation.Clone()
	for i := range depots.Balances {
		for j := range depots.Balances[i] {
			depots.Balances[i][j] = big.NewInt(0)
		}
	}
	return chanRecord{params: params.Clone(), state: state.Clone(), deposits: &depots}
}

// getAssetIdxs returns a map containing, for each asset, its index in the input slice.
func getAssetIdxs(assets []channel.Asset) map[Asset]int {
	indexes := make(map[Asset]int)
	for idx, asset := range assets {
		indexes[*asset.(*Asset)] = idx
	}
	return indexes
}
