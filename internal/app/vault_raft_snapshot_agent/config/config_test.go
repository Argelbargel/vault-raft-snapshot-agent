package config

import (
	"fmt"
	"os"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

type configConfigStub struct {
	hasUploaders bool
	Vault        struct {
		Url  string `validate:"required"`
		Test string
	}
	Uploaders struct {
		AWS struct {
			Credentials struct {
				Key    string
				Secret string
			}
		}
	}
}

func (stub configConfigStub) HasUploaders() bool {
	return stub.hasUploaders
}

func TestReadConfigBindsEnvVariables(t *testing.T) {
	t.Setenv("VAULT_ADDR", "http://from.env:8200")
	t.Setenv("AWS_ACCESS_KEY_ID", "env-key")
	t.Setenv("SECRET_ACCESS_KEY", "env-secret")
	t.Setenv("VRSA_VAULT_TEST", "test")

	config := configConfigStub{hasUploaders: true}
	err := ReadConfig(&config, "")
	assert.NoError(t, err, "ReadConfig failed unexpectedly")

	assert.Equal(t, os.Getenv("VAULT_ADDR"), config.Vault.Url, "ReadConfig did not bind env-var VAULT_ADDR")
	assert.Equal(t, os.Getenv("AWS_ACCESS_KEY_ID"), config.Uploaders.AWS.Credentials.Key, "ReadConfig did not bind env-var AWS_ACCESS_KEY_ID")
	assert.Equal(t, os.Getenv("SECRET_ACCESS_KEY"), config.Uploaders.AWS.Credentials.Secret, "ReadConfig did not bind env-var SECRET_ACCESS_KEY")
	assert.Equal(t, os.Getenv("VRSA_VAULT_TEST"), config.Vault.Test, "ReadConfig did not bind env-var VRSA_VAULT_AUTH_KUBERNETES_JWTPATH")
}

func TestFailsOnMissingConfigFile(t *testing.T) {
	t.Setenv("VAULT_ADDR", "http://from.env:8200")
	config := configConfigStub{hasUploaders: true}
	err := ReadConfig(&config, "./missing.yaml")
	assert.Error(t, err, "ReadConfig should fail for missing config-file")
}

func TestFailsForInvalidConfiguration(t *testing.T) {
	config := configConfigStub{hasUploaders: true}
	err := ReadConfig(&config, "")
	assert.Error(t, err, "ReadConfig should fail for invalid configuration")
}

func TestFailsOnMissingUploaders(t *testing.T) {
	t.Setenv("VAULT_ADDR", "http://from.env:8200")
	config := configConfigStub{hasUploaders: false}
	err := ReadConfig(&config, "")
	assert.Error(t, err, "ReadConfig should fail for missing uploaders")
}

func TestOnConfigChangePassesConfigToHandler(t *testing.T) {
	configFile := fmt.Sprintf("%s/config.json", t.TempDir())
	config := configConfigStub{hasUploaders: true}

	err :=writeFile(t, configFile, "{\"vault\":{\"url\": \"test\"}}")
	assert.NoError(t, err, "writing config file failed unexpectedly")
	
	err = ReadConfig(&config, configFile)

	assert.NoError(t, err, "ReadConfig failed unexpectedly")
	assert.Equal(t, "test", config.Vault.Url)

	configCh := make(chan configConfigStub, 1)
	errCh := OnConfigChange(&configConfigStub{hasUploaders: true}, func(c *configConfigStub) error {
		configCh <- *c
		return nil
	})

	err = writeFile(t, configFile, "{\"vault\":{\"url\": \"new\"}}")
	assert.NoError(t, err, "writing config file failed unexpectedly")

	assert.NoError(t, <-errCh, "OnConfigChange failed unexpectedly")

	newConfig := <-configCh
	assert.Equal(t, "new", newConfig.Vault.Url)

	parser.OnConfigChange(func() { /* prevent error messages on cleanup */ })
}

func TestOnConfigChangeIgnoresInvalidConfiguration(t *testing.T) {
	configFile := fmt.Sprintf("%s/config.json", t.TempDir())
	config := configConfigStub{hasUploaders: true}

	err := writeFile(t, configFile, "{\"vault\":{\"url\": \"test\"}}")
	assert.NoError(t, err, "writing config file failed unexpectedly")

	err = ReadConfig(&config, configFile)
	assert.NoError(t, err, "ReadConfig failed unexpectedly")
	assert.Equal(t, "test", config.Vault.Url)

	newConfig := configConfigStub{hasUploaders: true}
	errCh := OnConfigChange(&newConfig, func(c *configConfigStub) error {
		c.Vault.Url = "new"
		return nil
	})

	err = writeFile(t, configFile, "{\"vault\":{}}")
	assert.NoError(t, err, "writing config file failed unexpectedly")

	assert.Error(t, <-errCh, "OnConfigChange should fail for invalid configuration")
	assert.Equal(t, "", newConfig.Vault.Url)

	parser.OnConfigChange(func() { /* prevent error messages on cleanup */ })
}

func writeFile(t *testing.T, dest string, contents string) error {
	t.Helper()

	if runtime.GOOS != "windows" {
		tmpFile := fmt.Sprintf("%s.tmp", dest)
		if err := os.WriteFile(tmpFile, []byte(contents), 0644); err != nil {
			return err
		}

		return os.Rename(tmpFile, dest)
	} else {
		return os.WriteFile(dest, []byte(contents), 0644)
	}
}
