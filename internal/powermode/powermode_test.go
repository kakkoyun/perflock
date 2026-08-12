// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package powermode

import "testing"

func TestParse(t *testing.T) {
	for value, want := range map[string]Mode{
		"auto": Automatic,
		"low":  Low,
		"high": High,
	} {
		got, err := Parse(value)
		if err != nil {
			t.Errorf("Parse(%q): %v", value, err)
			continue
		}
		if got != want {
			t.Errorf("Parse(%q) = %v, want %v", value, got, want)
		}
	}
	if _, err := Parse("maximum"); err == nil {
		t.Fatal("Parse accepted an unknown power mode")
	}
}
