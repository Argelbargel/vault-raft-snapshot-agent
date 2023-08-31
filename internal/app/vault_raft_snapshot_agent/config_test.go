package vault_raft_snapshot_agent

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/app/vault_raft_snapshot_agent/upload"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/app/vault_raft_snapshot_agent/vault"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/app/vault_raft_snapshot_agent/vault/auth"
)

var defaultJwtPath string = "/var/run/secrets/kubernetes.io/serviceaccount/token"

func TestReadEmptyConfig(t *testing.T) {
	file := "../../../testdata/empty.yaml"
	config, err := ReadConfig(file)
	if err == nil {
		t.Fatalf(`ReadConfig(%s) should return error for empty file, got %v`, file, config)
	}
	t.Logf("ReadConfig(%s) returned error: %v", file, err)
}

func TestReadConfigWithInvalidAddr(t *testing.T) {
	file := "../../../testdata/invalid-addr.yaml"
	config, err := ReadConfig(file)
	if err == nil {
		t.Fatalf(`ReadConfig(%s) should return error for config with invalid addr, got %v`, file, config)
	}
	t.Logf("ReadConfig(%s) returned error: %v", file, err)
}
func TestReadConfigWithoutUploaders(t *testing.T) {
	file := "../../../testdata/no-uploaders.yaml"
	config, err := ReadConfig(file)
	if err == nil {
		t.Fatalf(`ReadConfig(%s) should return error for config without uploaders, got %v`, file, config)
	}
	t.Logf("ReadConfig(%s) returned error: %v", file, err)
}

func TestReadConfigWithInvalidUploader(t *testing.T) {
	file := "../../../testdata/invalid-uploader.yaml"
	config, err := ReadConfig(file)
	if err == nil {
		t.Fatalf(`ReadConfig(%s) should return error for config with invalid uploader, got %v`, file, config)
	}
	t.Logf("ReadConfig(%s) returned error: %v", file, err)
}

func TestReadConfigWithInvalidLocalUploadPath(t *testing.T) {
	file := "../../../testdata/invalid-local-upload-path.yaml"
	config, err := ReadConfig(file)
	if err == nil {
		t.Fatalf(`ReadConfig(%s) should return error for config with invalid local upload-path, got %v`, file, config)
	}
	t.Logf("ReadConfig(%s) returned error: %v", file, err)
}

func TestReadConfigWithInvalidAuth(t *testing.T) {
	file := "../../../testdata/invalid-auth.yaml"
	config, err := ReadConfig(file)
	if err == nil {
		t.Fatalf(`ReadConfig(%s) should return error for config with invalid auth, got %v`, file, config)
	}
	t.Logf("ReadConfig(%s) returned error: %v", file, err)
}

func TestReadCompleteConfig(t *testing.T) {
	expectedConfig := SnapshotterConfig{
		Vault: vault.VaultClientConfig{
			Url:      "https://example.com:8200",
			Insecure: true,
			Auth: auth.AuthConfig{
				AppRole: auth.AppRoleAuthConfig{
					Path:  "approle",
					Empty: true,
				},
				Kubernetes: auth.KubernetesAuthConfig{
					Role:    "test-role",
					Path:    "test-auth",
					JWTPath: "./jwt",
				},
			},
		},
		Snapshots: SnapshotConfig{
			Frequency: time.Hour * 2,
			Retain:    10,
			Timeout:   time.Minute * 2,
		},
		Uploaders: upload.UploadersConfig{
			AWS: upload.AWSConfig{
				Endpoint:                "test-endpoint",
				Region:                  "test-region",
				Bucket:                  "test-bucket",
				KeyPrefix:               "test-prefix",
				UseServerSideEncryption: true,
				ForcePathStyle:          true,
				Credentials: upload.AWSCredentialsConfig{
					Key:    "test-key",
					Secret: "test-secret",
				},
			},
			Azure: upload.AzureConfig{
				AccountName:   "test-account",
				AccountKey:    "test-key",
				ContainerName: "test-container",
			},
			GCP: upload.GCPConfig{
				Bucket: "test-bucket",
			},
			Local: upload.LocalConfig{
				Path: ".",
			},
		},
	}

	file := "../../../testdata/complete.yaml"
	config, err := ReadConfig(file)
	if err != nil {
		t.Fatalf("ReadConfig(%s) failed unexpectedly: %v", file, err)
	}

	if !reflect.DeepEqual(expectedConfig, config) {
		t.Fatalf("ReadConfig returned unexpected config - expected %v, got %v", expectedConfig, config)
	}
}

