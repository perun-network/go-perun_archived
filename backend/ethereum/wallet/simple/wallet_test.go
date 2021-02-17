// Copyright 2021 - See NOTICE file for copyright holders.
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

package simple_test

import (
	"crypto/ecdsa"
	"encoding/hex"
	"math/rand"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/secp256k1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ethwallet "perun.network/go-perun/backend/ethereum/wallet"
	"perun.network/go-perun/backend/ethereum/wallet/simple"
	pkgtest "perun.network/go-perun/pkg/test"
	"perun.network/go-perun/wallet"
	"perun.network/go-perun/wallet/test"
)

var dataToSign = []byte("SomeLongDataThatShouldBeSignedPlease")

const sampleAddr = "1234560000000000000000000000000000000000"

func TestGenericSignatureTests(t *testing.T) {
	setup, _ := newSetup(t, pkgtest.Prng(t))
	test.GenericSignatureTest(t, setup)
	test.GenericSignatureSizeTest(t, setup)
	test.GenericAddressTest(t, setup)
}

func TestNewWallet(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(secp256k1.S256(), pkgtest.Prng(t))
	require.NoError(t, err)

	simpleWallet := simple.NewWallet(privateKey)
	require.NotNil(t, simpleWallet)
}

func TestUnlock(t *testing.T) {
	setup, simpleWallet := newSetup(t, pkgtest.Prng(t))

	missingAddr := common.BytesToAddress(setup.AddressBytes)
	_, err := simpleWallet.Unlock(ethwallet.AsWalletAddr(missingAddr))
	assert.Error(t, err, "should error on unlocking missing address")

	validAcc, _ := setup.UnlockedAccount()
	acc, err := simpleWallet.Unlock(validAcc.Address())
	assert.NoError(t, err, "should not error on unlocking missing address")
	assert.NotNil(t, acc, "account should be non nil when error is nil")
}

func TestWallet_Contains(t *testing.T) {
	setup, simpleWallet := newSetup(t, pkgtest.Prng(t))

	missingAddr := common.BytesToAddress(setup.AddressBytes)
	assert.False(t, simpleWallet.Contains(missingAddr))

	validAcc, err := setup.UnlockedAccount()
	require.NoError(t, err)
	assert.True(t, simpleWallet.Contains(ethwallet.AsEthAddr(validAcc.Address())))
}

func TestSignatures(t *testing.T) {
	simpleWallet := simple.NewWallet([]*ecdsa.PrivateKey{}...)
	acc := simpleWallet.NewRandomAccount(pkgtest.Prng(t))
	sig, err := acc.SignData(dataToSign)
	assert.NoError(t, err, "Sign with new account should succeed")
	assert.NotNil(t, sig)
	assert.Equal(t, len(sig), ethwallet.SigLen, "Ethereum signature has wrong length")
	valid, err := new(ethwallet.Backend).VerifySignature(dataToSign, sig, acc.Address())
	assert.True(t, valid, "Verification should succeed")
	assert.NoError(t, err, "Verification should succeed")
}

func newSetup(t require.TestingT, prng *rand.Rand) (*test.Setup, *simple.Wallet) {
	numAccounts := prng.Intn(99) + 1
	privateKeys := make([]*ecdsa.PrivateKey, numAccounts)
	for i := 0; i < numAccounts; i++ {
		privateKey, err := ecdsa.GenerateKey(secp256k1.S256(), prng)
		require.NoError(t, err)
		privateKeys[i] = privateKey
	}

	simpleWallet := simple.NewWallet(privateKeys...)
	require.NotNil(t, simpleWallet)

	randomKeyIndex := prng.Intn(numAccounts)
	addr := crypto.PubkeyToAddress(privateKeys[randomKeyIndex].PublicKey)
	acc, err := simpleWallet.Unlock(ethwallet.AsWalletAddr(addr))
	require.NoError(t, err)
	require.NotNil(t, acc)

	validAddrBytes, err := hex.DecodeString(sampleAddr)
	require.NoError(t, err, "invalid sample address")

	return &test.Setup{
		UnlockedAccount: func() (wallet.Account, error) { return acc, nil },
		Backend:         new(ethwallet.Backend),
		AddressBytes:    validAddrBytes,
		DataToSign:      dataToSign,
	}, simpleWallet
}
