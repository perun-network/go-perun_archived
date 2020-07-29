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

	"perun.network/go-perun/channel"
	"perun.network/go-perun/wallet"
)

// Balances holds balances of accounts per asset address.
type Balances map[Asset]map[wallet.AddrKey]channel.Bal

// Clone returns a clone of `b`.
func (b Balances) Clone() Balances {
	bals := make(Balances, len(b))
	for asset, holders := range b {
		bals[asset] = make(map[wallet.AddrKey]channel.Bal, len(holders))
		for holder, bal := range holders {
			bals[asset][holder] = new(big.Int).Set(bal)
		}
	}
	return bals
}
