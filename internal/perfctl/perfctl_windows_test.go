// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build windows

package perfctl

import (
	"fmt"
	"reflect"
	"testing"

	"golang.org/x/sys/windows"
)

type fakePowerAPI struct {
	values      powerValues
	writes      []powerValues
	activations int
}

func (f *fakePowerAPI) readAC(_ *windows.GUID, setting *windows.GUID) (uint32, error) {
	switch *setting {
	case processorMinimum:
		return f.values.acMin, nil
	case processorMaximum:
		return f.values.acMax, nil
	default:
		return 0, fmt.Errorf("unknown setting %v", setting)
	}
}

func (f *fakePowerAPI) readDC(_ *windows.GUID, setting *windows.GUID) (uint32, error) {
	switch *setting {
	case processorMinimum:
		return f.values.dcMin, nil
	case processorMaximum:
		return f.values.dcMax, nil
	default:
		return 0, fmt.Errorf("unknown setting %v", setting)
	}
}

func (f *fakePowerAPI) writeAC(_ *windows.GUID, setting *windows.GUID, value uint32) error {
	switch *setting {
	case processorMinimum:
		f.values.acMin = value
	case processorMaximum:
		f.values.acMax = value
	default:
		return fmt.Errorf("unknown setting %v", setting)
	}
	return nil
}

func (f *fakePowerAPI) writeDC(_ *windows.GUID, setting *windows.GUID, value uint32) error {
	switch *setting {
	case processorMinimum:
		f.values.dcMin = value
	case processorMaximum:
		f.values.dcMax = value
	default:
		return fmt.Errorf("unknown setting %v", setting)
	}
	return nil
}

func (f *fakePowerAPI) activate(_ *windows.GUID) error {
	f.activations++
	f.writes = append(f.writes, f.values)
	return nil
}

func TestWindowsControllerPinAndRestore(t *testing.T) {
	original := powerValues{acMin: 5, acMax: 100, dcMin: 10, dcMax: 80}
	api := &fakePowerAPI{values: original}
	controller := &windowsController{api: api}

	restore, err := controller.Pin(65)
	if err != nil {
		t.Fatalf("Pin: %v", err)
	}
	pinned := powerValues{acMin: 65, acMax: 65, dcMin: 65, dcMax: 65}
	if !reflect.DeepEqual(api.values, pinned) {
		t.Fatalf("pinned values = %+v, want %+v", api.values, pinned)
	}
	if err := restore(); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if !reflect.DeepEqual(api.values, original) {
		t.Fatalf("restored values = %+v, want %+v", api.values, original)
	}
	if api.activations != 2 {
		t.Fatalf("power scheme activated %d times, want 2", api.activations)
	}
}

func TestWindowsControllerRejectsInvalidPercent(t *testing.T) {
	controller := &windowsController{api: &fakePowerAPI{}}
	for _, percent := range []int{-1, 101} {
		if _, err := controller.Pin(percent); err == nil {
			t.Errorf("Pin(%d) succeeded", percent)
		}
	}
}
