package runner

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"strings"

	"github.com/hashicorp/consul-template/config"
	dep "github.com/hashicorp/consul-template/dependency"
)

func applyTemplate(contents, key string) (string, error) {
	funcs := template.FuncMap{
		"key": func() (string, error) {
			return key, nil
		},
	}

	tmpl, err := template.New("filter").Funcs(funcs).Parse(contents)
	if err != nil {
		return "", nil
	}

	var buf bytes.Buffer
	if err = tmpl.Execute(&buf, nil); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func appendPrefixes(r *Runner,
	env map[string]string, d *dep.KVListQuery, data interface{}) error {
	var err error

	typed, ok := data.([]*dep.KeyPair)
	if !ok {
		return fmt.Errorf("error converting to keypair %s", d)
	}

	// Get the PrefixConfig so we can get configuration from it.
	cp := r.configPrefixMap[d.String()]

	// For each pair, update the environment hash. Subsequent runs could
	// overwrite an existing key.
	for _, pair := range typed {
		key, value := pair.Key, string(pair.Value)

		// It is not possible to have an environment variable that is blank, but
		// it is possible to have an environment variable _value_ that is blank.
		if strings.TrimSpace(key) == "" {
			continue
		}

		// If the user specified a custom format, apply that here.
		if config.StringPresent(cp.Format) {
			key, err = applyTemplate(config.StringVal(cp.Format), key)
			if err != nil {
				return err
			}
		}

		if config.BoolVal(r.config.Sanitize) {
			key = InvalidRegexp.ReplaceAllString(key, "_")
		}

		if config.BoolVal(r.config.Upcase) {
			key = strings.ToUpper(key)
		}

		if current, ok := env[key]; ok {
			log.Printf("[DEBUG] (runner) overwriting %s=%q (was %q) from %s", key, value, current, d)
			env[key] = value
		} else {
			log.Printf("[DEBUG] (runner) setting %s=%q from %s", key, value, d)
			env[key] = value
		}
	}

	return nil
}

func appendSecrets(r *Runner,
	env map[string]string, d *dep.VaultReadQuery, data interface{}) error {
	var err error

	typed, ok := data.(*dep.Secret)
	if !ok {
		return fmt.Errorf("error converting to secret %s", d)
	}

	// Get the PrefixConfig so we can get configuration from it.
	cp := r.configPrefixMap[d.String()]

	for key, value := range typed.Data {
		// Ignore any keys that are empty (not sure if this is even possible in
		// Vault, but I play defense).
		if strings.TrimSpace(key) == "" {
			continue
		}

		// Ignore any keys in which value is nil
		if value == nil {
			continue
		}

		if !config.BoolVal(cp.NoPrefix) {
			// Replace the path slashes with an underscore.
			pc, ok := r.configPrefixMap[d.String()]
			if !ok {
				return fmt.Errorf("missing dependency %s", d)
			}

			path := InvalidRegexp.ReplaceAllString(config.StringVal(pc.Path), "_")

			// Prefix the key value with the path value.
			key = fmt.Sprintf("%s_%s", path, key)
		}

		// If the user specified a custom format, apply that here.
		if config.StringPresent(cp.Format) {
			key, err = applyTemplate(config.StringVal(cp.Format), key)
			if err != nil {
				return err
			}
		}

		if config.BoolVal(r.config.Sanitize) {
			key = InvalidRegexp.ReplaceAllString(key, "_")
		}

		if config.BoolVal(r.config.Upcase) {
			key = strings.ToUpper(key)
		}

		if current, ok := env[key]; ok {
			log.Printf("[DEBUG] (runner) overwriting %s=%q (was %q) from %s", key, value, current, d)
			env[key] = value.(string)
		} else {
			log.Printf("[DEBUG] (runner) setting %s=%q from %s", key, value, d)
			env[key] = value.(string)
		}
	}

	return nil
}
