// Copyright (c) 2020 Chair of Applied Cryptography, Technische Universität
// Darmstadt, Germany. All rights reserved. This file is part of go-perun. Use
// of this source code is governed by the Apache 2.0 license that can be found
// in the LICENSE file.

package sync

import (
	"runtime"
	"sync"

	"perun.network/go-perun/pkg/sync/atomic"
)

// RWMutex is a shared mutex with upgradeable locks. Exclusive locks cannot be
// downgraded to shared locks. Any shared lock can be upgraded.
type RWMutex struct {
	master       sync.RWMutex
	transferring atomic.Int32
}

// Upgrade upgrades a shared lock to an exclusive lock. This operation takes
// precedence over all other locking operations.
func (m *RWMutex) Upgrade() {
	m.transferring.Add(1)
	m.master.RUnlock()
	m.master.Lock()
	m.transferring.Add(-1)
}

// Lock acquires an exclusive lock. This operation has lower priority than
// upgrading.
func (m *RWMutex) Lock() {
retry:
	m.master.Lock()
	// Roll back if we just interrupted an upgrade.
	if m.transferring.Load() != 0 {
		m.master.Unlock()
		runtime.Gosched()
		goto retry
	}
}

// Unlock releases an exclusive lock.
func (m *RWMutex) Unlock() {
	m.master.Unlock()
}

// RLock acquires a shared lock. This operation has lower priority than
// upgrading.
func (m *RWMutex) RLock() {
retry:
	m.master.RLock()
	// If someone is currently upgrading, back down.
	if m.transferring.Load() != 0 {
		m.master.RUnlock()
		runtime.Gosched()
		goto retry
	}
}

// RUnlock releases a shared lock. If a shared lock has been upgraded, call
// Unlock instead.
func (m *RWMutex) RUnlock() {
	m.master.RUnlock()
}
