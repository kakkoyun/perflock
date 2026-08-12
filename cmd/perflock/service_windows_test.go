// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build windows

package main

import (
	"errors"
	"net"
	"testing"
	"time"

	"golang.org/x/sys/windows/svc"
)

func TestWindowsServiceStopLifecycle(t *testing.T) {
	addr := socketName(t)
	service := &perflockService{addr: addr}
	requests := make(chan svc.ChangeRequest, 1)
	changes := make(chan svc.Status, 4)
	result := make(chan uint32, 1)
	go func() {
		_, exitCode := service.Execute(nil, requests, changes)
		result <- exitCode
	}()

	if status := receiveServiceStatus(t, changes); status.State != svc.StartPending {
		t.Fatalf("first status = %+v, want StartPending", status)
	}
	running := receiveServiceStatus(t, changes)
	if running.State != svc.Running || running.Accepts != svc.AcceptStop|svc.AcceptShutdown {
		t.Fatalf("running status = %+v", running)
	}

	requests <- svc.ChangeRequest{Cmd: svc.Interrogate, CurrentStatus: running}
	if status := receiveServiceStatus(t, changes); status != running {
		t.Fatalf("interrogate status = %+v, want %+v", status, running)
	}

	requests <- svc.ChangeRequest{Cmd: svc.Stop}
	if status := receiveServiceStatus(t, changes); status.State != svc.StopPending {
		t.Fatalf("stop status = %+v, want StopPending", status)
	}
	select {
	case exitCode := <-result:
		if exitCode != 0 {
			t.Fatalf("service exit code = %d, want 0", exitCode)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("service did not stop")
	}
}

func TestStopServiceReportsCloseAndServeErrors(t *testing.T) {
	for _, test := range []struct {
		name     string
		listener net.Listener
		serveErr error
		wantCode uint32
	}{
		{name: "success", listener: &fakeServiceListener{}},
		{name: "close failure", listener: &fakeServiceListener{closeErr: errors.New("close failed")}, wantCode: 1},
		{name: "serve failure", listener: &fakeServiceListener{}, serveErr: errors.New("serve failed"), wantCode: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			serveResult := make(chan error, 1)
			serveResult <- test.serveErr
			_, exitCode := stopService(test.listener, serveResult)
			if exitCode != test.wantCode {
				t.Fatalf("exit code = %d, want %d", exitCode, test.wantCode)
			}
		})
	}
}

func receiveServiceStatus(t *testing.T, changes <-chan svc.Status) svc.Status {
	t.Helper()
	select {
	case status := <-changes:
		return status
	case <-time.After(5 * time.Second):
		t.Fatal("service status timed out")
		return svc.Status{}
	}
}

type fakeServiceListener struct {
	closeErr error
}

func (*fakeServiceListener) Accept() (net.Conn, error) { return nil, net.ErrClosed }
func (f *fakeServiceListener) Close() error            { return f.closeErr }
func (*fakeServiceListener) Addr() net.Addr            { return fakeServiceAddr("test") }

type fakeServiceAddr string

func (fakeServiceAddr) Network() string  { return "test" }
func (a fakeServiceAddr) String() string { return string(a) }
