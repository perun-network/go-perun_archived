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

		// funded acts as an event for when a channel is successfully funded
		funded chan struct{}
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

// cloneBals clones the map of assets to holders from `l`.
func (l *Ledger) cloneBals() map[Asset]map[wallet.AddrKey]channel.Bal {
	bals := make(map[Asset]map[wallet.AddrKey]channel.Bal)
	for asset, holders := range l.bals {
		bals[asset] = make(map[wallet.AddrKey]channel.Bal)
		for holder, bal := range holders {
			bals[asset][holder] = new(big.Int).Set(bal)
		}
	}
	return bals
}

// Bals returns a copy of all balances of all assets of all the users.
func (l *Ledger) Bals() map[Asset]map[wallet.AddrKey]channel.Bal {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.cloneBals()
}

// ChannelBals returns a copy of all the balances in the channel with the given
// ID if existent, else returns `nil`.
func (l *Ledger) ChannelBals(id channel.ID) [][]channel.Bal {
	l.mu.Lock()
	defer l.mu.Unlock()
	chanRec, ok := l.channels[id]
	if !ok {
		return nil
	}
	return cloneAssetsBals(chanRec.state.Allocation.Balances)
}

// cloneAssetsBals clones given `original` and returns the clone.
func cloneAssetsBals(original [][]channel.Bal) [][]channel.Bal {
	cBals := make([][]channel.Bal, len(original))
	for i := range original {
		cBals[i] = channel.CloneBals(original[i])
	}
	return cBals
}

// ChannelDepots returns all the deposits' balances made in the channel with the
// given ID if existent, otherwise returns `nil`.
func (l *Ledger) ChannelDepots(id channel.ID) [][]channel.Bal {
	l.mu.Lock()
	defer l.mu.Unlock()
	channel, ok := l.channels[id]
	if !ok {
		return nil
	}
	return cloneAssetsBals(channel.deposits.Balances)
}

// Add adds amount to account's balance for the given asset.
func (l *Ledger) Add(asset *Asset, account wallet.Address, amount channel.Bal) (err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.add(asset, account, amount)
}

// add adds amount to account's balance for the given asset. This function is not thread-safe.
func (l *Ledger) add(asset *Asset, account wallet.Address, amount channel.Bal) error {
	bal := l.bal(*asset, account)
	if err := l.verifyTransfer(bal, amount); err != nil {
		return errors.WithMessage(err, "ledger transfer")
	}
	l.bals[*asset][wallet.Key(account)].Add(bal, amount)
	return nil
}

func (l *Ledger) verifyTransfer(bal, amount channel.Bal) (err error) {
	if bal.CmpAbs(amount) < 0 && amount.Sign() < 0 {
		err = errors.New("account balance cannot become negative")
	}
	return err
}

// Sub subtracts amount from account's balance for the given asset.
func (l *Ledger) Sub(asset *Asset, account wallet.Address, amount channel.Bal) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.sub(asset, account, amount)
}

// sub subtracts the amount from account's balance for the given asset. This function is not thread-safe.
func (l *Ledger) sub(asset *Asset, account wallet.Address, amount channel.Bal) error {
	return l.add(asset, account, new(big.Int).Neg(amount))
}

// Bal returns a copy of the balance of an account for a certain asset.
func (l *Ledger) Bal(asset Asset, account wallet.Address) channel.Bal {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.bal(asset, account)
}

// bal returns a copy of the balance of an account for a certain asset. This function is not thread-safe.
func (l *Ledger) bal(asset Asset, account wallet.Address) channel.Bal {
	return new(big.Int).Set(l.bals[asset][wallet.Key(account)])
}

// Transfer moves funds on the ledger, errors if not sufficient funds available.
func (l *Ledger) Transfer(asset *Asset, from, to wallet.Address, amount channel.Bal) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if amount.Sign() < 0 {
		return errors.New("cannot transfer negative amount")
	}

	if err := l.sub(asset, from, amount); err != nil {
		return err
	}

	if err := l.add(asset, to, amount); err != nil {
		return errors.WithMessage(err, "increasing receiver's balance")
	}
	return nil
}

// Deposit moves all on-chain balances of the funder into the channel balance of
// `request.Idx`, for each asset. The given `channel.FundingReq` is assumed not
// to be malformed.
func (l *Ledger) Deposit(ctx context.Context, request channel.FundingReq, funder wallet.Address) error {
	l.mu.Lock()

	alloc := request.State.Allocation
	if err := l.verifyChannelDeposit(wallet.Key(funder), request.Idx, alloc); err != nil {
		l.mu.Unlock()
		return errors.WithMessage(err, "depositing funds for channel from ledger")
	}

	chanRec, ok := l.channels[request.Params.ID()]
	if !ok {
		l.channels[request.Params.ID()] = newChanRecord(request.State, request.Params)
		chanRec = l.channels[request.Params.ID()]
	}

	for assetIdx, asset := range alloc.Assets {
		reqBal := alloc.Balances[assetIdx][request.Idx]
		chanRec.deposits.Balances[assetIdx][request.Idx].Set(reqBal)
		if err := l.sub(asset.(*Asset), funder, reqBal); err != nil {
			l.mu.Unlock()
			return errors.WithMessage(err, "subtracting funds for account from ledger")
		}
	}

	chanRec.emitEventIfFunded()
	l.mu.Unlock()

	select {
	case <-ctx.Done():
		return errors.Wrap(ctx.Err(), "context timeout")
	case <-chanRec.funded:
		return nil
	}
}

// verifyChannelDeposit verifies if a deposit of all assets within `alloc.Assets`
// for a channel is possible. This method is not threadsafe and requires the
// caller to have acquired a lock beforehand.
func (l *Ledger) verifyChannelDeposit(funder wallet.AddrKey, funderIdx channel.Index, alloc channel.Allocation) error {
	for assetIdx, asset := range alloc.Assets {
		reqBal := alloc.Balances[assetIdx][funderIdx]
		curBal := l.bals[*asset.(*Asset)][funder]
		if err := l.verifyTransfer(curBal, new(big.Int).Neg(reqBal)); err != nil {
			return errors.WithMessage(err, "depositing funds for channel from ledger")
		}
	}
	return nil
}

// emitEventIfFunded closes the `cr.funded` channel iff all assets have been
// successfully funded, thus emitting a successful `funded` event.
func (cr *chanRecord) emitEventIfFunded() {
	depositedSum := cr.deposits.Sum()
	requestedSum := cr.state.Allocation.Sum()
	select {
	case <-cr.funded:
		return
	default:
	}

	for i, depSum := range depositedSum {
		if depSum.Cmp(requestedSum[i]) < 0 {
			return
		}
	}
	close(cr.funded)
}

// newChanRecord returns a new empty channel record.
func newChanRecord(state *channel.State, params *channel.Params) chanRecord {
	depots := state.Allocation.Clone()
	for i := range depots.Balances {
		for j := range depots.Balances[i] {
			depots.Balances[i][j] = big.NewInt(0)
		}
	}
	return chanRecord{
		params:   params.Clone(),
		state:    state.Clone(),
		deposits: &depots,
		funded:   make(chan struct{}),
	}
}

// getAssetIdxs returns a map containing, for each asset, its index in the input slice.
func getAssetIdxs(assets []channel.Asset) map[Asset]int {
	indexes := make(map[Asset]int)
	for idx, asset := range assets {
		indexes[*asset.(*Asset)] = idx
	}
	return indexes
}
