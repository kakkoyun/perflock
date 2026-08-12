// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package ipc provides perflock's local client/server transport.
package ipc

import "errors"

var (
	// ErrAlreadyRunning indicates that a daemon already owns the endpoint.
	ErrAlreadyRunning = errors.New("perflock daemon is already running")
	// ErrPathTooLong indicates that a UNIX socket path exceeds the host limit.
	ErrPathTooLong = errors.New("UNIX socket path is too long")
)
