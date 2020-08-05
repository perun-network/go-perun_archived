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

	"github.com/pkg/errors"
	"perun.network/go-perun/channel"
	"perun.network/go-perun/wallet"
)

// Funder implements the channel.Funder interface for the simledger.
type Funder struct {
	Address wallet.Address
	backend *Ledger
}

// compile time check that we implement the perun funder interface.
var _ channel.Funder = (*Funder)(nil)

// NewSimFunder creates a new funder for the simulated ledger.
func NewSimFunder(backend *Ledger, address wallet.Address) *Funder {
	return &Funder{
		Address: address,
		backend: backend,
	}
}

// Fund implements the channel.Funder interface. It funds all assets in
// parallel. If not all participants successfully fund within a timeframe of
// ChallengeDuration seconds, Fund returns a FundingTimeoutError.
func (f *Funder) Fund(ctx context.Context, request channel.FundingReq) error {
	if isCancelled(ctx) {
		return errors.New("called with cancelled context")
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	if err := verifyValidRequest(request); err != nil {
		panic(errors.WithMessage(err, "depositing funds with invalid channel.FundingReq"))
	}

	if err := f.backend.Deposit(ctx, request, f.Address); err != nil {
		return channel.NewFundingTimeoutError(f.collectFundingErrors(request))
	}
	return nil
}

// verifyValidRequest checks if a `channel.FundingReq` is in proper form.
func verifyValidRequest(request channel.FundingReq) error {
	if request.Params == nil || request.State == nil {
		return errors.New("invalid FundingRequest")
	}
	return nil
}

// isCancelled is a non-blocking check if `ctx` was already cancelled.
func isCancelled(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

// collectFundingErrors gathers all `channel.AssetFundingError`s for all peers.
func (f *Funder) collectFundingErrors(request channel.FundingReq) []*channel.AssetFundingError {
	errors := make([]*channel.AssetFundingError, 0, len(request.Params.Parts))
	chanBals := f.backend.ChannelDepots(request.State.ID)
	for assetIdx := range request.State.Allocation.Assets {
		var peers []channel.Index
		for partIdx, balGot := range request.State.Allocation.Balances[assetIdx] {
			// chanBals == nil means not a single participant was able to fund
			// and thus create a channel entry in the ledger.
			if chanBals == nil || chanBals[assetIdx][partIdx].Cmp(balGot) != 0 {
				peers = append(peers, channel.Index(partIdx))
				continue
			}
		}
		errors = append(errors, &channel.AssetFundingError{
			Asset:         assetIdx,
			TimedOutPeers: peers,
		})
	}
	return errors
}
