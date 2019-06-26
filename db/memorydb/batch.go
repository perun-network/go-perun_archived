// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package memorydb

import (
	"github.com/pkg/errors"
)

type Batch struct {
	db    *Database
	cache map[string][]byte
	bytes uint
}

func (this *Batch) Put(key []byte, value []byte) error {
	oldValue, exists := this.cache[string(key)]
	if exists {
		this.bytes -= uint(len(oldValue))
	}
	this.bytes += uint(len(value))
	this.cache[string(key)] = value
	return nil
}

func (this *Batch) Delete(key []byte) error {
	oldValue, exists := this.cache[string(key)]
	if !exists {
		return errors.New("Tried to delete nonexistent entry.")
	} else {
		this.bytes -= uint(len(oldValue))
		delete(this.cache, string(key))
		return nil
	}
}

func (this *Batch) Len() uint {
	return uint(len(this.cache))
}

func (this *Batch) ValueSize() uint {
	return this.bytes
}

func (this *Batch) Write() error {
	for key := range this.cache {
		err := this.db.Put([]byte(key), this.cache[key])
		if err != nil {
			return errors.Wrap(err, "Failed to put entry.")
		}
	}
	return nil
}

func (this *Batch) Reset() {
	this.cache = nil
	this.bytes = 0
}
