// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build windows

package main

import (
	"net"

	"golang.org/x/sys/windows/svc"
)

const windowsServiceName = "perflock"

type perflockService struct {
	addr string
}

func runAsService(addr string) (bool, error) {
	isService, err := svc.IsWindowsService()
	if err != nil {
		return false, err
	}
	if !isService {
		return false, nil
	}
	return true, svc.Run(windowsServiceName, &perflockService{addr: addr})
}

func (s *perflockService) Execute(_ []string, requests <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	const accepted = svc.AcceptStop | svc.AcceptShutdown

	changes <- svc.Status{State: svc.StartPending}
	listener, err := newListener(s.addr)
	if err != nil {
		return false, 1
	}
	defer listener.Close()

	serveResult := make(chan error, 1)
	go func() {
		serveResult <- serveListener(listener)
	}()
	changes <- svc.Status{State: svc.Running, Accepts: accepted}

	for {
		select {
		case err := <-serveResult:
			changes <- svc.Status{State: svc.StopPending}
			if err != nil {
				return false, 1
			}
			return false, 0

		case request := <-requests:
			switch request.Cmd {
			case svc.Interrogate:
				changes <- request.CurrentStatus
			case svc.Stop, svc.Shutdown:
				changes <- svc.Status{State: svc.StopPending}
				return stopService(listener, serveResult)
			}
		}
	}
}

func stopService(listener net.Listener, serveResult <-chan error) (bool, uint32) {
	if err := listener.Close(); err != nil {
		return false, 1
	}
	if err := <-serveResult; err != nil {
		return false, 1
	}
	return false, 0
}
