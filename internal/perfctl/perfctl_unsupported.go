// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !linux && !windows

package perfctl

// SupportsPinning reports whether this platform has a CPU performance control
// implementation.
const SupportsPinning = false

// Open opens the host CPU performance controls.
func Open() (Controller, error) {
	return nil, ErrUnsupported
}
