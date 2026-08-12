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
	"os"
	"os/user"
	"runtime"
	"time"

	"github.com/aclements/perflock/internal/perfctl"
	"inet.af/peercred"
)

var theLock PerfLock

func doDaemon(path string) {
	// TODO: Don't start if another daemon is already running.

	// Linux supports an abstract namespace for UNIX domain sockets (see unix(7)).
	// These do not involve the filesystem, and are world-connectable.
	isAbstractSocket := runtime.GOOS == "linux" && len(path) > 1 && path[0] == '@'
	if !isAbstractSocket {
		os.Remove(path)
	}
	l, err := net.Listen("unix", path)
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()
	if !isAbstractSocket {
		err = os.Chmod(path, 0777)
		if err != nil {
			log.Fatal(err)
		}
	}

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

	restoreGovernor func() error
}

func NewServer(c net.Conn) *Server {
	return &Server{c: c}
}

func (s *Server) Serve() {
	// Drop any held locks if we exit for any reason.
	defer s.drop()

	// Get connection credentials.
	cred, err := peercred.Get(s.c)
	if err != nil {
		log.Print("reading credentials: ", err)
		return
	}

	s.userName = "???"
	if uid, ok := cred.UserID(); ok {
		if u, err := user.LookupId(uid); err == nil {
			s.userName = u.Username
		}
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
				err := s.setGovernor(action.Percent)
				errString := ""
				if err != nil {
					errString = err.Error()
				}
				if err := gw.Encode(errString); err != nil {
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
	// Restore the CPU governor before releasing the lock.
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
