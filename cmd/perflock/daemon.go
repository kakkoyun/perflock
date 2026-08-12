// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"github.com/aclements/perflock/internal/ipc"
	"github.com/aclements/perflock/internal/perfctl"
	"github.com/aclements/perflock/internal/powermode"
)

var theLock PerfLock

func doDaemon(path string) {
	l, err := ipc.Listen(path)
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	// Receive connections.
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatal(err)
		}

		go func(c net.Conn) {
			defer c.Close()
			NewServer(c).Serve()
		}(conn)
	}
}

type Server struct {
	c        net.Conn
	userName string

	locker    *Locker
	acquiring bool

	restoreGovernor  func() error
	restorePowerMode func() error
}

func NewServer(c net.Conn) *Server {
	return &Server{c: c}
}

func (s *Server) Serve() {
	// Drop any held locks if we exit for any reason.
	defer s.drop()

	// Get connection credentials for display. Endpoint permissions, rather than
	// this label, are the access-control boundary.
	s.userName = "???"
	if userName, ok := ipc.PeerUser(s.c); ok {
		s.userName = userName
	}

	// Receive incoming actions. We do this in a goroutine so the
	// main handler can select on EOF or lock acquisition.
	actions := make(chan PerfLockAction)
	go func() {
		gr := gob.NewDecoder(s.c)
		for {
			var msg PerfLockAction
			err := gr.Decode(&msg)
			if err != nil {
				if err != io.EOF {
					log.Print(err)
				}
				close(actions)
				return
			}
			actions <- msg
		}
	}()

	// Process incoming actions.
	var acquireC <-chan bool
	gw := gob.NewEncoder(s.c)
	for {
		select {
		case action, ok := <-actions:
			if !ok {
				// Connection closed.
				return
			}
			if s.acquiring {
				log.Printf("protocol error: message while acquiring")
				return
			}
			switch action := action.Action.(type) {
			case ActionAcquire:
				if s.locker != nil {
					log.Printf("protocol error: acquiring lock twice")
					return
				}
				msg := fmt.Sprintf("%s\t%s\t%s", s.userName, time.Now().Format(time.Stamp), action.Msg)
				if action.Shared {
					msg += " [shared]"
				}
				s.locker = theLock.Enqueue(action.Shared, action.NonBlocking, msg)
				if s.locker != nil {
					// Enqueued. Wait for acquire.
					s.acquiring = true
					acquireC = s.locker.C
				} else {
					// Non-blocking acquire failed.
					if err := gw.Encode(false); err != nil {
						log.Print(err)
						return
					}
				}

			case ActionList:
				list := theLock.Queue()
				if err := gw.Encode(list); err != nil {
					log.Print(err)
					return
				}

			case ActionSetGovernor:
				if s.locker == nil || s.locker.shared {
					log.Printf("protocol error: setting governor without exclusive lock")
					return
				}
				if s.restoreGovernor != nil {
					log.Printf("protocol error: setting governor twice")
					return
				}
				if err := gw.Encode(errorString(s.setGovernor(action.Percent))); err != nil {
					log.Print(err)
					return
				}

			case ActionSetPowerMode:
				if s.locker == nil || s.locker.shared {
					log.Printf("protocol error: setting power mode without exclusive lock")
					return
				}
				mode := powermode.Mode(action.Mode)
				if err := gw.Encode(errorString(s.setPowerMode(mode))); err != nil {
					log.Print(err)
					return
				}

			default:
				log.Printf("unknown message")
				return
			}

		case <-acquireC:
			// Lock acquired.
			s.acquiring, acquireC = false, nil
			if err := gw.Encode(true); err != nil {
				log.Print(err)
				return
			}
		}
	}
}

func (s *Server) drop() {
	// Restore performance settings before releasing the lock.
	if s.restorePowerMode != nil {
		if err := s.restorePowerMode(); err != nil {
			log.Print(err)
		}
		s.restorePowerMode = nil
	}
	if s.restoreGovernor != nil {
		if err := s.restoreGovernor(); err != nil {
			log.Print(err)
		}
		s.restoreGovernor = nil
	}
	// Release the lock.
	if s.locker != nil {
		theLock.Dequeue(s.locker)
		s.locker = nil
	}
}

func (s *Server) setGovernor(percent int) error {
	controller, err := perfctl.Open()
	if err != nil {
		return err
	}
	restore, err := controller.Pin(percent)
	if err != nil {
		return err
	}
	s.restoreGovernor = restore
	return nil
}

func (s *Server) setPowerMode(mode powermode.Mode) error {
	controller, err := powermode.Open()
	if err != nil {
		return err
	}
	restore, err := controller.Set(mode)
	if err != nil {
		return err
	}
	s.restorePowerMode = restore
	return nil
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
