// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build linux

package perfctl

import "testing"

func TestNearest(t *testing.T) {
	available := []int{100, 200, 400}
	for _, test := range []struct {
		target int
		want   int
	}{
		{50, 100},
		{100, 100},
		{149, 100},
		{151, 200},
		{300, 200},
		{301, 400},
		{500, 400},
	} {
		if got := nearest(test.target, available); got != test.want {
			t.Errorf("nearest(%d, %v) = %d, want %d", test.target, available, got, test.want)
		}
	}
}
