package secret

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type Secret string

const (
	Zero       = Secret("")
	envPrefix  = "env://"
	filePrefix = "file://"
)

func FromEnv(varName string) Secret {
	return withPrefix(envPrefix, varName)
}

func FromFile(file string) Secret {
	return withPrefix(filePrefix, file)
}

func FromString(value string) Secret {
	return Secret(value)
}

func withPrefix(prefix string, value string) Secret {
	return FromString(prefix + value)
}

func (s Secret) String() string {
	v, err := s.Resolve(false)
	if err != nil {
		log.Panicf("could not resolve %s: %s", string(s), err)
	}

	return v
}

func (s Secret) IsZero() bool {
	return s == Zero
}

func (s Secret) Resolve(required bool) (string, error) {
	v := string(s)

	if strings.HasPrefix(v, envPrefix) {
		name := strings.TrimPrefix(v, envPrefix)
		value, present := os.LookupEnv(name)
		if !present && required {
			return "", fmt.Errorf("environment variable %s is not present", name)
		}
		return value, nil
	}

	if strings.HasPrefix(v, filePrefix) {
		file := strings.TrimPrefix(v, filePrefix)
		value, err := os.ReadFile(file)
		if err != nil && (!os.IsNotExist(err) || required) {
			return "", fmt.Errorf("could not read file %s", file)
		}
		return string(value), nil
	}

	return v, nil
}

func (s Secret) WithAbsoluteFilePath(baseDir string) Secret {
	if baseDir == "" {
		return s
	}

	v := string(s)
	if !strings.HasPrefix(v, filePrefix) {
		return s
	}

	file := strings.TrimPrefix(v, filePrefix)
	if filepath.IsAbs(file) || strings.HasPrefix(file, "/") {
		return s
	}

	file = filepath.Join(baseDir, file)
	return FromFile(file)
}

func (s Secret) SetDefaults() {
	fmt.Println("setting secret-defaults")
}
