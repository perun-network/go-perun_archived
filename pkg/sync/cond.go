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

package sync

import (
	"context"
	"sync"
)

// Cond is a condition variable implementation analog to sync.Cond, but with
// the addition of WaitCtx.
type Cond struct {
	signal Signal
	lock   sync.Locker
}

// NewCond creates a new condition variable associated with the specified lock.
func NewCond(l sync.Locker) *Cond {
	return &Cond{
		signal: *NewSignal(),
		lock:   l,
	}
}

// Broadcast wakes up all waiting coroutines.
func (c *Cond) Broadcast() {
	c.signal.Broadcast()
}

// Signal wakes up a single waiting coroutine, if any.
func (c *Cond) Signal() {
	c.signal.Signal()
}

// Wait releases the condition variable's lock and waits until it is notified.
// Locks the condition variable's lock after resuming. Due to the slight delay
// between resuming and re-locking, the condition predicate has to be checked
// manually afterwards.
func (c *Cond) Wait() {
	c.lock.Unlock()
	c.signal.Wait()
	c.lock.Lock()
}

// WaitCtx behaves like Wait but resumes early if the context expires. Returns
// whether the condition variable was notified.
func (c *Cond) WaitCtx(ctx context.Context) bool {
	c.lock.Unlock()
	ret := c.signal.WaitCtx(ctx)
	c.lock.Lock()
	return ret
}
