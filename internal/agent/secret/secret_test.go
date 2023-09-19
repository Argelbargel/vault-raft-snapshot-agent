package secret

import (
	"fmt"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/test"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSecretResolvesEnvironmentVariable(t *testing.T) {
	t.Setenv("TEST", "resolved")

	secret := FromEnv("TEST")
	v, err := secret.Resolve(true)

	assert.NoError(t, err, "resolve failed unexpectedly")
	assert.Equal(t, os.Getenv("TEST"), v)
}

func TestSecretResolvesFile(t *testing.T) {
	secretFile := fmt.Sprintf("%s/secret", t.TempDir())
	err := test.WriteFile(t, secretFile, "secret")
	assert.NoError(t, err, "could not write file %s", secretFile)

	secret := FromFile(secretFile)
	v, err := secret.Resolve(true)

	assert.NoError(t, err, "resolve failed unexpectedly")
	assert.Equal(t, "secret", v)
}

func TestSecretResolvesPlainString(t *testing.T) {
	secret := Secret("plain")
	v, err := secret.Resolve(true)

	assert.NoError(t, err, "resolve failed unexpectedly")
	assert.Equal(t, "plain", v)
}

func TestRequiredResolveFailsIfEnvVarIsMissing(t *testing.T) {
	secret := FromEnv("TEST")
	v, err := secret.Resolve(true)

	assert.Error(t, err, "resolve should fail if environment-variable is missing")
	assert.Zero(t, v)
}

func TestOptionalResolveReturnsEmptyIfEnvVarIsMissing(t *testing.T) {
	secret := FromEnv("TEST")
	v, err := secret.Resolve(false)

	assert.NoError(t, err, "resolve should not fail for missing environment-variable when not required")
	assert.Zero(t, v)
}

func TestRequiredResolveFailsIfFileCanNotBeRead(t *testing.T) {
	secret := FromFile("/missing/file")
	v, err := secret.Resolve(true)

	assert.Error(t, err, "resolve should fail if file can not be read")
	assert.Zero(t, v)
}

func TestOptionalResolveReturnsEmptyIfFileCanNotBeRead(t *testing.T) {
	secret := FromFile("/missing/file")
	v, err := secret.Resolve(false)

	assert.NoError(t, err, "resolve should not fail for missing file when not required")
	assert.Zero(t, v)
}

func TestStringResolvesSecret(t *testing.T) {
	t.Setenv("TEST", "resolved")

	secret := FromEnv("TEST")
	assert.Equal(t, os.Getenv("TEST"), secret.String())
}

func TestStringReturnsEmptyIfSecretCanNotBeResolved(t *testing.T) {
	assert.Equal(t, "", FromEnv("TEST").String())
}

func TestWithAbsoluteFilePathResolvesRelativeFilePath(t *testing.T) {
	baseDir := t.TempDir()
	secret := FromFile("./test")

	assert.Equal(t, FromFile(filepath.Clean(fmt.Sprintf("%s/test", baseDir))), secret.WithAbsoluteFilePath(baseDir))
}

func TestWithAbsoluteFileReturnsForEmptyBaseDir(t *testing.T) {
	secret := FromFile("./test")
	assert.Equal(t, secret, secret.WithAbsoluteFilePath(""))
}

func TestWithAbsoluteFileReturnsForEnvSecret(t *testing.T) {
	secret := FromEnv("test")
	assert.Equal(t, secret, secret.WithAbsoluteFilePath(t.TempDir()))
}

func TestWithAbsoluteFileReturnsForPlainSecret(t *testing.T) {
	secret := FromString("test")
	assert.Equal(t, secret, secret.WithAbsoluteFilePath(t.TempDir()))
}
