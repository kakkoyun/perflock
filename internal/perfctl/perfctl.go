// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package perfctl controls host CPU performance settings while a benchmark runs.
package perfctl

import "errors"

// ErrUnsupported indicates that the host does not expose CPU performance
// controls that perflock can use.
var ErrUnsupported = errors.New("CPU performance control unsupported on this platform")

// Controller constrains system CPU performance while a benchmark runs.
type Controller interface {
	// Pin constrains CPU performance to percent of the available range. The
	// returned function restores the settings that were active before Pin.
	Pin(percent int) (restore func() error, err error)
}
