# reenvoy

## Description

At a fundamental level, deploys of services (for this discussion services include the application, Envoy, log agents, stat relays, etc.) that do not drop traffic take place one of two ways

Read more at: https://blog.envoyproxy.io/envoy-hot-restart-1d16b14555b5



## Usage

```go

opts := reenvoy.SpawnOptions{
		KillTimeout:         2 * time.Second,
		DockerContainer:     false,
		ConfigPath:          c.ConfigPath,
		Stdout:              os.Stdout,
		StdErr:              os.Stderr,
		DrainTimes:          c.flags.drainTimes,
		ParentShutdownTimes: c.flags.parentShutdownTimes,
	}

	re := reenvoy.New(opts)
  
  ....
  
// Restart Envoy  
if err := re.Restart(); err != nil {
  logger.Error(err)
}
logger.Info("Success reload envoy")

// Kill Envoy and their children
re.ForceKillAllChildren()

```
