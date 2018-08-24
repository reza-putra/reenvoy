package reenvoy

import (
	"bytes"
	"log"
	"os"
	"os/exec"
	"strconv"

	"github.com/evo3cx/merror"
)

type Reenvoy interface {
	Restart() error
	Stop() error
	Start() error
}

type reenvoy struct {
	restartEpoch int
	command      string
}

// TermWaitSeconds The number of seconds to wait for children to gracefully exit
// after propagating SIGTERM before force to killing children.
const TermWaitSeconds = 30

// restartEpoch is amount of time envoy already restart and passed to `--restart-epoch` option.
var restartEpoch = 0

// func New() Reenvoy {
// 	re := new(reenvoy)
// 	gotenv.Load()

// 	re.restartEpoch, _ = strconv.Atoi(os.Getenv("RESTART_EPOCH"))

// 	return re
// }

func (re *reenvoy) Start() error {
	var stdOut *bytes.Buffer

	cmd := exec.Command(re.command)
	cmd.Stderr = stdOut
	cmd.Stdout = stdOut
	if err := cmd.Run(); err != nil {
		return merror.AppError(err, "run command failed")
	}

	return nil
}

func (re *reenvoy) Stop() error {
	return nil
}

func (re *reenvoy) Reload() error {
	return nil
}

// sighupHandler is handler for signal SIGUP. this signal is used to ause the restarter to fork and exec a new child
func sighupHandler(signum, frame string) {
	log.Println("got SIGHUP")
}

// This routine forks and execs a new child process and keeps track of its PID. Before we fork,
// set the current restart epoch in an env variable that processes can read if they care.
func commandAndExec() {
	os.Setenv("RESTART_EPOCH", strconv.Itoa(restartEpoch))
	restartEpoch++

	log.Println("forking and execing new child process at epoch", restartEpoch)
	// cmd := exec.Command()

}
