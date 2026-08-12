// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build darwin

package powermode

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

type fakeRunner struct {
	battery   Mode
	charger   Mode
	ignoreSet bool
	commands  [][]string
}

func (f *fakeRunner) run(args ...string) ([]byte, error) {
	f.commands = append(f.commands, append([]string(nil), args...))
	if len(args) == 2 && args[0] == "-g" && args[1] == "custom" {
		return []byte(fmt.Sprintf("Battery Power:\n powermode %d\nAC Power:\n powermode %d\n", f.battery, f.charger)), nil
	}
	if len(args) != 3 || args[1] != "powermode" {
		return nil, fmt.Errorf("unexpected command %v", args)
	}
	value, err := strconv.Atoi(args[2])
	if err != nil {
		return nil, err
	}
	if f.ignoreSet && args[0] == "-a" {
		return nil, nil
	}
	mode := Mode(value)
	switch args[0] {
	case "-a":
		f.battery, f.charger = mode, mode
	case "-b":
		f.battery = mode
	case "-c":
		f.charger = mode
	default:
		return nil, fmt.Errorf("unexpected power source %q", args[0])
	}
	return nil, nil
}

func TestDarwinControllerSetAndRestore(t *testing.T) {
	runner := &fakeRunner{battery: Low, charger: Automatic}
	controller := &darwinController{runner: runner}

	restore, err := controller.Set(High)
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if runner.battery != High || runner.charger != High {
		t.Fatalf("set modes = battery %v, charger %v", runner.battery, runner.charger)
	}
	if err := restore(); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if runner.battery != Low || runner.charger != Automatic {
		t.Fatalf("restored modes = battery %v, charger %v", runner.battery, runner.charger)
	}
}

func TestDarwinControllerRejectsUnavailableMode(t *testing.T) {
	runner := &fakeRunner{battery: Low, charger: Automatic, ignoreSet: true}
	controller := &darwinController{runner: runner}

	restore, err := controller.Set(High)
	if restore != nil {
		t.Fatal("Set returned a restore function for an unavailable mode")
	}
	if !strings.Contains(fmt.Sprint(err), ErrUnsupported.Error()) {
		t.Fatalf("Set error = %v, want %v", err, ErrUnsupported)
	}
	if runner.battery != Low || runner.charger != Automatic {
		t.Fatalf("modes changed after failed probe: battery %v, charger %v", runner.battery, runner.charger)
	}
}

func TestParseSettings(t *testing.T) {
	got, err := parseSettings("Battery Power:\n powermode 1\n sleep 10\nAC Power:\n powermode 2\n")
	if err != nil {
		t.Fatal(err)
	}
	if got.battery == nil || *got.battery != Low || got.charger == nil || *got.charger != High {
		t.Fatalf("parseSettings = %+v", got)
	}
}

func TestInspectParsers(t *testing.T) {
	if got := parsePowerSource("Now drawing from 'Battery Power'\n"); got != "battery" {
		t.Fatalf("parsePowerSource = %q, want battery", got)
	}
	if warnings := parseThermalWarnings("Note: No thermal warning level has been recorded\n"); len(warnings) != 0 {
		t.Fatalf("no-warning output produced %v", warnings)
	}
	warnings := parseThermalWarnings("CPU Power notify state 1\nPerformance Warning Level: 2\n")
	if len(warnings) != 2 || !warnings[0].Warning || !warnings[1].Warning {
		t.Fatalf("parseThermalWarnings = %+v", warnings)
	}

	topology, ok := parseTopology(strings.Join([]string{
		"hw.nperflevels: 2",
		"hw.perflevel0.name: Performance",
		"hw.perflevel0.logicalcpu: 12",
		"hw.perflevel1.name: Efficiency",
		"hw.perflevel1.logicalcpu: 4",
	}, "\n"))
	if !ok || topology != "CPU topology: 12 Performance cores, 4 Efficiency cores" {
		t.Fatalf("parseTopology = %q, %v", topology, ok)
	}
}

func TestRestoreCommands(t *testing.T) {
	runner := &fakeRunner{battery: Automatic, charger: Automatic}
	controller := &darwinController{runner: runner}
	battery, charger := Low, High
	if err := controller.restoreSettings(settings{battery: &battery, charger: &charger}); err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"-b", "powermode", "1"}, {"-c", "powermode", "2"}}
	if !reflect.DeepEqual(runner.commands, want) {
		t.Fatalf("restore commands = %v, want %v", runner.commands, want)
	}
}
