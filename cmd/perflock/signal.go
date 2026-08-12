// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"os"
	"os/signal"
	"syscall"
)

func ignoreSignals() {
	// Register handlers that deliberately drop SIGINT and SIGQUIT in the parent.
	// signal.Ignore is not suitable here: exec'd children inherit SIG_IGN and
	// would ignore the same signals instead of receiving the console interrupt.
	signal.Notify(make(chan os.Signal), os.Interrupt, syscall.SIGQUIT)
}
