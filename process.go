package reenvoy

import (
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

func init() {
	// Seed the default rand Source with current time to produce better random
	// numbers used with splay
	rand.Seed(time.Now().UnixNano())
}

var (
	// ErrMissingCommand is the error returned when no command is specified
	// to run.
	ErrMissingCommand = errors.New("missing command")

	// ExitCodeOK is the default OK exit code.
	ExitCodeOK = 0

	// ExitCodeError is the default error code returned when the process exits with
	// an error without a more specific code.
	ExitCodeError = 127
)

type Process struct {
	sync.RWMutex

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

	// exec is the actual child process under management.
	exec *exec.Cmd
	// exitCh is the channel where the processes exit will be returned.
	exitCh chan int

	// Splay is the maximum random amount of time to wait before sending signals.
	// This option helps reduce the thundering herd problem by effectively
	// sleeping for a random amount of time before sending the signal. This
	// prevents multiple processes from all signaling at the same time. This value
	// may be zero (which disables the splay entirely).
	Splay time.Duration

	// KillSignal is the signal to send to gracefully kill this process. This
	// value may be nil.
	KillSignal os.Signal

	// KillTimeout is the amount of time to wait for the process to gracefully
	// terminate before force-killing.
	KillTimeout time.Duration

	// stopLock is the mutex to lock when stopping. stopCh is the circuit breaker
	// to force-terminate any waiting splays to kill the process now. stopped is
	// a boolean that tells us if we have previously been stopped.
	stopLock sync.RWMutex
	stopped  bool
	stopCh   chan struct{}

	Stdin  io.Reader
	Stdout io.Writer
	StdErr io.Writer
}

// NewProc creates a new child process for management with high-level APIs for
// sending signals to the child process, restarting the child process, and
// gracefully terminating the child process.
func NewProc() (*Process, error) {
	p := new(Process)
	p.stopCh = make(chan struct{}, 1)
	return p, nil
}

// Start starts and begins execution of the child process. A buffered channel
// is returned which is where the command's exit code will be returned upon
// exit. Any errors that occur prior to starting the command will be returned
// as the second error argument, but any errors returned by the command after
// execution will be returned as a non-zero value over the exit code channel.
func (r *Process) Start() error {
	log.Printf("[INFO] spawning: %s", r.Command)
	r.Lock()
	defer r.Unlock()
	return r.start()
}

// Restart send the reload signal to the process and does not wait for a response
// If no reload signal was provided, the process is restarted and
// replaces the process attached to this Child
func (r *Process) Restart() error {

	if r.ReloadSignal == nil {
		log.Println("[INFO] restarting process")

		// Take a full lock because sart is going to replace the process. We also
		// want to make sure that no other routines attempt to send reload signal during
		// this transition
		r.Lock()
		defer r.Unlock()

		r.kill()
		log.Println("[INFO] kill old process")

		log.Println("[INFO] start new process")
		return r.start()
	}

	log.Println("[INFO] reloadinig process")

	// We only need read lock here because neither the process nor the exit
	// channel are changging
	r.RLock()
	defer r.RUnlock()

	return r.reload()
}

func (r *Process) start() error {
	cmd := exec.Command(r.Command, r.Args...)
	cmd.Stdin = r.Stdin
	cmd.Stderr = r.StdErr
	cmd.Stdout = r.Stdout
	cmd.Env = r.Env

	if err := cmd.Start(); err != nil {
		return err
	}

	r.exec = cmd

	// Create a new exitCh so that previously invoked commands (if any) don't
	// cause us to exit, and start a goroutine to wait for that process to end.
	exitCh := make(chan int, 1)
	go func() {
		var code int
		err := cmd.Wait()
		if err == nil {
			code = ExitCodeOK
		} else {
			code = ExitCodeError
			if exiterr, ok := err.(*exec.ExitError); ok {
				if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
					code = status.ExitStatus()
				}
			}
		}

		// If the child is in the process of killing, do not send a response back
		// down the exit channel.
		if r.stopped {
			return
		}

		select {
		case <-r.stopCh:
		case exitCh <- code:
		}
	}()

	r.exitCh = exitCh
	r.stopCh = make(chan struct{}, 1)

	// If a timeout was given, start the timer to wait for the child to exit
	if r.Timeout != 0 {
		select {
		case code := <-exitCh:
			if code != 0 {
				return fmt.Errorf(
					"command exited with a non-zero exit status:\n"+
						"\n"+
						"    %s\n"+
						"\n"+
						"This is assumed to be a failure. Please ensure the command\n"+
						"exits with a zero exit status.",
					r.Command,
				)
			}
		case <-time.After(r.Timeout):
			// Force-kill the process
			if r.exec != nil && r.exec.Process != nil {
				r.exec.Process.Kill()
			}

			return fmt.Errorf(
				"command did not exit within %q:\n"+
					"\n"+
					"    %s\n"+
					"\n"+
					"Commands must exit in a timely manner in order for processing to\n"+
					"continue. Consider using a process supervisor or utilizing the\n"+
					"built-in exec mode instead.",
				r.Timeout,
				r.Command,
			)
		}
	}
	return nil
}

