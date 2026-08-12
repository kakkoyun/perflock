// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build darwin

package powermode

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

const (
	pmsetPath = "/usr/bin/pmset"
	// Supported reports whether this platform exposes macOS power modes.
	Supported = true
)

type settings struct {
	battery *Mode
	charger *Mode
}

type commandRunner interface {
	run(args ...string) ([]byte, error)
}

type systemRunner struct{}

func (systemRunner) run(args ...string) ([]byte, error) {
	return exec.Command(pmsetPath, args...).CombinedOutput()
}

type darwinController struct {
	runner commandRunner
}

// Open opens the macOS power-mode controls.
func Open() (Controller, error) {
	return &darwinController{runner: systemRunner{}}, nil
}

func (c *darwinController) Set(mode Mode) (func() error, error) {
	if mode < Automatic || mode > High {
		return nil, fmt.Errorf("invalid power mode %d", mode)
	}
	old, err := c.readSettings()
	if err != nil {
		return nil, err
	}
	if old.battery == nil && old.charger == nil {
		return nil, fmt.Errorf("%w: pmset does not report powermode", ErrUnsupported)
	}

	restore := func() error { return c.restoreSettings(old) }
	if err := c.setAll(mode); err != nil {
		return nil, combineRestoreError(err, restore())
	}
	current, err := c.readSettings()
	if err != nil {
		restoreErr := restore()
		return nil, combineRestoreError(err, restoreErr)
	}
	if !settingsMatch(current, old, mode) {
		restoreErr := restore()
		err := fmt.Errorf("%w: pmset did not apply %s power mode", ErrUnsupported, mode)
		return nil, combineRestoreError(err, restoreErr)
	}
	return restore, nil
}

func (c *darwinController) readSettings() (settings, error) {
	output, err := c.runner.run("-g", "custom")
	if err != nil {
		return settings{}, commandError("read power modes", output, err)
	}
	parsed, err := parseSettings(string(output))
	if err != nil {
		return settings{}, err
	}
	return parsed, nil
}

func (c *darwinController) setAll(mode Mode) error {
	output, err := c.runner.run("-a", "powermode", strconv.Itoa(int(mode)))
	if err != nil {
		return commandError("set power mode", output, err)
	}
	return nil
}

func (c *darwinController) restoreSettings(old settings) error {
	var firstErr error
	if old.battery != nil {
		output, err := c.runner.run("-b", "powermode", strconv.Itoa(int(*old.battery)))
		if err != nil {
			firstErr = commandError("restore battery power mode", output, err)
		}
	}
	if old.charger != nil {
		output, err := c.runner.run("-c", "powermode", strconv.Itoa(int(*old.charger)))
		if err != nil && firstErr == nil {
			firstErr = commandError("restore charger power mode", output, err)
		}
	}
	return firstErr
}

func parseSettings(output string) (settings, error) {
	var (
		parsed  settings
		section **Mode
	)
	for _, rawLine := range strings.Split(output, "\n") {
		line := strings.TrimSpace(rawLine)
		switch line {
		case "Battery Power:":
			section = &parsed.battery
			continue
		case "AC Power:":
			section = &parsed.charger
			continue
		}
		if section == nil {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[0] != "powermode" {
			continue
		}
		value, err := strconv.Atoi(fields[1])
		if err != nil || value < int(Automatic) || value > int(High) {
			return settings{}, fmt.Errorf("parse pmset powermode value %q", fields[1])
		}
		mode := Mode(value)
		*section = &mode
	}
	return parsed, nil
}

func settingsMatch(current, original settings, want Mode) bool {
	if original.battery != nil && (current.battery == nil || *current.battery != want) {
		return false
	}
	if original.charger != nil && (current.charger == nil || *current.charger != want) {
		return false
	}
	return true
}

func commandError(operation string, output []byte, err error) error {
	detail := strings.TrimSpace(string(output))
	if detail == "" {
		return fmt.Errorf("%s: %w", operation, err)
	}
	return fmt.Errorf("%s: %s: %w", operation, detail, err)
}

func combineRestoreError(err, restoreErr error) error {
	if restoreErr == nil {
		return err
	}
	return fmt.Errorf("%v; restore prior power mode: %w", err, restoreErr)
}
