package reenvoy

import (
	"io"
	"time"

	"github.com/thejerf/suture"
)

const (
	envoyDockerImage            = "envoyproxy/envoy:3f59fb5c0f6554f8b3f2e73ab4c1437a63d42668"
	defaultDrainTimes           = time.Second * 60
	defaultParentShutdownTimes  = time.Second * 70
	defaultSpecFailureThreshold = 3
)

// Options spawn child options
type Options struct {
	// ConfigPath envoy config dir path
	ConfigPath string

	// Env specifies the environment of the process.
	// Each entry is of the form "key=value".
	// If Env is nil, the new process uses the current process's
	// environment.
	// If Env contains duplicate environment keys, only the last
	// value in the slice for each duplicate key is used.
	Env []string

	// Run envoy from container
	DockerContainer bool

	// ParentShutdownTimes The time in second that Envoy will wait before shutting down the parent process during a hot restart.
	// Readmore at https://www.envoyproxy.io/docs/envoy/v1.7.0/intro/arch_overview/hot_restart#arch-overview-hot-restart
	ParentShutdownTimes time.Duration

	//DrainTimes the time in second that Envoy will drain connection during restart
	DrainTimes time.Duration

	RestartEpoch int

	// Supervisor spec
	Spec suture.Spec

	// Process std output
	Stdout io.Writer
	StdErr io.Writer
}

func defaultOptions(opt Options) Options {

	if opt.DrainTimes.Nanoseconds() < 1 {
		opt.DrainTimes = defaultDrainTimes
	}

	if opt.ParentShutdownTimes.Nanoseconds() < 1 {
		opt.ParentShutdownTimes = defaultParentShutdownTimes
	}

	if opt.Spec.FailureThreshold == 0 {
		opt.Spec.FailureThreshold = defaultSpecFailureThreshold
	}
	return opt
}

func (opt Options) getCommand() (command string, args []string) {
	if opt.DockerContainer {
		return commandWithDocker(opt)
	}

	return commandWithEnvoy(opt)
}