func (r *Process) reload() error {
	select {
	case <-r.stopCh:
	case <-r.randomSplay():
	}

	return r.signal(r.ReloadSignal)
}

//GetPID return pid current process
func (r *Process) GetPID() PID {
	if !r.running() {
		return 0
	}

	return PID(r.Pid())
}

// Pid return pid of current process
func (r *Process) Pid() int {
	if !r.running() {
		return 0
	}

	return r.exec.Process.Pid
}

//  check if we already have running process
func (r *Process) running() bool {
	return r.exec != nil && r.exec.Process != nil
}

// Kill sends the kill signal to process and waits for successful termination.
// If no kill signal is defined, the process is killed with the most aggressive kill signal.
// If the process does not gracefully stop within the provided KillTimeout, the process is force-killed.
// If a splay was provided, this function will sleep for a random period of time between 0 and
// the provided splay value to reduce the thundering herd problem. This function
// does not return any errors because it guarantees the process will be dead by
// the return of the function call.
func (r *Process) Kill() {
	log.Printf("[INFO] killing process")
	r.Lock()
	defer r.Unlock()
	r.kill()
}

func (r *Process) kill() {
	if !r.running() {
		return
	}

	exited := false
	process := r.exec.Process

	if r.exec.ProcessState == nil {
		select {
		case <-r.stopCh:
		case <-r.randomSplay():
		}
	} else {
		log.Printf("[DEBUG] (runner) Kill() called but process dead; not waiting for splay.")
	}

	if r.KillSignal != nil {
		if err := process.Signal(r.KillSignal); err == nil {
			// Wait a few seconds for it to exit
			killCh := make(chan struct{}, 1)
			go func() {
				defer close(killCh)
				process.Wait()
			}()

			select {
			case <-r.stopCh:
			case <-killCh:
				exited = true
			case <-time.After(r.KillTimeout):
			}
		}
	}

	if !exited {
		process.Kill()
	}

	r.exec = nil
}

// Stop behavaes almost indetical to Kill except it suppresses feature process
// from bieng stared by this child and prevents the kiling of the child
// process from sending its value backup the exit channel. This is usefull when dong
// graceful sthudown of the application
func (r *Process) Stop() {
	log.Printf("[INFO] stopped process")

	r.stopLock.Lock()
	defer r.stopLock.Unlock()

	if r.stopped {
		log.Println("[WARN] process already stopped")
		return
	}

	r.kill()
	close(r.stopCh)
	r.stopped = true
}

func (r *Process) randomSplay() <-chan time.Time {
	if r.Splay == 0 {
		return time.After(0)
	}

	ns := r.Splay.Nanoseconds()
	offset := rand.Int63n(ns)
	t := time.Duration(offset)

	log.Printf("[DEBUG] (child) waiting %.2fs for random splay", t.Seconds())

	return time.After(t)
}

// ExitCh return the current exit channel for this process. this channel may change if the process is restarted, so implementers must
// not cache this value.
func (r *Process) ExitCh() <-chan int {
	r.RLock()
	defer r.RUnlock()
	return r.exitCh
}

// Signal sends a signal to the Process, returning any errors that accur.
// Sending Interrupt on Windows is not implemented.
func (r *Process) Signal(s os.Signal) error {
	log.Printf("[INFO] receiving signal %q", s.String())
	r.RLock()
	defer r.RLock()
	return r.signal(s)
}

func (r *Process) signal(s os.Signal) error {
	if !r.running() {
		return nil
	}

	return r.exec.Process.Signal(s)
}
