// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build linux

package perfctl

import (
	"errors"
	"strings"
	"testing"
)

type fakeFrequencyDomain struct {
	availableMin int
	availableMax int
	available    []int
	currentMin   int
	currentMax   int
	currentErr   error
	setErrAt     int
	setCalls     [][2]int
}

func (f *fakeFrequencyDomain) AvailableRange() (int, int, []int) {
	return f.availableMin, f.availableMax, f.available
}

func (f *fakeFrequencyDomain) CurrentRange() (int, int, error) {
	return f.currentMin, f.currentMax, f.currentErr
}

func (f *fakeFrequencyDomain) SetRange(min, max int) error {
	f.setCalls = append(f.setCalls, [2]int{min, max})
	if f.setErrAt > 0 && len(f.setCalls) == f.setErrAt {
		return errors.New("set failed")
	}
	return nil
}

func TestLinuxControllerPinAndRestore(t *testing.T) {
	first := &fakeFrequencyDomain{
		availableMin: 100,
		availableMax: 500,
		available:    []int{100, 300, 500},
		currentMin:   100,
		currentMax:   500,
	}
	second := &fakeFrequencyDomain{
		availableMin: 200,
		availableMax: 600,
		currentMin:   250,
		currentMax:   550,
	}
	controller := &linuxController{domains: []frequencyDomain{first, second}}

	restore, err := controller.Pin(50)
	if err != nil {
		t.Fatalf("Pin: %v", err)
	}
	assertSetCalls(t, first, [][2]int{{300, 300}})
	assertSetCalls(t, second, [][2]int{{400, 400}})

	if err := restore(); err != nil {
		t.Fatalf("restore: %v", err)
	}
	assertSetCalls(t, first, [][2]int{{300, 300}, {100, 500}})
	assertSetCalls(t, second, [][2]int{{400, 400}, {250, 550}})
}

func TestLinuxControllerRollsBackPartialFailure(t *testing.T) {
	first := &fakeFrequencyDomain{
		availableMin: 100,
		availableMax: 500,
		currentMin:   100,
		currentMax:   500,
	}
	second := &fakeFrequencyDomain{
		availableMin: 200,
		availableMax: 600,
		currentMin:   250,
		currentMax:   550,
		setErrAt:     1,
	}
	controller := &linuxController{domains: []frequencyDomain{first, second}}

	restore, err := controller.Pin(50)
	if restore != nil {
		t.Fatal("Pin returned a restore function after failure")
	}
	if err == nil || !strings.Contains(err.Error(), "set CPU frequency range") {
		t.Fatalf("Pin error = %v", err)
	}
	assertSetCalls(t, first, [][2]int{{300, 300}, {100, 500}})
	assertSetCalls(t, second, [][2]int{{400, 400}, {250, 550}})
}

func TestLinuxControllerRestoreContinuesAfterError(t *testing.T) {
	first := &fakeFrequencyDomain{
		availableMin: 100,
		availableMax: 500,
		currentMin:   100,
		currentMax:   500,
		setErrAt:     2,
	}
	second := &fakeFrequencyDomain{
		availableMin: 200,
		availableMax: 600,
		currentMin:   250,
		currentMax:   550,
	}
	controller := &linuxController{domains: []frequencyDomain{first, second}}

	restore, err := controller.Pin(50)
	if err != nil {
		t.Fatalf("Pin: %v", err)
	}
	if err := restore(); err == nil || !strings.Contains(err.Error(), "restore CPU frequency range") {
		t.Fatalf("restore error = %v", err)
	}
	assertSetCalls(t, second, [][2]int{{400, 400}, {250, 550}})
}

func TestLinuxControllerReadFailureDoesNotWrite(t *testing.T) {
	first := &fakeFrequencyDomain{currentErr: errors.New("read failed")}
	second := &fakeFrequencyDomain{}
	controller := &linuxController{domains: []frequencyDomain{first, second}}

	if _, err := controller.Pin(50); err == nil || !strings.Contains(err.Error(), "read current CPU frequency range") {
		t.Fatalf("Pin error = %v", err)
	}
	assertSetCalls(t, first, nil)
	assertSetCalls(t, second, nil)
}

func TestLinuxControllerRejectsInvalidPercent(t *testing.T) {
	controller := &linuxController{}
	for _, percent := range []int{-1, 101} {
		if _, err := controller.Pin(percent); err == nil {
			t.Errorf("Pin(%d) succeeded", percent)
		}
	}
}

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

func assertSetCalls(t *testing.T, domain *fakeFrequencyDomain, want [][2]int) {
	t.Helper()
	if len(domain.setCalls) != len(want) {
		t.Fatalf("SetRange calls = %v, want %v", domain.setCalls, want)
	}
	for index := range want {
		if domain.setCalls[index] != want[index] {
			t.Fatalf("SetRange calls = %v, want %v", domain.setCalls, want)
		}
	}
}
