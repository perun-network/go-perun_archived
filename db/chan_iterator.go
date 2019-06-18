// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package db

import (
	"bytes"
	"fmt"
)

type Item struct {
	Key   []byte
	Value []byte
}

func (i Item) isNil() bool {
	return i.Key == nil
}

func (i Item) String() string {
	return fmt.Sprintf("{%x: %x}", i.Key, i.Value)
}

func (i Item) Equal(other *Item) bool {
	return bytes.Equal(i.Key, other.Key) && bytes.Equal(i.Value, other.Value)

}

// ChanIterator is an iterator that can be iterated via channels.
// Release() should be called at the end to free up resources.
// A ChanIterator `cit` should be called in a for/select loop like so:
// ```
//	for item := range cit.Items():
//		// do something with item
//	}
//	cit.Done() <- struct{}{}
//	if err := cit.Error(); err != nil {
//		// handle err
//	}
// ```
// The implementation should create a single items and error channel during
// creation and return the same channels on multiple calls to Items() and Err()
// See IteratorChanWrapper and its contructor AsChanIterator for an example
// how to implement a ChanIterator
type ChanIterator interface {
	Items() <-chan Item

	//
	Error() error

	// Done tells the iterator that reading from Items() is done and ressources
	// can be released.
	Done() chan<- struct{}
}

// Iteratee wraps the NewIterator methods of a backing data store.
type ChanIterateable interface {
	// NewIterator creates a binary-alphabetical iterator over the entire keyspace
	// contained within the key-value database.
	NewChanIterator() ChanIterator

	// NewIteratorWithStart creates a binary-alphabetical iterator over a subset of
	// database content starting at a particular initial key (or after, if it does
	// not exist).
	NewChanIteratorWithStart(start []byte) ChanIterator

	// NewIteratorWithPrefix creates a binary-alphabetical iterator over a subset
	// of database content with a particular key prefix.
	NewChanIteratorWithPrefix(prefix []byte) ChanIterator
}