func TestReadConfigSetsDefaultValues(t *testing.T) {
	expectedConfig := SnapshotterConfig{
		Vault: vault.VaultClientConfig{
			Url:      "http://127.0.0.1:8200",
			Insecure: false,
			Auth: auth.AuthConfig{
				AppRole: auth.AppRoleAuthConfig{
					Path:  "approle",
					Empty: true,
				},
				Kubernetes: auth.KubernetesAuthConfig{
					Role:    "test-role",
					Path:    "kubernetes",
					JWTPath: defaultJwtPath,
				},
			},
		},
		Snapshots: SnapshotConfig{
			Frequency: time.Hour,
			Retain:    0,
			Timeout:   time.Minute,
		},
		Uploaders: upload.UploadersConfig{
			AWS: upload.AWSConfig{
				Credentials: upload.AWSCredentialsConfig{Empty: true},
				Empty:       true,
			},
			Azure: upload.AzureConfig{Empty: true},
			GCP:   upload.GCPConfig{Empty: true},
			Local: upload.LocalConfig{
				Path: ".",
			},
		},
	}

	file := "../../../testdata/defaults.yaml"
	config, err := ReadConfig(file)
	if err != nil {
		t.Fatalf("ReadConfig(%s) failed unexpectedly: %v", file, err)
	}

	if !reflect.DeepEqual(expectedConfig, config) {
		t.Fatalf("ReadConfig returned unexpected config - expected %v, got %v", expectedConfig, config)
	}
}

func TestReadConfigBindsEnvVariables(t *testing.T) {
	os.Setenv("VAULT_ADDR", "http://from.env:8200")
	os.Setenv("AWS_ACCESS_KEY_ID", "env-key")
	os.Setenv("SECRET_ACCESS_KEY", "env-secret")

	file := "../../../testdata/envvars.yaml"
	config, err := ReadConfig(file)
	if err != nil {
		t.Fatalf("ReadConfig(%s) failed unexpectedly: %v", file, err)
	}

	if config.Vault.Url != os.Getenv("VAULT_ADDR") {
		t.Fatalf("ReadConfig did not bind env-var VAULT_ADDR - expected %s, got %s", os.Getenv("VAULT_ADDR"), config.Vault.Url)
	}

	if config.Uploaders.AWS.Credentials.Key != os.Getenv("AWS_ACCESS_KEY_ID") {
		t.Fatalf("ReadConfig did not bind env-var AWS_ACCESS_KEY_ID - expected %s, got %s", os.Getenv("AWS_ACCESS_KEY_ID"), config.Uploaders.AWS.Credentials.Key)
	}

	if config.Uploaders.AWS.Credentials.Secret != os.Getenv("SECRET_ACCESS_KEY") {
		t.Fatalf("ReadConfig did not bind env-var SECRET_ACCESS_KEY - expected %s, got %s", os.Getenv("SECRET_ACCESS_KEY"), config.Uploaders.AWS.Credentials.Secret)
	}


}

func init() {
	if err := os.MkdirAll(filepath.Dir(defaultJwtPath), 0777); err != nil && !errors.Is(err, os.ErrExist) {
		log.Fatalf("could not create directorys for jwt-file %s: %v", defaultJwtPath, err)
	}

	file, err := os.OpenFile(defaultJwtPath, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		log.Fatalf("could not create jwt-file %s: %v", defaultJwtPath, err)
	}

	file.Close()

	if err != nil {
		log.Fatalf("could not read jwt-file %s: %v", defaultJwtPath, err)
	}
}
