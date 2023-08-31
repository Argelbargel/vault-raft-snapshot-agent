package vault_raft_snapshot_agent

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/app/vault_raft_snapshot_agent/upload"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/app/vault_raft_snapshot_agent/vault"
	"github.com/creasty/defaults"
	"github.com/fsnotify/fsnotify"
	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
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

// ReadConfig reads the configuration file
func ReadConfig(file string) (SnapshotterConfig, error) {
	config := SnapshotterConfig{}

	viper.AddConfigPath("/etc/vault.d/")
	viper.AddConfigPath(".")
	viper.SetConfigName("snapshot")
	viper.BindEnv("vault.url", "VAULT_ADDR")
	viper.BindEnv("uploaders.aws.credentials.key", "AWS_ACCESS_KEY_ID")
	viper.BindEnv("uploaders.aws.credentials.secret", "SECRET_ACCESS_KEY")

	if file != "" {
		file, err := filepath.Abs(file)
		if err != nil {
			return config, fmt.Errorf("could not build absolute path to config-file %s: %s", file, err)
		}

		viper.SetConfigFile(file)
	}

	err := viper.ReadInConfig()
	if err != nil {
		return config, fmt.Errorf("error reading config file: %s", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		return config, fmt.Errorf("could not determine current working directory: %s", err)
	}

	configDir := filepath.Dir(file)
	if err := os.Chdir(configDir); err != nil {
		return config, fmt.Errorf("could not switch working-directory to %s to parse configuration: %s", configDir, err)
	}

	defer os.Chdir(wd)
	return unmarshalConfig(config)
}

func unmarshalConfig(config SnapshotterConfig) (SnapshotterConfig, error) {
	err := viper.Unmarshal(&config)
	if err != nil {
		return config, err
	}

	if err := defaults.Set(&config); err != nil {
		return config, fmt.Errorf("could not set configuration's default-values: %s", err)
	}

	validate := validator.New()
	if err := validate.Struct(config); err != nil {
		return config, err
	}

	if !config.Uploaders.HasUploaders() {
		return config, fmt.Errorf("no uploaders configured!")
	}

	return config, nil
}

func WatchConfigAndReconfigure(snapshotter *Snapshotter) {
	viper.OnConfigChange(func(e fsnotify.Event) {
		config, err := unmarshalConfig(SnapshotterConfig{})
		if err != nil {
			log.Printf("ignoring invalid configuration-change: %v", err)
		}

		err = snapshotter.Reconfigure(config)
		if err != nil {
			log.Fatalf("error reconfiguring snapshotter: %v", err)
		}

	})
	viper.WatchConfig()

}
