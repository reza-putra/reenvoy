package reenvoy

import (
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"sync"
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

type Processor interface {
	Serve()
	Stop()
	Start()
	ProcessState() *os.ProcessState
	GetPID() PID
	Complete() bool
}

type Process struct {
	// Command is the name of the command to execute. Args are the list of
	// arguments to pass when starting the command.
	command string
	args    []string

	// Env specifies the environment of the process.
	// Each entry is of the form "key=value".
	Env []string

	// exec is the actual child process under management.
	exec *exec.Cmd

	Stdout io.Writer
	StdErr io.Writer

	// stopLock is the mutex to lock when stopping. stopCh is the circuit breaker
	// to force-terminate any waiting splays to kill the process now. stopped is
	// a boolean that tells us if we have previously been stopped.
	stopLock           sync.RWMutex
	isFinished         bool
	useDockerContainer bool
}

// NewProcess creates a new child process
func NewProcess(opt Options) Processor {
	p := new(Process)
	p.command, p.args = opt.getCommand()
	p.Stdout = opt.Stdout
	p.StdErr = opt.StdErr
	p.Env = opt.Env

	return p
}

func (p *Process) Serve() {
	if p.isFinished {
		return
	}

	p.start()
}

func (p *Process) Complete() bool {
	return p.isFinished
}

// Start starts and begins execution of the child process.
func (r *Process) Start() {
	r.start()
}

func (r *Process) start() {

	cmd := exec.Command(r.command, r.args...)
	cmd.Stderr = os.Stdout
	cmd.Stdout = os.Stdout
	cmd.Env = r.Env

	if err := cmd.Start(); err != nil {
		log.Printf("[Process] Start %s err: %s \n", r.command, err)
		return
	}

	if err := cmd.Wait(); err != nil {
		r.isFinished = true
		log.Printf("[Process] Fail %s err: %s \n", r.command, err)
		return
	}

	log.Printf("[Process] Finished %s  \n", r.command)
	r.isFinished = true
	r.exec = cmd

	return
}

//GetPID return pid current process
func (r *Process) GetPID() PID {
	if !r.running() {
		return 0
	}

	return PID(r.exec.Process.Pid)
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
	r.kill()
}

func (r *Process) kill() {
	if !r.running() {
		return
	}

	if !r.isFinished {
		r.exec.Process.Kill()
		log.Println("[INFO] kill process ", r.GetPID())
	}

	r.exec = nil
}

// Stop behavaes almost indetical to Kill except it suppresses feature process
// from bieng stared by this child and prevents the kiling of the child
// process from sending its value backup the exit channel. This is usefull when dong
// graceful sthudown of the application
func (r *Process) Stop() {
	log.Printf("[INFO] stopped process")

	if r.isFinished {
		log.Println("[WARN] process already stopped")
		return
	}

	r.kill()
	r.isFinished = true
}

//ProcessState 	contains information about an exited process,
// available after a call to Wait or Run.
func (r *Process) ProcessState() *os.ProcessState {
	return r.exec.ProcessState
}

func (r *Process) signal(s os.Signal) error {
	log.Printf("[INFO] receiving signal %q", s.String())
	if !r.running() {
		return nil
	}

	return r.exec.Process.Signal(s)
}

func commandWithDocker(opt Options) (command string, args []string) {
	command = "docker"
	args = []string{
		"run",
		"--network",
		"host",
		"-v",
		fmt.Sprintf("%s:/testdata", opt.ConfigPath),
		envoyDockerImage,
		"envoy",
		"--mode",
		"serve",
		"--restart-epoch",
		strconv.Itoa(opt.RestartEpoch),
		"--drain-time-s",
		fmt.Sprintf("%v", opt.DrainTimes.Seconds()),
		"--parent-shutdown-time-s",
		fmt.Sprintf("%v", opt.ParentShutdownTimes.Seconds()),
		"-c",
		"/testdata/envoy.yaml",
	}

	return
}

func commandWithEnvoy(opt Options) (command string, args []string) {
	command = "envoy"
	args = []string{
		"--mode",
		"serve",
		"--restart-epoch",
		strconv.Itoa(opt.RestartEpoch),
		"--drain-time-s",
		fmt.Sprintf("%v", opt.DrainTimes.Seconds()),
		"--parent-shutdown-time-s",
		fmt.Sprintf("%v", opt.ParentShutdownTimes.Seconds()),
		"-c",
		fmt.Sprintf("%s/envoy.yaml", opt.ConfigPath),
	}

	return
}
