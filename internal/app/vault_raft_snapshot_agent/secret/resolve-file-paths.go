package secret

import (
	"errors"
	"reflect"
	"strings"
)

var (
	errorInvalidType = errors.New("subject must be a struct passed by pointer")
	secretType       = reflect.TypeOf(Secret(""))
)

func ResolveFilePaths(subject interface{}, baseDir string) error {
	if baseDir == "" {
		return nil
	}

	if reflect.TypeOf(subject).Kind() != reflect.Ptr {
		return errorInvalidType
	}

	s := reflect.ValueOf(subject).Elem()

	return resolveSecretFilePaths(s, baseDir)
}

func resolveSecretFilePaths(value reflect.Value, baseDir string) error {
	t := value.Type()

	if t.Kind() != reflect.Struct {
		return errorInvalidType
	}

	for i := 0; i < t.NumField(); i++ {
		f := value.Field(i)

		if !f.CanSet() {
			continue
		}

		if f.Kind() == reflect.Ptr {
			f = f.Elem()
		}

		if f.Kind() == reflect.Struct {
			if err := resolveSecretFilePaths(f, baseDir); err != nil {
				return err
			}
		}

		if f.Type() != secretType || !strings.HasPrefix(f.String(), filePrefix) {
			continue
		}

		secret := f.Convert(secretType).MethodByName("WithAbsoluteFilePath").Call([]reflect.Value{reflect.ValueOf(baseDir)})
		f.Set(secret[0].Convert(secretType))
	}

	return nil
}
