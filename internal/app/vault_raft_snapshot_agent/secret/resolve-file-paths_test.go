package secret

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"path/filepath"
	"testing"
)

func TestResolvesRelativePaths(t *testing.T) {
	var test struct {
		File  Secret
		Plain Secret
	}
	test.File = FromFile("./file")
	test.Plain = FromString("./plain")

	dir := t.TempDir()
	err := ResolveFilePaths(&test, dir)

	assert.NoError(t, err, "ResolveSecretFilePath failed unexpectedly")

	assert.Equal(t, FromFile(filepath.Clean(fmt.Sprintf("%s/file", dir))), test.File)
	assert.Equal(t, FromString("./plain"), test.Plain)
}

func TestResolvesRecursively(t *testing.T) {
	type inner struct {
		File Secret
	}

	innerPtr := inner{FromFile("./innerPtr")}

	var outer struct {
		Inner    inner
		InnerPtr *inner
	}
	outer.Inner.File = FromFile("./inner")
	outer.InnerPtr = &innerPtr

	dir := t.TempDir()
	err := ResolveFilePaths(&outer, dir)
	assert.NoError(t, err, "ResolveSecretFilePath failed unexpectedly")

	assert.Equal(t, FromFile(filepath.Clean(fmt.Sprintf("%s/inner", dir))), outer.Inner.File)
	assert.Equal(t, FromFile(filepath.Clean(fmt.Sprintf("%s/innerPtr", dir))), innerPtr.File)
}
