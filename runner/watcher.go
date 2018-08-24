package runner

import (
	"log"

	"github.com/evo3cx/reenvoy"
	"github.com/hashicorp/consul-template/config"
	dep "github.com/hashicorp/consul-template/dependency"
	"github.com/hashicorp/consul-template/watch"
	"github.com/pkg/errors"
)

// newWatcher creates a new watcher.
func newWatcher(c *reenvoy.Config, clients *dep.ClientSet, once bool) (*watch.Watcher, error) {
	log.Printf("[INFO] (runner) creating watcher")

	w, err := watch.NewWatcher(&watch.NewWatcherInput{
		Clients:         clients,
		MaxStale:        config.TimeDurationVal(c.MaxStale),
		Once:            once,
		RenewVault:      config.StringPresent(c.Vault.Token) && config.BoolVal(c.Vault.RenewToken),
		RetryFuncConsul: watch.RetryFunc(c.Consul.Retry.RetryFunc()),
		// TODO: Add a sane default retry - right now this only affects "local"
		// dependencies like reading a file from disk.
		RetryFuncDefault: nil,
		RetryFuncVault:   watch.RetryFunc(c.Vault.Retry.RetryFunc()),
		VaultGrace:       config.TimeDurationVal(c.Vault.Grace),
		VaultToken:       config.StringVal(c.Vault.Token),
	})
	if err != nil {
		return nil, errors.Wrap(err, "runner")
	}
	return w, nil
}
