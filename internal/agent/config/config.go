package config

import (
	"fmt"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/logging"
)

type Parser[T Configuration] struct {
	delegate rattlesnake
}

type Configuration interface {
	HasUploaders() bool
}

func NewParser[T Configuration](envPrefix string, configFilename string, configSearchPaths ...string) Parser[T] {
	return Parser[T]{newRattlesnake(envPrefix, configFilename, configSearchPaths...)}
}

// ReadConfig reads the configuration file
func (p Parser[T]) ReadConfig(config T, file string) error {
	err := p.delegate.BindAllEnv(
		map[string]string{"vault.url": "VAULT_ADDR"},
	)
	if err != nil {
		return fmt.Errorf("could not bind environment-variables: %s", err)
	}

	if file != "" {
		if err := p.delegate.SetConfigFile(file); err != nil {
			return err
		}
	}

	if err := p.delegate.ReadInConfig(); err != nil {
		if p.delegate.IsConfigurationNotFoundError(err) {
			logging.Warn("Could not find any configuration file, will create configuration based solely on environment...")
		} else {
			return err
		}
	}

	if usedConfigFile := p.delegate.ConfigFileUsed(); usedConfigFile != "" {
		logging.Info(fmt.Sprintf("Using configuration from %s...", usedConfigFile))
	}

	if err := p.delegate.Unmarshal(config); err != nil {
		return fmt.Errorf("could not unmarshal configuration: %s", err)
	}

	if !config.HasUploaders() {
		return fmt.Errorf("no uploaders configured")
	}

	return nil
}

func (p Parser[T]) OnConfigChange(config T, handler func(config T) error) <-chan error {
	ch := make(chan error, 1)

	p.delegate.OnConfigChange(func() {
		if err := p.delegate.Unmarshal(config); err != nil {
			logging.Warn(
				"Ignoring configuration change as configuration is invalid",
				"config-file", p.delegate.ConfigFileUsed(),
				"error", err,
			)
			ch <- err
		} else {
			ch <- handler(config)
		}
	})

	return ch
}
