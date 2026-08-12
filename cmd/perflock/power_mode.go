// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "github.com/aclements/perflock/internal/powermode"

type powerModeFlag struct {
	mode     powermode.Mode
	explicit bool
}

func (f *powerModeFlag) String() string {
	return f.mode.String()
}

func (f *powerModeFlag) Set(value string) error {
	mode, err := powermode.Parse(value)
	if err != nil {
		return err
	}
	f.mode = mode
	f.explicit = true
	return nil
}
