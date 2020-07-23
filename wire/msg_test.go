// Copyright (c) 2019 Chair of Applied Cryptography, Technische Universität
// Darmstadt, Germany. All rights reserved. This file is part of go-perun. Use
// of this source code is governed by the Apache 2.0 license that can be found
// in the LICENSE file.

package wire

import (
	"io"
	"math/rand"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	iotest "perun.network/go-perun/pkg/io/test"
	"perun.network/go-perun/pkg/test"
	wtest "perun.network/go-perun/wallet/test"
)

func NewRandomEnvelope(rng *rand.Rand, m Msg) *Envelope {
	return &Envelope{wtest.NewRandomAddress(rng), wtest.NewRandomAddress(rng), m}
}

var nilDecoder = func(io.Reader) (Msg, error) { return nil, nil }

func TestType_Valid_String(t *testing.T) {
	test.OnlyOnce(t)

	const testTypeVal, testTypeStr = 252, "testTypeA"
	testType := Type(testTypeVal)
	assert.False(t, testType.Valid(), "unregistered type should not be valid")
	assert.Equal(t, strconv.Itoa(testTypeVal), testType.String(),
		"unregistered type's String() should return its integer value")

	RegisterExternalDecoder(testTypeVal, nilDecoder, testTypeStr)
	assert.True(t, testType.Valid(), "registered type should be valid")
	assert.Equal(t, testTypeStr, testType.String(),
		"registered type's String() should be 'testType'")
}

func TestRegisterExternalDecoder(t *testing.T) {
	test.OnlyOnce(t)

	const testTypeVal, testTypeStr = 251, "testTypeB"

	RegisterExternalDecoder(testTypeVal, nilDecoder, testTypeStr)
	assert.Panics(t,
		func() { RegisterExternalDecoder(testTypeVal, nilDecoder, testTypeStr) },
		"second registration of same type should fail",
	)
	assert.Panics(t,
		func() { RegisterExternalDecoder(Ping, nilDecoder, "PingFail") },
		"registration of internal type should fail",
	)
}

func TestEnvelope_EncodeDecode(t *testing.T) {
	ping := NewRandomEnvelope(test.Prng(t), NewPingMsg())
	iotest.GenericSerializerTest(t, ping)
}
