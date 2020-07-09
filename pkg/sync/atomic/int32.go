// Copyright (c) 2020 Chair of Applied Cryptography, Technische Universität
// Darmstadt, Germany. All rights reserved. This file is part of go-perun. Use
// of this source code is governed by the Apache 2.0 license that can be found
// in the LICENSE file.

package atomic

import "sync/atomic"

type Int32 struct {
	v int32
}

func (i *Int32) Init(v int32) {
	i.v = v
}

func (i *Int32) Store(v int32) {
	atomic.StoreInt32(&i.v, v)
}

func (i *Int32) Load() int32 {
	return atomic.LoadInt32(&i.v)
}

func (i *Int32) Swap(v int32) int32 {
	return atomic.SwapInt32(&i.v, v)
}

func (i *Int32) CompareAndSwap(expected, v int32) (swapped bool) {
	return atomic.CompareAndSwapInt32(&i.v, expected, v)
}

func (i *Int32) Add(v int32) (new int32) {
	return atomic.AddInt32(&i.v, v)
}
