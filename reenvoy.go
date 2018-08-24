package reenvoy

import (
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/evo3cx/merror"
)

// PID process identification number in linux
type PID int

type Child interface {
	Restart() error
	Stop()
	Kill()
	Start() error
	GetPID() PID
}

type reenvoy struct {
	restartEpoch int
}

// TermWaitSeconds The number of seconds to wait for children to gracefully exit
// after propagating SIGTERM before force to killing children.
const TermWaitSeconds = 30

// restartEpoch is amount of time envoy already restart and passed to `--restart-epoch` option.
var (
	restartEpoch int
	listChild    []Child
)

func (re *reenvoy) Start(c Child) error {
	if err := c.Start(); err != nil {
		return merror.AppError(err, "start child failed")
	}

	listChild = append(listChild, c)
	return nil
}

func (re *reenvoy) Stop() error {
	return nil
}

func (re *reenvoy) Reload() error {
	return nil
}

// StopAllChildren stop iterate through all known child processes, send a TERM signal to each of them.
func StopAllChildren() {
	for _, child := range listChild {
		log.Println("[INFO] Stopped childred with pid", child.GetPID())
		child.Stop()
	}

	log.Println("all children exited cleanly")
}

// ForceKillAllChildren iterate through all known child processes and force kill them. Typically
// StopAllChildren() should be attempted first to give child processes an
// opportunity to clean up state before exiting
func ForceKillAllChildren() {
	for _, child := range listChild {
		log.Println("[INFO] Kill childred with pid", child.GetPID())
		child.Kill()
	}

	log.Println("all children exited cleanly")
}

func Start(opt SpawnOptions) (*Reenvoy, error) {
	r := &Reenvoy{
		Options: opt,
	}

	if err := r.spawn(opt); err != nil {
		return nil, err
	}

	sigterm := make(chan os.Signal, 1)
	sighub := make(chan os.Signal, 1)

	// register our signal to receive notification
	signal.Notify(sigterm, syscall.SIGINT, syscall.SIGTERM)
	signal.Notify(sighub, syscall.SIGHUP)

	go r.Sigterm(sigterm)
	go r.Sighup(sighub)

	return r, nil
}

type Handler interface{}

type Reenvoy struct {
	childs  []Child
	Options SpawnOptions
}

// spawn a new child process and keeps track of its PID.
func (r *Reenvoy) spawn(opt SpawnOptions) error {
	os.Setenv("RESTART_EPOCH", strconv.Itoa(restartEpoch))
	log.Println("[INFO] spawn a new child process at epoch", restartEpoch)

	restartEpoch++
	child, err := SpawnProcess(opt)
	if err != nil {
		return err
	}

	if len(r.childs) == 0 {
		r.childs = []Child{child}
	} else {
		log.Printf("[INFO] spawn new child process with pid %v \n", child.GetPID())
		r.childs = append(r.childs, child)
	}
	return nil
}

// Sigterm handler for stop all the children process
func (r *Reenvoy) Sigterm(signal chan os.Signal) {
	sig := <-signal
	log.Println("[INFO] recieve signal", sig)
	StopAllChildren()
}

// Sighup Handler when receive signal SIGUP.
// This signal is used to cause the restarter to fork and exec a new child.
func (r *Reenvoy) Sighup(signal chan os.Signal) {
	sig := <-signal
	log.Println("[INFO] recieve signal", sig)
}

func (r *Reenvoy) Sigchild() {
	log.Println("[INFO] recieve signal", syscall.SIGCHLD)
}

func (r *Reenvoy) Sigusr1() {
	log.Println("[INFO] recieve signal", syscall.SIGUSR1)
}
