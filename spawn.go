package reenvoy

import (
	"io"
	"time"
)

const (
	envoyDockerImage = "envoyproxy/envoy:3f59fb5c0f6554f8b3f2e73ab4c1437a63d42668"
)

//SpawnOptions spawn child options
type SpawnOptions struct {
	// ErrCh and DoneCh are channels where errors and finish notifications occur.
	ErrCh      chan error
	DoneCh     chan struct{}
	ConfigPath string

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

	// KillTimeout is the amount of time to wait for the process to gracefully
	// terminate before force-killing.
	KillTimeout time.Duration

	DockerContainer bool

	Stdout io.Writer
	StdErr io.Writer

	RestartEpoch int

	// ParentShutdownTimes The time in second that Envoy will wait before shutting down the parent process during a hot restart.
	// Readmore at https://www.envoyproxy.io/docs/envoy/v1.7.0/intro/arch_overview/hot_restart#arch-overview-hot-restart
	ParentShutdownTimes time.Duration

	//DrainTimes the time in second that Envoy will drain connection during restart
	DrainTimes time.Duration
}

//SpawnProcess spawn new process and return instance of process
func SpawnProcess(opt SpawnOptions) (*Process, error) {
	opt = defaultOptions(opt)
	p := &Process{
		Env:                 opt.Env,
		Timeout:             opt.Timeout,
		KillTimeout:         opt.KillTimeout,
		Stdout:              opt.Stdout,
		StdErr:              opt.StdErr,
		DockerContainer:     opt.DockerContainer,
		ConfigPath:          opt.ConfigPath,
		restartEpoch:        opt.RestartEpoch,
		DrainTimes:          opt.DrainTimes,
		ParentShutdownTimes: opt.ParentShutdownTimes,
	}

	if err := p.Start(); err != nil {
		return nil, err
	}

	return p, nil
}

func defaultOptions(opt SpawnOptions) SpawnOptions {
	if opt.KillTimeout.Nanoseconds() < 1 {
		opt.KillTimeout = 5 * time.Second
	}

	if opt.DrainTimes.Nanoseconds() < 1 {
		opt.DrainTimes = 60 * time.Second
	}

	if opt.ParentShutdownTimes.Nanoseconds() < 1 {
		opt.ParentShutdownTimes = 70 * time.Second
	}

	return opt
}
