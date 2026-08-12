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

const sysctlPath = "/usr/sbin/sysctl"

// Inspect reports macOS power and performance conditions that can make
// benchmark results less stable.
func Inspect() []Message {
	var messages []Message

	powerOutput, err := exec.Command(pmsetPath, "-g", "ps").CombinedOutput()
	powerSource := ""
	if err == nil {
		powerSource = parsePowerSource(string(powerOutput))
		if powerSource == "battery" {
			messages = append(messages, Message{Warning: true, Text: "running on battery power"})
		}
	}

	controller := &darwinController{runner: systemRunner{}}
	if settings, err := controller.readSettings(); err == nil {
		var active *Mode
		switch powerSource {
		case "battery":
			active = settings.battery
		case "charger":
			active = settings.charger
		}
		if active != nil && *active == Low {
			messages = append(messages, Message{Warning: true, Text: "Low Power Mode is active"})
		}
	}

	if thermalOutput, err := exec.Command(pmsetPath, "-g", "therm").CombinedOutput(); err == nil {
		messages = append(messages, parseThermalWarnings(string(thermalOutput))...)
	}

	if topologyOutput, err := exec.Command(sysctlPath,
		"hw.nperflevels",
		"hw.perflevel0.name", "hw.perflevel0.logicalcpu",
		"hw.perflevel1.name", "hw.perflevel1.logicalcpu",
	).CombinedOutput(); err == nil {
		if topology, ok := parseTopology(string(topologyOutput)); ok {
			messages = append(messages, Message{Text: topology})
		}
	}
	return messages
}

func parsePowerSource(output string) string {
	switch {
	case strings.Contains(output, "Now drawing from 'AC Power'"):
		return "charger"
	case strings.Contains(output, "Now drawing from 'Battery Power'"):
		return "battery"
	default:
		return ""
	}
}

func parseThermalWarnings(output string) []Message {
	var messages []Message
	for _, rawLine := range strings.Split(output, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "Note: No ") {
			continue
		}
		lower := strings.ToLower(line)
		if strings.Contains(lower, "thermal") || strings.Contains(lower, "performance") || strings.Contains(lower, "cpu power") {
			messages = append(messages, Message{Warning: true, Text: "pmset reports " + line})
		}
	}
	return messages
}

func parseTopology(output string) (string, bool) {
	values := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if ok {
			values[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	levels, err := strconv.Atoi(values["hw.nperflevels"])
	if err != nil || levels < 2 {
		return "", false
	}

	parts := make([]string, 0, 2)
	for level := 0; level < 2; level++ {
		prefix := fmt.Sprintf("hw.perflevel%d.", level)
		name := values[prefix+"name"]
		cores, err := strconv.Atoi(values[prefix+"logicalcpu"])
		if name == "" || err != nil {
			return "", false
		}
		parts = append(parts, fmt.Sprintf("%d %s cores", cores, name))
	}
	return "CPU topology: " + strings.Join(parts, ", "), true
}
