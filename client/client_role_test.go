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

package client_test

import (
	"context"
	"math/rand"
	"time"

	"perun.network/go-perun/channel"
	ctest "perun.network/go-perun/client/test"
	"perun.network/go-perun/log"
	wtest "perun.network/go-perun/wallet/test"
	"perun.network/go-perun/wire"
)

const roleOperationTimeout = 1 * time.Second

func NewSetups(rng *rand.Rand, names []string) []ctest.RoleSetup {
	var (
		bus   = wire.NewLocalBus()
		n     = len(names)
		setup = make([]ctest.RoleSetup, n)
	)

	for i := 0; i < n; i++ {
		acc := wtest.NewRandomAccount(rng)
		setup[i] = ctest.RoleSetup{
			Name:        names[i],
			Identity:    acc,
			Bus:         bus,
			Funder:      &logFunder{log.WithField("role", names[i])},
			Adjudicator: &logAdjudicator{log.WithField("role", names[i])},
			Wallet:      wtest.NewWallet(),
			Timeout:     roleOperationTimeout,
		}
	}

	return setup
}

type (
	logFunder struct {
		log log.Logger
	}

	logAdjudicator struct {
		log log.Logger
	}
)

func (f *logFunder) Fund(_ context.Context, req channel.FundingReq) error {
	f.log.Infof("Funding: %v", req)
	return nil
}

func (a *logAdjudicator) Register(ctx context.Context, req channel.AdjudicatorReq) error {
	a.log.Infof("Register: %v", req)
	return nil
}

func (a *logAdjudicator) Progress(ctx context.Context, req channel.ProgressReq) error {
	a.log.Infof("Progress: %v", req)
	return nil
}

func (a *logAdjudicator) Withdraw(ctx context.Context, req channel.AdjudicatorReq, subStates map[channel.ID]*channel.State) error {
	a.log.Infof("Withdraw: %v", req)
	return nil
}

func (a *logAdjudicator) Subscribe(ctx context.Context, c channel.ID) (channel.AdjudicatorSubscription, error) {
	a.log.Infof("Subscribe: %v", c)
	return nil, nil
}

func (a *logAdjudicator) DisputeState(ctx context.Context, c channel.ID) (channel.DisputeState, error) {
	a.log.Infof("DisputeState: %v", c)
	return channel.DisputeState{}, nil
}
