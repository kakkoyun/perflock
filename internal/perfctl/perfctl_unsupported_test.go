// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !linux && !windows

package perfctl

import (
	"errors"
	"testing"
)

func TestOpenUnsupported(t *testing.T) {
	controller, err := Open()
	if controller != nil {
		t.Fatalf("Open returned controller %T", controller)
	}
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("Open error = %v, want %v", err, ErrUnsupported)
	}
}
