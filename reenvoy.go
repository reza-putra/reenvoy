package reenvoy

import (
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/thejerf/suture"
)

// PID process identification number in linux
type PID int

// ReEnvoy will be hot restarted for config changes and binary updates
type ReEnvoy interface {
	Start() error
	StopAllChildren()
	Restart(epoch int)
}

// Reenvoy is a supervisor whose handle all child process
type Reenvoy struct {
	supervisor    *suture.Supervisor
	options       Options
	tokenServices []suture.ServiceToken
	process       []Processor
}

//New return intance of ReEnvoy
func New(opt Options) ReEnvoy {
	opt = defaultOptions(opt)
	r := &Reenvoy{
		options:    opt,
		supervisor: suture.New("reenvoy", opt.Spec),
	}

	return r
}

func (r *Reenvoy) Restart(counterEpoch int) {

	r.options.RestartEpoch = counterEpoch
	process := NewProcess(r.options)
	r.Add(process)
	return
}

//Start start new process with default value
func (r *Reenvoy) Start() error {
	if r.supervisor == nil {
		return errors.New("[ReEnvoy] supervisor cannot nit")
	}

	fmt.Println("[ReEnvoy] Start Supervisor")
	r.supervisor.ServeBackground()
	return nil
}

func (r *Reenvoy) Add(service suture.Service) {
	token := r.supervisor.Add(service)
	r.tokenServices = append(r.tokenServices, token)
}

// Sigterm handler for stop all the children process
func (r *Reenvoy) Sigterm(signal chan os.Signal) {
	sig := <-signal
	log.Println("[INFO] recieve signal", sig)
	r.StopAllChildren()
}

// StopAllChildren stop iterate through all known child processes, send a TERM signal to each of them.
func (r *Reenvoy) StopAllChildren() {
	for _, token := range r.tokenServices {
		r.supervisor.Remove(token)
	}

	r.supervisor.Stop()
}
