// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !darwin

package powermode

// Supported reports whether this platform exposes macOS power modes.
const Supported = false

// Open opens the host power-mode controls.
func Open() (Controller, error) {
	return nil, ErrUnsupported
}
