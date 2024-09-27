package config

import (
	"fmt"
	"os"
	"testing"

	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/test"
	"github.com/stretchr/testify/assert"
)

type configDataStub struct {
	hasStorages bool
	Vault       struct {
		Url  string `validate:"required"`
		Test string
	}
	Storages struct {
		AWS struct {
			Credentials struct {
				Key    string
				Secret string
			}
		}
	}
}

func (stub configDataStub) HasStorages() bool {
	return stub.hasStorages
}

func TestReadConfigBindsEnvVariables(t *testing.T) {
	parser := NewParser[*configDataStub]("TEST", "")

	t.Setenv("VAULT_ADDR", "http://from.env:8200")
	t.Setenv("TEST_VAULT_TEST", "test")

	data := configDataStub{hasStorages: true}
	err := parser.ReadConfig(&data, "")
	assert.NoError(t, err, "ReadConfig failed unexpectedly")

	assert.Equal(t, os.Getenv("VAULT_ADDR"), data.Vault.Url, "ReadConfig did not bind env-var VAULT_ADDR")
	assert.Equal(t, os.Getenv("TEST_VAULT_TEST"), data.Vault.Test, "ReadConfig did not bind env-var TEST_VAULT_TEST")
}

func TestFailsOnMissingConfigFile(t *testing.T) {
	parser := NewParser[*configDataStub]("TEST", "")

	t.Setenv("VAULT_ADDR", "http://from.env:8200")

	data := configDataStub{hasStorages: true}
	err := parser.ReadConfig(&data, "./missing.yaml")
	assert.Error(t, err, "ReadConfig should fail for missing config-file")
}

func TestFailsForInvalidConfiguration(t *testing.T) {
	parser := NewParser[*configDataStub]("TEST", "")

	data := configDataStub{hasStorages: true}
	err := parser.ReadConfig(&data, "")
	assert.Error(t, err, "ReadConfig should fail for invalid configuration")
}

func TestFailsOnMissingStorages(t *testing.T) {
	parser := NewParser[*configDataStub]("TEST", "")

	t.Setenv("VAULT_ADDR", "http://from.env:8200")

	data := configDataStub{hasStorages: false}
	err := parser.ReadConfig(&data, "")
	assert.Error(t, err, "ReadConfig should fail for missing storages")
}

func TestOnConfigChangePassesConfigToHandler(t *testing.T) {
	parser := NewParser[*configDataStub]("TEST", "")

	configFile := fmt.Sprintf("%s/config.json", t.TempDir())
	config := configDataStub{hasStorages: true}

	err := test.WriteFile(t, configFile, "{\"vault\":{\"url\": \"test\"}}")
	assert.NoError(t, err, "writing config file failed unexpectedly")

	err = parser.ReadConfig(&config, configFile)

	assert.NoError(t, err, "ReadConfig failed unexpectedly")
	assert.Equal(t, "test", config.Vault.Url)

	configCh := make(chan configDataStub, 1)
	errCh := parser.OnConfigChange(&configDataStub{hasStorages: true}, func(c *configDataStub) error {
		configCh <- *c
		return nil
	})

	err = test.WriteFile(t, configFile, "{\"vault\":{\"url\": \"new\"}}")
	assert.NoError(t, err, "writing config file failed unexpectedly")

	assert.NoError(t, <-errCh, "OnConfigChange failed unexpectedly")

	newConfig := <-configCh
	assert.Equal(t, "new", newConfig.Vault.Url)

	parser.delegate.OnConfigChange(func() { /* prevent error messages on cleanup */ })
}

func TestOnConfigChangeIgnoresInvalidConfiguration(t *testing.T) {
	parser := NewParser[*configDataStub]("TEST", "")

	configFile := fmt.Sprintf("%s/config.json", t.TempDir())
	config := configDataStub{hasStorages: true}

	err := test.WriteFile(t, configFile, "{\"vault\":{\"url\": \"test\"}}")
	assert.NoError(t, err, "writing config file failed unexpectedly")

	err = parser.ReadConfig(&config, configFile)
	assert.NoError(t, err, "ReadConfig failed unexpectedly")
	assert.Equal(t, "test", config.Vault.Url)

	newConfig := configDataStub{hasStorages: true}
	errCh := parser.OnConfigChange(&newConfig, func(c *configDataStub) error {
		c.Vault.Url = "new"
		return nil
	})

	err = test.WriteFile(t, configFile, "{\"vault\":{}}")
	assert.NoError(t, err, "writing config file failed unexpectedly")

	assert.Error(t, <-errCh, "OnConfigChange should fail for invalid configuration")
	assert.Equal(t, "", newConfig.Vault.Url)

	parser.delegate.OnConfigChange(func() { /* prevent error messages on cleanup */ })
}
