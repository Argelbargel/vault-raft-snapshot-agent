package config

import (
	"errors"
	"fmt"
	secret2 "github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/config/secret"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/creasty/defaults"
	"github.com/fsnotify/fsnotify"
	"github.com/go-playground/validator/v10"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/cast"
	"github.com/spf13/viper"
)

// a rattlesnake is a viper adapted to our needs ;-)
type rattlesnake struct {
	v *viper.Viper
}

func newRattlesnake(envPrefix string, configName string, configPaths ...string) rattlesnake {
	v := viper.New()
	v.SetEnvPrefix(envPrefix)
	v.SetConfigName(configName)
	for _, path := range configPaths {
		v.AddConfigPath(path)
	}
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	return rattlesnake{v}
}

func (r rattlesnake) BindEnv(input ...string) error {
	return r.v.BindEnv(input...)
}

func (r rattlesnake) BindAllEnv(env map[string]string) error {
	for k, v := range env {
		if err := r.BindEnv(k, v); err != nil {
			return err
		}
	}
	return nil
}

func (r rattlesnake) SetConfigFile(file string) error {
	if file != "" {
		file, err := filepath.Abs(file)
		if err != nil {
			return fmt.Errorf("could not build absolute path to config-file %s: %s", file, err)
		}
		r.v.SetConfigFile(file)
	}

	return nil
}

func (r rattlesnake) ReadInConfig() error {
	return r.v.ReadInConfig()
}

func (r rattlesnake) ConfigFileUsed() string {
	return r.v.ConfigFileUsed()
}

func (r rattlesnake) Unmarshal(config interface{}) error {
	if err := bindStruct(r.v, config); err != nil {
		return fmt.Errorf("could not bind env vars for configuration: %s", err)
	}

	if err := r.v.Unmarshal(config); err != nil {
		return err
	}

	if err := defaults.Set(config); err != nil {
		return fmt.Errorf("could not set configuration's default-values: %s", err)
	}

	if err := secret2.ResolveFilePaths(config, filepath.Dir(r.ConfigFileUsed())); err != nil {
		return fmt.Errorf("could not resolve relative paths in configuration: %s", err)
	}

	validate := validator.New()
	validate.RegisterCustomTypeFunc(validateSecret, secret2.Zero)
	if err := validate.Struct(config); err != nil {
		return err
	}

	return nil
}

func (r rattlesnake) OnConfigChange(run func()) {
	r.v.OnConfigChange(func(in fsnotify.Event) {
		run()
	})
	r.v.WatchConfig()
}

func (r rattlesnake) IsConfigurationNotFoundError(err error) bool {
	var configFileNotFoundError viper.ConfigFileNotFoundError
	notfound := errors.As(err, &configFileNotFoundError)
	return notfound
}

func validateSecret(field reflect.Value) interface{} {
	s, ok := field.Interface().(secret2.Secret)
	if !ok {
		return nil
	}

	v, err := s.Resolve(false)
	if err != nil {
		v = ""
	}
	return v
}

// implements automatic unmarshalling from environment variables
// see https://github.com/spf13/viper/pull/1429
// can be removed if that pr is merged
func bindStruct(v *viper.Viper, input interface{}) error {
	envKeysMap := map[string]interface{}{}
	if err := mapstructure.Decode(input, &envKeysMap); err != nil {
		return err
	}

	structKeys := flattenAndMergeMap(map[string]bool{}, envKeysMap, "")
	for key := range structKeys {
		if err := v.BindEnv(key); err != nil {
			return err
		}
	}

	return nil
}

func flattenAndMergeMap(shadow map[string]bool, m map[string]interface{}, prefix string) map[string]bool {
	if shadow != nil && prefix != "" && shadow[prefix] {
		// prefix is shadowed => nothing more to flatten
		return shadow
	}
	if shadow == nil {
		shadow = make(map[string]bool)
	}

	var m2 map[string]interface{}
	if prefix != "" {
		prefix += "."
	}
	for k, val := range m {
		fullKey := prefix + k
		switch val := val.(type) {
		case map[string]interface{}:
			m2 = val
		case map[interface{}]interface{}:
			m2 = cast.ToStringMap(val)
		default:
			// immediate value
			shadow[strings.ToLower(fullKey)] = true
			continue
		}
		// recursively merge to shadow map
		shadow = flattenAndMergeMap(shadow, m2, fullKey)
	}
	return shadow
}
