// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package powermode controls macOS power mode while a benchmark runs.
package powermode

import (
	"errors"
	"fmt"
)

// ErrUnsupported indicates that the host does not expose the requested power
// mode.
var ErrUnsupported = errors.New("power mode unsupported on this platform")

// Mode is a macOS system power mode.
type Mode int

const (
	Automatic Mode = iota
	Low
	High
)

func (m Mode) String() string {
	switch m {
	case Automatic:
		return "auto"
	case Low:
		return "low"
	case High:
		return "high"
	default:
		return fmt.Sprintf("Mode(%d)", m)
	}
}

// Parse parses a command-line power mode.
func Parse(value string) (Mode, error) {
	switch value {
	case "auto":
		return Automatic, nil
	case "low":
		return Low, nil
	case "high":
		return High, nil
	default:
		return Automatic, fmt.Errorf("power mode must be auto, low, or high")
	}
}

// Controller changes the host power mode while a benchmark runs.
type Controller interface {
	Set(mode Mode) (restore func() error, err error)
}

// Message is one macOS benchmark-environment observation.
type Message struct {
	Warning bool
	Text    string
}
