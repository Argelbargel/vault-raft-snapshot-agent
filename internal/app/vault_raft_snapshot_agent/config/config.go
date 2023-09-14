package config

import (
	"fmt"
	"log"
)

var parser rattlesnake = newRattlesnake("snapshot", "VRSA", "/etc/vault.d/", ".")

type Config interface {
	HasUploaders() bool
}

// ReadConfig reads the configuration file
func ReadConfig[T Config](config T, file string) error {
	err := parser.BindAllEnv(
		map[string]string{
			"vault.url":                        "VAULT_ADDR",
			"uploaders.aws.credentials.key":    "AWS_ACCESS_KEY_ID",
			"uploaders.aws.credentials.secret": "SECRET_ACCESS_KEY",
		},
	)
	if err != nil {
		return fmt.Errorf("could not bind environment-variables: %s", err)
	}

	if file != "" {
		if err := parser.SetConfigFile(file); err != nil {
			return err
		}
	}

	if err := parser.ReadInConfig(); err != nil {
		if parser.IsConfigurationNotFoundError(err) {
			log.Printf("Could not find any configuration file, will create configuration based solely on environment...")
		} else {
			return err
		}
	}

	if usedConfigFile := parser.ConfigFileUsed(); usedConfigFile != "" {
		log.Printf("Using configuration from %s...\n", usedConfigFile)
	}

	if err := parser.Unmarshal(config); err != nil {
		return fmt.Errorf("could not unmarshal configuration: %s", err)
	}

	if !config.HasUploaders() {
		return fmt.Errorf("no uploaders configured!")
	}

	return nil
}

func OnConfigChange[T Config](config T, handler func(config T) error) <-chan error {
	ch := make(chan error, 1)

	parser.OnConfigChange(func() {
		if err := parser.Unmarshal(config); err != nil {
			log.Printf("Ignoring configuration change as configuration in %s is invalid: %v\n", parser.ConfigFileUsed(), err)
			ch <- err
		} else {
			ch <- handler(config)
		}
	})

	return ch
}
