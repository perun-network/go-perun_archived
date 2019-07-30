// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package wire

import (
	"io"
	"reflect"

	"github.com/pkg/errors"

	"perun.network/go-perun/log"
	"perun.network/go-perun/pkg/test"
)

// ByteSlice is a serializable byte slice.
type ByteSlice []byte

// Decode reads a byte slice from the given stream.
// Decode reads exactly len(b) bytes.
// This means the caller has to specify how many bytes he wants to read.
func (b *ByteSlice) Decode(reader io.Reader) error {
	_, err := io.ReadFull(reader, *b)
	return errors.Wrap(err, "failed to read []byte")
}

// Encode writes len(b) to the stream.
func (b ByteSlice) Encode(writer io.Writer) error {
	_, err := writer.Write(b)
	return errors.Wrap(err, "failed to write []byte")
}

// Converts a byte array to a byte slice.
// If the passed value is not a byte array (or byte slice), returns false.
func tryCastFromArray(v interface{}) (bool, ByteSlice) {
	// Get the reclect value.
	rv := reflect.ValueOf(v)
	// Dereference if pointer.
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.Slice:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			return true, rv.Interface().([]byte)
		} else {
			return false, nil
		}
	case reflect.Array:
		// Try to convert it to a slice.
		if failed, msg := test.CheckPanic(func() { rv = rv.Slice(0, rv.Len()) }); failed {
			log.Panicf("Failed to convert array to slice: %v", msg)
			panic("This should never happen")
		}

		if rv.Type().Elem().Kind() != reflect.Uint8 {
			return false, nil
		}

		// Return the byte slice.
		//log.Panicf("Success %T, %v", rv.Interface(), rv.Interface())
		return true, rv.Interface().([]byte)
	default:
		return false, nil
	}
}
