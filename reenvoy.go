package reenvoy

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

// PID process identification number in linux
type PID int

// ReEnvoy will be hot restarted for config changes and binary updates
type ReEnvoy interface {
	Restart() error
	StopAllChildren()
	ForceKillAllChildren()
	IsRunning() bool
}

//Start start new process with default value
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

//New return intance of ReEnvoy and default value without run a process
func New(opt SpawnOptions) ReEnvoy {
	opt = defaultOptions(opt)
	r := &Reenvoy{
		Options: opt,
	}

	sigterm := make(chan os.Signal, 1)
	sighub := make(chan os.Signal, 1)

	// register our signal to receive notification
	signal.Notify(sigterm, syscall.SIGINT, syscall.SIGTERM)
	signal.Notify(sighub, syscall.SIGHUP)

	go r.Sigterm(sigterm)
	go r.Sighup(sighub)

	return r
}

type Reenvoy struct {
	currentProcess Child
	parentProcess  Child
	Options        SpawnOptions
	restartEpoch   int
}

func (r *Reenvoy) IsRunning() bool {
	state := r.currentProcess.ProcessState()
	return !state.Exited()
}

// spawn a new child process and keeps track of its PID.
func (r *Reenvoy) spawn(opt SpawnOptions) error {
	process, err := SpawnProcess(opt, r.restartEpoch)
	if err != nil {
		return err
	}

	log.Printf("[INFO] spawn new process with pid %v restart epoc \n", process.GetPID(), r.restartEpoch)
	r.parentProcess = r.currentProcess
	r.currentProcess = process

	return nil
}

func (r *Reenvoy) Restart() error {
	if err := r.spawn(r.Options); err != nil {
		return err
	}
	r.restartEpoch++

	return nil
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
	if r.currentProcess != nil {
		log.Println("[INFO] Stopped current process with pid", r.currentProcess.GetPID())
		r.currentProcess.Stop()
	}

	if r.parentProcess != nil {
		log.Println("[INFO] Stopped parent process with pid", r.parentProcess.GetPID())
		r.parentProcess.Stop()
	}

}

// ForceKillAllChildren force kill current & parent process.
func (r *Reenvoy) ForceKillAllChildren() {
	if r.currentProcess != nil {
		log.Println("[INFO] Kill current process with pid", r.currentProcess.GetPID())
		r.currentProcess.Kill()
	}

	if r.parentProcess != nil {
		log.Println("[INFO] Kill parent process with pid", r.parentProcess.GetPID())
		r.parentProcess.Kill()
	}
}
