package agent

import (
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/config/secret"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/test"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/config"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/storage"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/vault"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/vault/auth"

	"github.com/stretchr/testify/assert"
)

func relativeTo(configFile string, file string) string {
	if !filepath.IsAbs(file) && !strings.HasPrefix(file, "/") {
		file = filepath.Join(filepath.Dir(configFile), file)
	}

	if !filepath.IsAbs(file) && !strings.HasPrefix(file, "/") {
		file, _ = filepath.Abs(file)
		file = filepath.Clean(file)
	}

	return file
}

func TestReadCompleteConfig(t *testing.T) {
	configFile := "../../testdata/complete.yaml"

	expectedConfig := SnapshotAgentConfig{
		Vault: vault.VaultClientConfig{
			Nodes: vault.VaultNodesConfig{
				Urls: []string{"https://example.com:8200"},
			},
			Insecure: true,
			Timeout:  5 * time.Minute,
			Auth: auth.VaultAuthConfig{
				AppRole: &auth.AppRoleAuthConfig{
					Path:     "test-approle-path",
					RoleId:   "test-approle",
					SecretId: "test-approle-secret",
				},
				AWS: &auth.AWSAuthConfig{
					Path:             "test-aws-path",
					Role:             "test-aws-role",
					Region:           "test-region",
					EC2Nonce:         "test-nonce",
					EC2SignatureType: auth.AWS_EC2_RSA2048,
				},
				Azure: &auth.AzureAuthConfig{
					Path:     "test-azure-path",
					Role:     "test-azure-role",
					Resource: "test-resource",
				},
				GCP: &auth.GCPAuthConfig{
					Path:                "test-gcp-path",
					Role:                "test-gcp-role",
					ServiceAccountEmail: "test@example.com",
				},
				Kubernetes: &auth.KubernetesAuthConfig{
					Role:     "test-kubernetes-role",
					Path:     "test-kubernetes-path",
					JWTToken: secret.FromFile(relativeTo(configFile, "./jwt")),
				},
				LDAP: &auth.LDAPAuthConfig{
					Path:     "test-ldap-path",
					Username: "test-ldap-user",
					Password: "test-ldap-pass",
				},
				Token: test.PtrTo[auth.Token]("test-token"),
				UserPass: &auth.UserPassAuthConfig{
					Path:     "test-userpass-path",
					Username: "test-user",
					Password: "test-pass",
				},
			},
		},
		Snapshots: SnapshotsConfig{
			StorageConfigDefaults: storage.StorageConfigDefaults{
				Frequency:       time.Hour * 2,
				Retain:          10,
				Timeout:         time.Minute * 2,
				NamePrefix:      "test-",
				NameSuffix:      ".test",
				TimestampFormat: "2006-01-02",
			},
			Storages: storage.StoragesConfig{
				AWS: &storage.AWSStorageConfig{
					AccessKeyId:             "test-key",
					AccessKey:               "test-secret",
					SessionToken:            "test-session",
					Endpoint:                "test-endpoint",
					Region:                  "test-region",
					Bucket:                  "test-bucket",
					KeyPrefix:               "test-prefix",
					UseServerSideEncryption: true,
					ForcePathStyle:          true,
				},
				Azure: &storage.AzureStorageConfig{
					StorageControllerConfig: storage.StorageControllerConfig{
						Retain: test.PtrTo(0),
					},
					AccountName: "test-account",
					AccountKey:  "test-key",
					Container:   "test-container",
					CloudDomain: "blob.core.chinacloudapi.cn",
				},
				GCP: &storage.GCPStorageConfig{
					StorageControllerConfig: storage.StorageControllerConfig{
						Retain: test.PtrTo(1),
					},
					Bucket: "test-bucket",
				},
				Local: &storage.LocalStorageConfig{
					StorageControllerConfig: storage.StorageControllerConfig{
						Retain: test.PtrTo(2),
					},
					Path: ".",
				},
				Swift: &storage.SwiftStorageConfig{
					StorageControllerConfig: storage.StorageControllerConfig{
						Retain: test.PtrTo(3),
					},
					Container: "test-container",
					UserName:  "test-username",
					ApiKey:    "test-api-key",
					AuthUrl:   "https://auth.com",
					Domain:    "https://user.com",
					Region:    "test-region",
					TenantId:  "test-tenant",
				},
				S3: &storage.S3StorageConfig{
					StorageControllerConfig: storage.StorageControllerConfig{
						Retain: test.PtrTo(4),
					},
					Endpoint:     "test-s3-endpoint",
					Bucket:       "test-s3-bucket",
					AccessKeyId:  "test-s3-key",
					AccessKey:    "test-s3-secret",
					SessionToken: "test-s3-token",
					Region:       "test-s3-region",
					Insecure:     true,
					SkipSSLVerify: true,
				},
			},
		},
	}

	data := SnapshotAgentConfig{}
	parser := config.NewParser[*SnapshotAgentConfig]("VRSA", "")
	err := parser.ReadConfig(&data, configFile)

	assert.NoError(t, err, "ReadConfig(%s) failed unexpectedly", configFile)
	assert.Equal(t, expectedConfig, data)
}

func TestReadConfigSetsDefaultValues(t *testing.T) {
	configFile := "../../testdata/snapshots.yaml"

	expectedConfig := SnapshotAgentConfig{
		Vault: vault.VaultClientConfig{
			Nodes: vault.VaultNodesConfig{
				Urls: []string {"http://127.0.0.1:8200"},
			},
			Insecure: false,
			Timeout:  time.Minute,
			Auth: auth.VaultAuthConfig{
				Kubernetes: &auth.KubernetesAuthConfig{
					Role:     "test-role",
					Path:     "kubernetes",
					JWTToken: secret.FromFile(relativeTo(configFile, "./jwt")),
				},
			},
		},
		Snapshots: SnapshotsConfig{
			StorageConfigDefaults: storage.StorageConfigDefaults{
				Frequency:       time.Hour,
				Retain:          0,
				Timeout:         time.Minute,
				NamePrefix:      "raft-snapshot-",
				NameSuffix:      ".snap",
				TimestampFormat: "2006-01-02T15-04-05Z-0700",
			},
			Storages: storage.StoragesConfig{
				Local: &storage.LocalStorageConfig{
					Path: ".",
				},
			},
		},
	}

	data := SnapshotAgentConfig{}
	parser := config.NewParser[*SnapshotAgentConfig]("VRSA", "")
	err := parser.ReadConfig(&data, configFile)

	assert.NoError(t, err, "ReadConfig(%s) failed unexpectedly", configFile)
	assert.Equal(t, expectedConfig, data)
}
