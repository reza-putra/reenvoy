package reenvoy

import (
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
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

// ReEnvoy will be hot restarted for config changes and binary updates
type ReEnvoy interface {
	Restart() error
	StopAllChildren()
	ForceKillAllChildren()
}

func Start(opt SpawnOptions) (ReEnvoy, error) {
	opt = defaultOptions(opt)

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

type Reenvoy struct {
	childs       []Child
	Options      SpawnOptions
	restartEpoch int
}

// spawn a new child process and keeps track of its PID.
func (r *Reenvoy) spawn(opt SpawnOptions) error {
	os.Setenv("RESTART_EPOCH", strconv.Itoa(r.restartEpoch))
	log.Println("[INFO] spawn a new child process at epoch", r.restartEpoch)

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

func (r *Reenvoy) Restart() error {
	r.restartEpoch++
	return r.spawn(r.Options)
}

// Sigterm handler for stop all the children process
func (r *Reenvoy) Sigterm(signal chan os.Signal) {
	sig := <-signal
	log.Println("[INFO] recieve signal", sig)
	r.StopAllChildren()
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

// StopAllChildren stop iterate through all known child processes, send a TERM signal to each of them.
func (r *Reenvoy) StopAllChildren() {
	for _, child := range r.childs {
		log.Println("[INFO] Stopped childred with pid", child.GetPID())
		child.Stop()
	}

	log.Println("all children exited cleanly")
}

// ForceKillAllChildren iterate through all known child processes and force kill them. Typically
// StopAllChildren() should be attempted first to give child processes an
// opportunity to clean up state before exiting
func (r *Reenvoy) ForceKillAllChildren() {
	for _, child := range r.childs {
		log.Println("[INFO] Kill childred with pid", child.GetPID())
		child.Kill()
	}

	log.Println("all children exited cleanly")
}
