// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build windows

package perfctl

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// SupportsPinning reports whether this platform has a CPU performance control
// implementation.
const SupportsPinning = true

var (
	powrprof                   = windows.NewLazySystemDLL("powrprof.dll")
	procPowerGetActiveScheme   = powrprof.NewProc("PowerGetActiveScheme")
	procPowerReadACValueIndex  = powrprof.NewProc("PowerReadACValueIndex")
	procPowerReadDCValueIndex  = powrprof.NewProc("PowerReadDCValueIndex")
	procPowerWriteACValueIndex = powrprof.NewProc("PowerWriteACValueIndex")
	procPowerWriteDCValueIndex = powrprof.NewProc("PowerWriteDCValueIndex")
	procPowerSetActiveScheme   = powrprof.NewProc("PowerSetActiveScheme")

	processorSettings = mustGUID("{54533251-82be-4824-96c1-47b60b740d00}")
	processorMinimum  = mustGUID("{893dee8e-2bef-41e0-89c6-b55d0929964c}")
	processorMaximum  = mustGUID("{bc5038f7-23e0-4960-96da-33abaf5935ec}")
)

type windowsController struct {
	scheme windows.GUID
	api    powerAPI
}

type powerValues struct {
	acMin uint32
	acMax uint32
	dcMin uint32
	dcMax uint32
}

type powerAPI interface {
	readAC(scheme, setting *windows.GUID) (uint32, error)
	readDC(scheme, setting *windows.GUID) (uint32, error)
	writeAC(scheme, setting *windows.GUID, value uint32) error
	writeDC(scheme, setting *windows.GUID, value uint32) error
	activate(scheme *windows.GUID) error
}

type systemPowerAPI struct{}

// Open opens the active Windows power scheme.
func Open() (Controller, error) {
	var schemePointer unsafe.Pointer
	if err := powerCall("get active power scheme", procPowerGetActiveScheme, 0, uintptr(unsafe.Pointer(&schemePointer))); err != nil {
		return nil, err
	}
	if schemePointer == nil {
		return nil, fmt.Errorf("get active power scheme: returned a nil scheme")
	}
	defer windows.LocalFree(windows.Handle(schemePointer))
	scheme := *(*windows.GUID)(schemePointer)
	return &windowsController{scheme: scheme, api: systemPowerAPI{}}, nil
}

func (c *windowsController) Pin(percent int) (func() error, error) {
	if percent < 0 || percent > 100 {
		return nil, fmt.Errorf("CPU performance percentage %d is outside 0-100", percent)
	}
	old, err := c.readValues()
	if err != nil {
		return nil, err
	}
	restore := func() error { return c.writeValues(old) }
	target := uint32(percent)
	if err := c.writeValues(powerValues{target, target, target, target}); err != nil {
		if restoreErr := restore(); restoreErr != nil {
			return nil, fmt.Errorf("pin CPU performance: %v; restore prior settings: %w", err, restoreErr)
		}
		return nil, fmt.Errorf("pin CPU performance: %w", err)
	}
	return restore, nil
}

func (c *windowsController) readValues() (powerValues, error) {
	var values powerValues
	reads := []struct {
		name    string
		setting *windows.GUID
		dc      bool
		value   *uint32
	}{
		{"AC minimum processor state", &processorMinimum, false, &values.acMin},
		{"AC maximum processor state", &processorMaximum, false, &values.acMax},
		{"DC minimum processor state", &processorMinimum, true, &values.dcMin},
		{"DC maximum processor state", &processorMaximum, true, &values.dcMax},
	}
	for _, read := range reads {
		var (
			value uint32
			err   error
		)
		if read.dc {
			value, err = c.api.readDC(&c.scheme, read.setting)
		} else {
			value, err = c.api.readAC(&c.scheme, read.setting)
		}
		if err != nil {
			return powerValues{}, fmt.Errorf("read %s: %w", read.name, err)
		}
		*read.value = value
	}
	return values, nil
}

func (c *windowsController) writeValues(values powerValues) error {
	writes := []struct {
		name    string
		setting *windows.GUID
		dc      bool
		value   uint32
	}{
		{"AC minimum processor state", &processorMinimum, false, values.acMin},
		{"AC maximum processor state", &processorMaximum, false, values.acMax},
		{"DC minimum processor state", &processorMinimum, true, values.dcMin},
		{"DC maximum processor state", &processorMaximum, true, values.dcMax},
	}
	var firstErr error
	for _, write := range writes {
		var err error
		if write.dc {
			err = c.api.writeDC(&c.scheme, write.setting, write.value)
		} else {
			err = c.api.writeAC(&c.scheme, write.setting, write.value)
		}
		if err != nil && firstErr == nil {
			firstErr = fmt.Errorf("write %s: %w", write.name, err)
		}
	}
	if err := c.api.activate(&c.scheme); err != nil && firstErr == nil {
		firstErr = fmt.Errorf("activate power scheme: %w", err)
	}
	return firstErr
}

func (systemPowerAPI) readAC(scheme, setting *windows.GUID) (uint32, error) {
	return readPowerValue("read AC power value", procPowerReadACValueIndex, scheme, setting)
}

func (systemPowerAPI) readDC(scheme, setting *windows.GUID) (uint32, error) {
	return readPowerValue("read DC power value", procPowerReadDCValueIndex, scheme, setting)
}

func (systemPowerAPI) writeAC(scheme, setting *windows.GUID, value uint32) error {
	return writePowerValue("write AC power value", procPowerWriteACValueIndex, scheme, setting, value)
}

func (systemPowerAPI) writeDC(scheme, setting *windows.GUID, value uint32) error {
	return writePowerValue("write DC power value", procPowerWriteDCValueIndex, scheme, setting, value)
}

func (systemPowerAPI) activate(scheme *windows.GUID) error {
	return powerCall("activate power scheme", procPowerSetActiveScheme, 0, uintptr(unsafe.Pointer(scheme)))
}

func readPowerValue(operation string, proc *windows.LazyProc, scheme, setting *windows.GUID) (uint32, error) {
	var value uint32
	err := powerCall(operation, proc,
		0,
		uintptr(unsafe.Pointer(scheme)),
		uintptr(unsafe.Pointer(&processorSettings)),
		uintptr(unsafe.Pointer(setting)),
		uintptr(unsafe.Pointer(&value)),
	)
	return value, err
}

func writePowerValue(operation string, proc *windows.LazyProc, scheme, setting *windows.GUID, value uint32) error {
	return powerCall(operation, proc,
		0,
		uintptr(unsafe.Pointer(scheme)),
		uintptr(unsafe.Pointer(&processorSettings)),
		uintptr(unsafe.Pointer(setting)),
		uintptr(value),
	)
}

func powerCall(operation string, proc *windows.LazyProc, args ...uintptr) error {
	result, _, _ := proc.Call(args...)
	if result != 0 {
		return fmt.Errorf("%s: %w", operation, syscall.Errno(result))
	}
	return nil
}

func mustGUID(value string) windows.GUID {
	guid, err := windows.GUIDFromString(value)
	if err != nil {
		panic(err)
	}
	return guid
}
