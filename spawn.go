package reenvoy

import (
	"errors"
	"io"
	"os"
	"syscall"
	"time"

	"github.com/evo3cx/merror"
)

//SpawnOptions spawn child options
type SpawnOptions struct {
	// ErrCh and DoneCh are channels where errors and finish notifications occur.
	ErrCh  chan error
	DoneCh chan struct{}

	// Command is the name of the command to execute. Args are the list of
	// arguments to pass when starting the command.
	Command string
	Args    []string

	// Env specifies the environment of the process.
	// Each entry is of the form "key=value".
	// If Env is nil, the new process uses the current process's
	// environment.
	// If Env contains duplicate environment keys, only the last
	// value in the slice for each duplicate key is used.
	Env []string

	// Timeout is the maximum amount of time to allow the command to execute. If
	// set to 0, the command is permitted to run infinitely.
	Timeout time.Duration

	// ReloadSignal is the signal to send to reload this process. This value may
	// be nil.
	ReloadSignal os.Signal

	// Splay is the maximum random amount of time to wait before sending signals.
	// This option helps reduce the thundering herd problem by effectively
	// sleeping for a random amount of time before sending the signal. This
	// prevents multiple processes from all signaling at the same time. This value
	// may be zero (which disables the splay entirely).
	// Splay time.Duration

	// KillSignal is the signal to send to gracefully kill this process. This
	// value may be nil.
	KillSignal os.Signal

	// KillTimeout is the amount of time to wait for the process to gracefully
	// terminate before force-killing.
	KillTimeout time.Duration

	Stdin  io.Reader
	Stdout io.Writer
	StdErr io.Writer
}

//SpawnProcess spawn new process and return instance of process
func SpawnProcess(opt SpawnOptions) (*Process, error) {
	if opt.Command == "" {
		return nil, errors.New("Command cannot empty")
	}

	opt = defaultOptions(opt)

	p := &Process{
		Command:      opt.Command,
		Args:         opt.Args,
		Env:          opt.Env,
		Timeout:      opt.Timeout,
		ReloadSignal: opt.ReloadSignal,
		KillSignal:   opt.KillSignal,
		KillTimeout:  opt.KillTimeout,
		// Splay:        opt.Splay,
		Stdin:  opt.Stdin,
		Stdout: opt.Stdout,
		StdErr: opt.StdErr,
	}

	if err := p.Start(); err != nil {
		return nil, merror.AppError(err, "spawn spawn child failed")
	}

	return p, nil
}

func defaultOptions(opt SpawnOptions) SpawnOptions {
	if opt.Command == "" {
		opt.Command = "envoy"
	}

	if opt.KillSignal == nil {
		opt.KillSignal = os.Kill
	}

	if opt.ReloadSignal == nil {
		opt.ReloadSignal = syscall.SIGHUP
	}

	if opt.KillTimeout.Nanoseconds() < 0 {
		opt.KillTimeout = 5 * time.Second
	}

	return opt
}
