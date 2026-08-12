// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build linux

package perfctl

import (
	"fmt"

	"github.com/aclements/perflock/internal/cpupower"
)

// SupportsPinning reports whether this platform has a CPU performance control
// implementation.
const SupportsPinning = true

type linuxController struct {
	domains []frequencyDomain
}

type frequencyDomain interface {
	AvailableRange() (min, max int, available []int)
	CurrentRange() (min, max int, err error)
	SetRange(min, max int) error
}

type linuxSettings struct {
	domain   frequencyDomain
	min, max int
}

// Open opens the host CPU performance controls.
func Open() (Controller, error) {
	domains, err := cpupower.Domains()
	if err != nil {
		return nil, fmt.Errorf("discover CPU frequency domains: %w", err)
	}
	if len(domains) == 0 {
		return nil, fmt.Errorf("discover CPU frequency domains: none found")
	}
	controller := &linuxController{domains: make([]frequencyDomain, len(domains))}
	for index, domain := range domains {
		controller.domains[index] = domain
	}
	return controller, nil
}

func (c *linuxController) Pin(percent int) (func() error, error) {
	if percent < 0 || percent > 100 {
		return nil, fmt.Errorf("CPU performance percentage %d is outside 0-100", percent)
	}
	old := make([]linuxSettings, 0, len(c.domains))
	for _, domain := range c.domains {
		min, max, err := domain.CurrentRange()
		if err != nil {
			return nil, fmt.Errorf("read current CPU frequency range: %w", err)
		}
		old = append(old, linuxSettings{domain: domain, min: min, max: max})
	}

	restore := func() error {
		var firstErr error
		for _, settings := range old {
			if err := settings.domain.SetRange(settings.min, settings.max); err != nil && firstErr == nil {
				firstErr = fmt.Errorf("restore CPU frequency range: %w", err)
			}
		}
		return firstErr
	}

	for _, domain := range c.domains {
		min, max, available := domain.AvailableRange()
		target := (max-min)*percent/100 + min
		if len(available) != 0 {
			target = nearest(target, available)
		}
		if err := domain.SetRange(target, target); err != nil {
			restoreErr := restore()
			if restoreErr != nil {
				return nil, fmt.Errorf("set CPU frequency range: %v; %w", err, restoreErr)
			}
			return nil, fmt.Errorf("set CPU frequency range: %w", err)
		}
	}
	return restore, nil
}

func nearest(target int, available []int) int {
	closest := available[0]
	for _, candidate := range available[1:] {
		if abs(target-candidate) < abs(target-closest) {
			closest = candidate
		}
	}
	return closest
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
