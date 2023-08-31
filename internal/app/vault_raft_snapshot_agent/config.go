package vault_raft_snapshot_agent

import (
	"fmt"
	"log"
	"time"

	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/app/vault_raft_snapshot_agent/upload"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/app/vault_raft_snapshot_agent/vault"
)

type SnapshotterConfig struct {
	Vault     vault.VaultClientConfig
	Snapshots SnapshotConfig
	Uploaders upload.UploadersConfig
}

type SnapshotConfig struct {
	Frequency time.Duration `default:"1h" mapstructure:",omitempty"`
	Retain    int
	Timeout   time.Duration `default:"60s" mapstructure:",omitempty"`
}

var parser rattlesnake = newRattlesnake("snapshot", "VRSA", "/etc/vault.d/", ".")

// ReadConfig reads the configuration file
func ReadConfig(file string) (SnapshotterConfig, error) {
	parser.BindEnv("vault.url", "VAULT_ADDR")
	parser.BindEnv("uploaders.aws.credentials.key", "AWS_ACCESS_KEY_ID")
	parser.BindEnv("uploaders.aws.credentials.secret", "SECRET_ACCESS_KEY")

	config := SnapshotterConfig{}

	if file != "" {
		if err := parser.SetConfigFile(file); err != nil {
			return config, err
		}
	}

	if err := parser.ReadInConfig(); err != nil {
		if parser.IsConfigurationNotFoundError(err) {
			if file != "" {
				return config, err
			} else {
				log.Printf("Could not find any configuration file, will create configuration based solely on environment...")
			}
		} else {
			return config, err
		}
	}

	if err := parser.Unmarshal(&config); err != nil {
		return config, fmt.Errorf("could not unmarshal configuration: %s", err)
	}

	if !config.Uploaders.HasUploaders() {
		return config, fmt.Errorf("no uploaders configured!")
	}

	return config, nil
}

func WatchConfigAndReconfigure(snapshotter *Snapshotter) {
	parser.OnConfigChange(func() {
		config := SnapshotterConfig{}
		
		if err := parser.Unmarshal(&config); err != nil {
			log.Printf("Ignoring configuration change as configuration in %s is invalid: %v", parser.ConfigFileUsed(), err)
		}

		if err := snapshotter.Reconfigure(config); err != nil {
			log.Fatalf("Cound not reconfigure snapshotter from %s: %v", parser.ConfigFileUsed(), err)
		}
	})
}
