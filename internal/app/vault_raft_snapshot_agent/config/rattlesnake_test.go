package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

type rattlesnakeConfigStub struct {
	Path Path
}

func TestUnmarshalResolvesRelativePaths(t *testing.T) {
	wd, err := os.Getwd()
	assert.NoError(t, err, "Getwd failed unexpectedly")

	err = parser.SetConfigFile(fmt.Sprintf("%s/config.yml", wd))
	assert.NoError(t, err, "SetConfigFile failed unexpectedly")

	err = parser.BindAllEnv(
		map[string]string{
			"path": "TEST_PATH",
		},
	)
	assert.NoError(t, err, "BindAllEnv failed unexpectedly")

	t.Setenv("TEST_PATH", "./file.ext")

	config := rattlesnakeConfigStub{}
	parser.Unmarshal(&config)

	assert.Equal(t, Path(filepath.Clean(fmt.Sprintf("%s/file.ext", wd))), config.Path)
}
