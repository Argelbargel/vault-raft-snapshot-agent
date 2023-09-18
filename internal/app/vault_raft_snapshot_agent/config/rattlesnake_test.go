package config

import (
	"fmt"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/app/vault_raft_snapshot_agent/secret"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/app/vault_raft_snapshot_agent/test"
)

func TestUnmarshalResolvesRelativePathsInSecrets(t *testing.T) {
	rattlesnake := newRattlesnake("test", "TEST")

	config := struct {
		File secret.Secret
	}{
		File: "file://./file.ext",
	}

	baseDir := t.TempDir()
	err := rattlesnake.SetConfigFile(fmt.Sprintf("%s/config.yml", baseDir))
	assert.NoError(t, err, "SetConfigFile failed unexpectedly")

	err = rattlesnake.Unmarshal(&config)
	assert.NoError(t, err, "Unmarshal failed unexpectedly")
	assert.Equal(t, secret.FromFile(filepath.Clean(fmt.Sprintf("%s/file.ext", baseDir))), config.File)
}

func TestUnmarshalSetsDefaultValues(t *testing.T) {
	rattlesnake := newRattlesnake("test", "TEST")

	var config struct {
		Default string `default:"default-value"`
	}

	err := rattlesnake.Unmarshal(&config)

	assert.NoError(t, err, "Unmarshal failed unexpectedly")
	assert.Equal(t, "default-value", config.Default)
}

func TestUnmarshalValidatesValues(t *testing.T) {
	rattlesnake := newRattlesnake("test", "TEST")

	config := struct {
		Url string `validate:"http_url"`
	}{
		Url: "invalid-url",
	}

	err := rattlesnake.Unmarshal(&config)

	assert.Error(t, err, "Unmarshal should fail on validation error")
	assert.Equal(t, "invalid-url", config.Url)
}

func TestUnmarshalValidatesSecrets(t *testing.T) {
	rattlesnake := newRattlesnake("test", "TEST")

	config := struct {
		Secret secret.Secret `validate:"required"`
	}{
		Secret: secret.FromFile("./missing/file"),
	}

	err := rattlesnake.Unmarshal(&config)

	assert.Error(t, err, "Unmarshal should fail on validation error")
}

func TestOnConfigChangeRunsHandler(t *testing.T) {
	rattlesnake := newRattlesnake("test", "TEST")

	configFile := fmt.Sprintf("%s/config.yml", t.TempDir())

	err := test.WriteFile(t, configFile, "{\"value\": \"\"}")
	assert.NoError(t, err, "writing config file failed unexpectedly")

	err = rattlesnake.SetConfigFile(configFile)
	assert.NoError(t, err, "SetConfigFile failed unexpectedly")

	var config struct {
		Value string
	}

	err = rattlesnake.Unmarshal(&config)
	assert.NoError(t, err, "Unmarshal failed unexpectedly")

	changed := make(chan bool, 1)
	rattlesnake.OnConfigChange(func() {
		changed <- true
	})

	err = test.WriteFile(t, configFile, "{\"value\": \"new\"}")
	assert.NoError(t, err, "writing config file failed unexpectedly")

	assert.True(t, <-changed)
}
