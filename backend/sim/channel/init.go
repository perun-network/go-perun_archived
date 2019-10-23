// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package channel // import "perun.network/go-perun/backend/sim/channel"

import (
	"io"

	"perun.network/go-perun/backend/sim/wallet"
	"perun.network/go-perun/channel"
	perun "perun.network/go-perun/wallet"
)

func init() {
	channel.SetBackend(new(backend))
	channel.SetAppBackend(new(appBackend))
}

type appBackend struct{}

var _ channel.AppBackend = &appBackend{}

func (appBackend) AppFromDefinition(pAddr perun.Address) (channel.App, error) {
	simAddr := pAddr.(*wallet.Address)
	return NewStateApp1(*simAddr), nil
}

func (appBackend) DecodeAsset(r io.Reader) (channel.Asset, error) {
	var asset wallet.Address
	return &asset, asset.Decode(r)
}
