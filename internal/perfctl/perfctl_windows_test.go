// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build windows

package perfctl

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

type fakePowerAPI struct {
	values         powerValues
	writes         []powerValues
	activations    int
	readCalls      int
	writeCalls     int
	failReadAt     int
	failWriteAt    int
	failActivateAt int
}

func (f *fakePowerAPI) readAC(_ *windows.GUID, setting *windows.GUID) (uint32, error) {
	f.readCalls++
	if f.readCalls == f.failReadAt {
		return 0, errors.New("read failed")
	}
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
	f.readCalls++
	if f.readCalls == f.failReadAt {
		return 0, errors.New("read failed")
	}
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
	f.writeCalls++
	if f.writeCalls == f.failWriteAt {
		return errors.New("write failed")
	}
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
	f.writeCalls++
	if f.writeCalls == f.failWriteAt {
		return errors.New("write failed")
	}
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
	if f.activations == f.failActivateAt {
		return errors.New("activate failed")
	}
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

func TestWindowsControllerReadFailureDoesNotWrite(t *testing.T) {
	api := &fakePowerAPI{failReadAt: 3}
	controller := &windowsController{api: api}

	restore, err := controller.Pin(65)
	if restore != nil {
		t.Fatal("Pin returned restore function after read failure")
	}
	if err == nil || !strings.Contains(err.Error(), "read DC minimum processor state") {
		t.Fatalf("Pin error = %v", err)
	}
	if api.writeCalls != 0 || api.activations != 0 {
		t.Fatalf("read failure caused %d writes and %d activations", api.writeCalls, api.activations)
	}
}

func TestWindowsControllerRollsBackPartialWriteFailure(t *testing.T) {
	original := powerValues{acMin: 5, acMax: 100, dcMin: 10, dcMax: 80}
	api := &fakePowerAPI{values: original, failWriteAt: 2}
	controller := &windowsController{api: api}

	restore, err := controller.Pin(65)
	if restore != nil {
		t.Fatal("Pin returned restore function after write failure")
	}
	if err == nil || !strings.Contains(err.Error(), "pin CPU performance") {
		t.Fatalf("Pin error = %v", err)
	}
	if !reflect.DeepEqual(api.values, original) {
		t.Fatalf("values after rollback = %+v, want %+v", api.values, original)
	}
	if api.writeCalls != 8 || api.activations != 2 {
		t.Fatalf("rollback made %d writes and %d activations, want 8 and 2", api.writeCalls, api.activations)
	}
}

func TestWindowsControllerRollsBackActivationFailure(t *testing.T) {
	original := powerValues{acMin: 5, acMax: 100, dcMin: 10, dcMax: 80}
	api := &fakePowerAPI{values: original, failActivateAt: 1}
	controller := &windowsController{api: api}

	restore, err := controller.Pin(65)
	if restore != nil {
		t.Fatal("Pin returned restore function after activation failure")
	}
	if err == nil || !strings.Contains(err.Error(), "activate power scheme") {
		t.Fatalf("Pin error = %v", err)
	}
	if !reflect.DeepEqual(api.values, original) {
		t.Fatalf("values after rollback = %+v, want %+v", api.values, original)
	}
	if api.activations != 2 {
		t.Fatalf("power scheme activated %d times, want 2", api.activations)
	}
}

func TestWindowsControllerRestoreContinuesAfterFailure(t *testing.T) {
	original := powerValues{acMin: 5, acMax: 100, dcMin: 10, dcMax: 80}
	api := &fakePowerAPI{values: original}
	controller := &windowsController{api: api}

	restore, err := controller.Pin(65)
	if err != nil {
		t.Fatalf("Pin: %v", err)
	}
	api.failWriteAt = api.writeCalls + 2
	if err := restore(); err == nil || !strings.Contains(err.Error(), "write AC maximum processor state") {
		t.Fatalf("restore error = %v", err)
	}
	want := powerValues{acMin: 5, acMax: 65, dcMin: 10, dcMax: 80}
	if !reflect.DeepEqual(api.values, want) {
		t.Fatalf("values after failed restore = %+v, want %+v", api.values, want)
	}
	if api.writeCalls != 8 || api.activations != 2 {
		t.Fatalf("restore made %d writes and %d activations, want 8 and 2", api.writeCalls, api.activations)
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

func TestSystemPowerAPIReadsActiveScheme(t *testing.T) {
	controller, err := Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	windowsController, ok := controller.(*windowsController)
	if !ok {
		t.Fatalf("Open returned %T", controller)
	}
	values, err := windowsController.readValues()
	if err != nil {
		t.Fatalf("read active power scheme: %v", err)
	}
	for name, value := range map[string]uint32{
		"AC minimum": values.acMin,
		"AC maximum": values.acMax,
		"DC minimum": values.dcMin,
		"DC maximum": values.dcMax,
	} {
		if value > 100 {
			t.Errorf("%s processor state = %d, want 0-100", name, value)
		}
	}
	t.Logf("active processor state: AC %d-%d%%, DC %d-%d%%", values.acMin, values.acMax, values.dcMin, values.dcMax)
}
