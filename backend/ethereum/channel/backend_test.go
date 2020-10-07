// Copyright 2019 - See NOTICE file for copyright holders.
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

package channel_test

import (
	"context"
	"encoding/hex"
	"io"
	"math/big"
	"math/rand"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ethchannel "perun.network/go-perun/backend/ethereum/channel"
	ethchanneltest "perun.network/go-perun/backend/ethereum/channel/test"
	"perun.network/go-perun/backend/ethereum/wallet"
	ethwallettest "perun.network/go-perun/backend/ethereum/wallet/test"
	"perun.network/go-perun/channel"
	"perun.network/go-perun/channel/test"
	iotest "perun.network/go-perun/pkg/io/test"
	pkgtest "perun.network/go-perun/pkg/test"
	perunwallet "perun.network/go-perun/wallet"
	wallettest "perun.network/go-perun/wallet/test"
)

func TestCalcID(t *testing.T) {
	rng := pkgtest.Prng(t)
	s := ethchanneltest.NewSetup(t, rng, 1)

	for i := 0; i < 250; i++ {
		params := test.NewRandomParams(rng)
		ethParams := ethchannel.ChannelParamsToEthParams(params)
		ethId, err := s.Adjs[0].CalcChannelID(context.TODO(), ethParams)
		chID := channel.CalcID(params)

		require.NoError(t, err)
		require.Equal(t, chID, ethId)
	}
}

func TestHashState(t *testing.T) {
	rng := pkgtest.Prng(t)
	s := ethchanneltest.NewSetup(t, rng, 1)

	for i := 0; i < 250; i++ {
		state := test.NewRandomState(rng)
		ethState := ethchannel.ChannelStateToEthState(state)
		ethId, err := s.Adjs[0].CalcStateHash(context.TODO(), ethState)
		chID := ethchannel.HashState(state)

		require.NoError(t, err)
		require.Equal(t, chID, ethId)
	}
}

func TestGenericTests(t *testing.T) {
	setup := newChannelSetup(pkgtest.Prng(t))
	test.GenericBackendTest(t, setup)
	test.GenericStateEqualTest(t, setup.State, setup.State2)
}

func newChannelSetup(rng *rand.Rand) *test.Setup {
	params, state := test.NewRandomParamsAndState(rng, test.WithNumLocked(int(rng.Int31n(4)+1)))
	params2, state2 := test.NewRandomParamsAndState(rng, test.WithIsFinal(!state.IsFinal), test.WithNumLocked(int(rng.Int31n(4)+1)))

	createAddr := func() perunwallet.Address {
		return wallettest.NewRandomAddress(rng)
	}

	return &test.Setup{
		Params:        params,
		Params2:       params2,
		State:         state,
		State2:        state2,
		Account:       wallettest.NewRandomAccount(rng),
		RandomAddress: createAddr,
	}
}

func newAddressFromString(s string) *wallet.Address {
	addr := wallet.Address(common.HexToAddress(s))
	return &addr
}

func TestChannelID(t *testing.T) {
	tests := []struct {
		name        string
		aliceAddr   string
		bobAddr     string
		appAddr     string
		challengDur uint64
		nonceStr    string
		channelID   string
	}{
		{"Test case 1",
			"0xf17f52151EbEF6C7334FAD080c5704D77216b732",
			"0xC5fdf4076b8F3A5357c5E395ab970B5B54098Fef",
			"0x9FBDa871d559710256a2502A2517b794B482Db40",
			uint64(60),
			"B0B0FACE",
			"f27b90711d11d10a155fc8ba0eed1ffbf449cf3730d88c0cb77b98f61750ab34"},
		{"Test case 2",
			"0x0000000000000000000000000000000000000000",
			"0x0000000000000000000000000000000000000000",
			"0x0000000000000000000000000000000000000000",
			uint64(0),
			"0",
			"c8ac0e8f7eeea864a050a8626dfa0ffb916f43c90bc6b2ba68df6ed063c952e2"},
	}
	for _, _tt := range tests {
		tt := _tt
		t.Run(tt.name, func(t *testing.T) {
			nonce, ok := new(big.Int).SetString(tt.nonceStr, 16)
			assert.True(t, ok, "Setting the nonce should not fail")
			alice := newAddressFromString(tt.aliceAddr)
			bob := newAddressFromString(tt.bobAddr)
			app := newAddressFromString(tt.appAddr)
			params := channel.Params{
				ChallengeDuration: tt.challengDur,
				Nonce:             nonce,
				Parts:             []perunwallet.Address{alice, bob},
				App:               channel.NewMockApp(app),
			}
			cID := channel.CalcID(&params)
			preCalc, err := hex.DecodeString(tt.channelID)
			assert.NoError(t, err, "Decoding the channelID should not error")
			assert.Equal(t, preCalc, cID[:], "ChannelID should match the testcase")
		})
	}

}

func TestAssetSerialization(t *testing.T) {
	rng := pkgtest.Prng(t)
	var asset ethchannel.Asset = ethwallettest.NewRandomAddress(rng)
	reader, writer := io.Pipe()
	done := make(chan struct{})

	go func() {
		defer close(done)
		assert.NoError(t, asset.Encode(writer))
	}()

	asset2, err := ethchannel.DecodeAsset(reader)
	assert.NoError(t, err, "Decode asset should not produce error")
	assert.Equal(t, &asset, asset2, "Decode asset should return the initial asset")
	<-done

	iotest.GenericSerializerTest(t, &asset)
}
