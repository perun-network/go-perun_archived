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
	"perun.network/go-perun/log"
)

// ensureConcluded ensures that conclude or concludeFinal (for non-final and
// final states, resp.) is called on the adjudicator.
// - a subscription on Concluded events is established
// - it searches for a past concluded event
//   - if found, channel is already concluded and success is returned
//   - if none found, conclude/concludeFinal is called on the adjudicator
// - it waits for a Concluded event from the blockchain.
func (a *Adjudicator) ensureConcluded(ctx context.Context, req channel.AdjudicatorReq) error {
	// Get dispute state. Return if already concluded.
	if a.testChannelConcluded(ctx, req.Tx.ID) {
		return nil
	}

	// Send conclude transaction.
	var err error
	if req.Tx.IsFinal {
		err = errors.WithMessage(a.callConcludeFinal(ctx, req), "calling concludeFinal")
	} else {
		err = errors.WithMessage(a.callConclude(ctx, req, nil), "calling conclude")
	}

	if IsErrTxFailed(err) {
		a.log.Warnf("Sending conclude transaction failed: %v", err)
		if a.testChannelConcluded(ctx, req.Tx.ID) {
			return nil
		}
	}

	return err
}

func (a *Adjudicator) testChannelConcluded(ctx context.Context, c channel.ID) bool {
	disputeState, err := a.contract.Disputes(NewCallOpts(ctx), c)
	if err != nil {
		log.Warnf("Getting dispute state failed: %v", err)
	}
	return disputeState.Phase == phaseConcluded
}
