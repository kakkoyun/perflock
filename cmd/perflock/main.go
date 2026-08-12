// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command perflock is a simple locking wrapper for running benchmarks
// on shared hosts.
//
// The typical use of perflock is:
//
//	perflock [-shared] command...
//
// This will acquire a system-wide lock while running command.
//
// In exclusive mode (the default), perflock prevents any other
// perflock'd command from running. This should be used for running
// benchmarks that are sensitive to the environment. In the future,
// this may do other things to set up a better isolated benchmarking
// environment.
//
// In shared mode (with the -shared flag), perflock can run other
// shared-mode commands concurrently. This should be used for commands
// that would perturb benchmarks but aren't themselves benchmarks.
//
// For convenience, we recommend you create shell aliases for
// perflock:
//
//	alias pl=perflock
//	alias pls='perflock -shared'
//
// perflock depends on a locking daemon, which can be started with
// perflock -daemon.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"github.com/aclements/perflock/internal/ipc"
	"github.com/aclements/perflock/internal/perfctl"
	"github.com/aclements/perflock/internal/powermode"
)

type governorSetter interface {
	SetGovernor(percent int) error
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "  %s [flags] command...\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -list\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -daemon\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\n")
		flag.PrintDefaults()
	}
	flagDaemon := flag.Bool("daemon", false, "start perflock daemon")
	flagList := flag.Bool("list", false, "print current and pending commands")
	flagSocket := flag.String("socket", ipc.DefaultAddr, "connect to endpoint `path`")
	flagShared := flag.Bool("shared", false, "acquire lock in shared mode (default: exclusive mode)")
	flagGovernor := newGovernorFlag()
	flag.Var(flagGovernor, "governor", "set CPU frequency to `percent` between the min and max\n\twhile running command, or \"none\" for no adjustment")
	flagPowerMode := &powerModeFlag{mode: powermode.Automatic}
	if powermode.Supported {
		flag.Var(flagPowerMode, "power-mode", "set macOS system power mode to `auto`, `low`, or `high` while running command")
	}
	flag.Parse()

	if *flagDaemon {
		if flag.NArg() > 0 {
			flag.Usage()
			os.Exit(2)
		}
		doDaemon(*flagSocket)
		return
	}

	log.SetFlags(0)

	if *flagList {
		if flag.NArg() > 0 {
			flag.Usage()
			os.Exit(2)
		}
		c := NewClient(*flagSocket)
		list := c.List()
		for _, l := range list {
			fmt.Println(l)
		}
		return
	}

	cmd := flag.Args()
	if len(cmd) == 0 {
		flag.Usage()
		os.Exit(2)
	}
	for _, message := range powermode.Inspect() {
		prefix := "info: "
		if message.Warning {
			prefix = "warning: "
		}
		log.Print(prefix, message.Text)
	}
	c := NewClient(*flagSocket)
	if !c.Acquire(*flagShared, true, shellEscapeList(cmd)) {
		list := c.List()
		fmt.Fprintf(os.Stderr, "Waiting for lock...\n")
		for _, l := range list {
			fmt.Fprintln(os.Stderr, l)
		}
		c.Acquire(*flagShared, false, shellEscapeList(cmd))
	}
	warning, err := applyGovernor(c, *flagShared, flagGovernor)
	if err != nil {
		log.Fatal(err)
	}
	if warning != nil {
		log.Print("warning: ", warning)
	}
	if flagPowerMode.explicit {
		if *flagShared {
			log.Fatal("-power-mode requires an exclusive lock")
		}
		if err := c.SetPowerMode(int(flagPowerMode.mode)); err != nil {
			log.Fatal("setting power mode: ", err)
		}
	}
	ignoreSignals()
	run(cmd)
}

type governorFlag struct {
	percent  int
	explicit bool
}

func applyGovernor(client governorSetter, shared bool, setting *governorFlag) (warning, fatal error) {
	if shared || setting.percent < 0 {
		return nil, nil
	}
	if err := client.SetGovernor(setting.percent); err != nil {
		if setting.explicit {
			return nil, fmt.Errorf("setting CPU governor: %w", err)
		}
		return fmt.Errorf("unable to set CPU governor: %w", err), nil
	}
	return nil, nil
}

func newGovernorFlag() *governorFlag {
	percent := -1
	if perfctl.SupportsPinning {
		percent = 90
	}
	return &governorFlag{percent: percent}
}

func (f *governorFlag) String() string {
	if f.percent < 0 {
		return "none"
	}
	return fmt.Sprintf("%d%%", f.percent)
}

func (f *governorFlag) Set(v string) error {
	if v == "none" {
		f.percent = -1
	} else {
		m := regexp.MustCompile(`^([0-9]+)%$`).FindStringSubmatch(v)
		if m == nil {
			return fmt.Errorf("governor must be \"none\" or \"N%%\"")
		}
		percent, err := strconv.Atoi(m[1])
		if err != nil {
			return fmt.Errorf("invalid governor percentage %q: %w", m[1], err)
		}
		if percent > 100 {
			return fmt.Errorf("governor must be between 0%% and 100%%")
		}
		f.percent = percent
	}
	f.explicit = true
	return nil
}

// run executes args as a command and exits with the command's exit
// status.
func run(args []string) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	err := cmd.Run()
	switch err := err.(type) {
	case nil:
		os.Exit(0)
	case *exec.ExitError:
		status := err.Sys().(syscall.WaitStatus)
		if status.Exited() {
			os.Exit(status.ExitStatus())
		}
		log.Fatal(err)
	default:
		log.Fatal(err)
	}
}

// shellEscape escapes a single shell token.
func shellEscape(x string) string {
	if len(x) == 0 {
		return "''"
	}
	for _, r := range x {
		if 'a' <= r && r <= 'z' || 'A' <= r && r <= 'Z' || '0' <= r && r <= '9' || strings.ContainsRune("@%_-+:,./", r) {
			continue
		}
		// Unsafe character.
		return "'" + strings.Replace(x, "'", "'\"'\"'", -1) + "'"
	}
	return x
}

// shellEscapeList escapes a list of shell tokens.
func shellEscapeList(xs []string) string {
	out := make([]string, len(xs))
	for i, x := range xs {
		out[i] = shellEscape(x)
	}
	return strings.Join(out, " ")
}
