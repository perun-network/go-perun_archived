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

package hd_test

import (
	"encoding/hex"
	"math/rand"
	"testing"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	hdwalletimpl "github.com/miguelmota/go-ethereum-hdwallet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ethwallet "perun.network/go-perun/backend/ethereum/wallet"
	"perun.network/go-perun/backend/ethereum/wallet/hd"
	pkgtest "perun.network/go-perun/pkg/test"
	"perun.network/go-perun/wallet"
	"perun.network/go-perun/wallet/test"
)

var dataToSign = []byte("SomeLongDataThatShouldBeSignedPlease")
var sampleAddr = "1234560000000000000000000000000000000000"

func TestGenericSignatureTests(t *testing.T) {
	s, _, _ := newSetup(t, pkgtest.Prng(t))
	test.GenericSignatureTest(t, s)
	test.GenericSignatureSizeTest(t, s)
	test.GenericAddressTest(t, s)
}

func TestNewWallet(t *testing.T) {
	prng := pkgtest.Prng(t)

	walletSeed := make([]byte, 20)
	prng.Read(walletSeed)
	mnemonic, err := hdwalletimpl.NewMnemonicFromEntropy(walletSeed)
	require.NoError(t, err)

	rawHDWallet, err := hdwalletimpl.NewFromMnemonic(mnemonic)
	require.NoError(t, err)

	t.Run("happy", func(t *testing.T) {
		hdWallet, err := hd.NewWallet(rawHDWallet, hd.DefaultRootDerivationPath.String(), 0)
		require.NoError(t, err)
		require.NotNil(t, hdWallet)
	})

	t.Run("err_DerivationPath", func(t *testing.T) {
		_, err := hd.NewWallet(rawHDWallet, "invalid-derivation-path", 0)
		require.Error(t, err)
	})

	t.Run("err_nilWallet", func(t *testing.T) {
		_, err := hd.NewWallet(nil, hd.DefaultRootDerivationPath.String(), 0)
		require.Error(t, err)
	})
}

func TestSignWithMissingKey(t *testing.T) {
	setup, accsWallet, _ := newSetup(t, pkgtest.Prng(t))

	missingAddr := common.BytesToAddress(setup.AddressBytes)
	acc := hd.NewAccountFromEth(accsWallet, accounts.Account{Address: missingAddr})
	require.NotNil(t, acc)
	_, err := acc.SignData(setup.DataToSign)
	assert.Error(t, err, "Sign with missing account should fail")
}

func TestUnlock(t *testing.T) {
	setup, _, hdWallet := newSetup(t, pkgtest.Prng(t))

	missingAddr := common.BytesToAddress(setup.AddressBytes)
	_, err := hdWallet.Unlock(ethwallet.AsWalletAddr(missingAddr))
	assert.Error(t, err, "should not error on unlocking missing address")

	validAcc, _ := setup.UnlockedAccount()
	acc, err := hdWallet.Unlock(validAcc.Address())
	assert.NoError(t, err, "should error on unlocking missing address")
	assert.NotNil(t, acc, "account should be non nil when error is nil")
}

func TestContains(t *testing.T) {
	setup, _, hdWallet := newSetup(t, pkgtest.Prng(t))

	assert.False(t, hdWallet.Contains(common.Address{}), "should not contain nil account")

	missingAddr := common.BytesToAddress(setup.AddressBytes)
	assert.False(t, hdWallet.Contains(missingAddr), "should not contain address of the missing account")

	validAcc, err := setup.UnlockedAccount()
	require.NoError(t, err)
	assert.True(t, hdWallet.Contains(ethwallet.AsEthAddr(validAcc.Address())), "should contain valid account")
}

// nolint:interfacer // rand.Rand is preferred over io.Reader here.
func newSetup(t require.TestingT, prng *rand.Rand) (*test.Setup, accounts.Wallet, *hd.Wallet) {
	walletSeed := make([]byte, 20)
	prng.Read(walletSeed)
	mnemonic, err := hdwalletimpl.NewMnemonicFromEntropy(walletSeed)
	require.NoError(t, err)

	rawHDWallet, err := hdwalletimpl.NewFromMnemonic(mnemonic)
	require.NoError(t, err)
	require.NotNil(t, rawHDWallet, "hdwallet must not be nil")

	hdWallet, err := hd.NewWallet(rawHDWallet, hd.DefaultRootDerivationPath.String(), 0)
	require.NoError(t, err)
	require.NotNil(t, hdWallet)

	acc, err := hdWallet.NewAccount()
	require.NoError(t, err)
	require.NotNil(t, acc)

	sampleBytes, err := hex.DecodeString(sampleAddr)
	require.NoError(t, err, "invalid sample address")

	return &test.Setup{
		UnlockedAccount: func() (wallet.Account, error) { return acc, nil },
		Backend:         new(ethwallet.Backend),
		AddressBytes:    sampleBytes,
		DataToSign:      dataToSign,
	}, rawHDWallet, hdWallet
}
