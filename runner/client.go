package runner

import (
	"fmt"

	"github.com/evo3cx/reenvoy"

	"github.com/hashicorp/consul-template/config"
	dep "github.com/hashicorp/consul-template/dependency"
)

// newClientSet creates a new client set from the given config.
func newClientSet(c *reenvoy.Config) (*dep.ClientSet, error) {
	clients := dep.NewClientSet()

	if err := clients.CreateConsulClient(&dep.CreateConsulClientInput{
		Address:                      config.StringVal(c.Consul.Address),
		Token:                        config.StringVal(c.Consul.Token),
		AuthEnabled:                  config.BoolVal(c.Consul.Auth.Enabled),
		AuthUsername:                 config.StringVal(c.Consul.Auth.Username),
		AuthPassword:                 config.StringVal(c.Consul.Auth.Password),
		SSLEnabled:                   config.BoolVal(c.Consul.SSL.Enabled),
		SSLVerify:                    config.BoolVal(c.Consul.SSL.Verify),
		SSLCert:                      config.StringVal(c.Consul.SSL.Cert),
		SSLKey:                       config.StringVal(c.Consul.SSL.Key),
		SSLCACert:                    config.StringVal(c.Consul.SSL.CaCert),
		SSLCAPath:                    config.StringVal(c.Consul.SSL.CaPath),
		ServerName:                   config.StringVal(c.Consul.SSL.ServerName),
		TransportDialKeepAlive:       config.TimeDurationVal(c.Consul.Transport.DialKeepAlive),
		TransportDialTimeout:         config.TimeDurationVal(c.Consul.Transport.DialTimeout),
		TransportDisableKeepAlives:   config.BoolVal(c.Consul.Transport.DisableKeepAlives),
		TransportIdleConnTimeout:     config.TimeDurationVal(c.Consul.Transport.IdleConnTimeout),
		TransportMaxIdleConns:        config.IntVal(c.Consul.Transport.MaxIdleConns),
		TransportMaxIdleConnsPerHost: config.IntVal(c.Consul.Transport.MaxIdleConnsPerHost),
		TransportTLSHandshakeTimeout: config.TimeDurationVal(c.Consul.Transport.TLSHandshakeTimeout),
	}); err != nil {
		return nil, fmt.Errorf("runner: %s", err)
	}

	if err := clients.CreateVaultClient(&dep.CreateVaultClientInput{
		Address:                      config.StringVal(c.Vault.Address),
		Token:                        config.StringVal(c.Vault.Token),
		UnwrapToken:                  config.BoolVal(c.Vault.UnwrapToken),
		SSLEnabled:                   config.BoolVal(c.Vault.SSL.Enabled),
		SSLVerify:                    config.BoolVal(c.Vault.SSL.Verify),
		SSLCert:                      config.StringVal(c.Vault.SSL.Cert),
		SSLKey:                       config.StringVal(c.Vault.SSL.Key),
		SSLCACert:                    config.StringVal(c.Vault.SSL.CaCert),
		SSLCAPath:                    config.StringVal(c.Vault.SSL.CaPath),
		ServerName:                   config.StringVal(c.Vault.SSL.ServerName),
		TransportDialKeepAlive:       config.TimeDurationVal(c.Vault.Transport.DialKeepAlive),
		TransportDialTimeout:         config.TimeDurationVal(c.Vault.Transport.DialTimeout),
		TransportDisableKeepAlives:   config.BoolVal(c.Vault.Transport.DisableKeepAlives),
		TransportIdleConnTimeout:     config.TimeDurationVal(c.Vault.Transport.IdleConnTimeout),
		TransportMaxIdleConns:        config.IntVal(c.Vault.Transport.MaxIdleConns),
		TransportMaxIdleConnsPerHost: config.IntVal(c.Vault.Transport.MaxIdleConnsPerHost),
		TransportTLSHandshakeTimeout: config.TimeDurationVal(c.Vault.Transport.TLSHandshakeTimeout),
	}); err != nil {
		return nil, fmt.Errorf("runner: %s", err)
	}

	return clients, nil
}
