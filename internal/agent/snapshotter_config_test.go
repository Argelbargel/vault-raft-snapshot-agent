package agent

import (
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/config"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/secret"
	upload2 "github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/upload"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/vault"
	auth2 "github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/vault/auth"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

	expectedConfig := SnapshotterConfig{
		Vault: vault.ClientConfig{
			Url:      "https://example.com:8200",
			Insecure: true,
			Timeout:  5 * time.Minute,
			Auth: auth2.VaultAuthConfig{
				AppRole: auth2.AppRoleAuthConfig{
					Path:     "test-approle-path",
					RoleId:   "test-approle",
					SecretId: "test-approle-secret",
				},
				AWS: auth2.AWSAuthConfig{
					Path:             "test-aws-path",
					Role:             "test-aws-role",
					Region:           "test-region",
					EC2Nonce:         "test-nonce",
					EC2SignatureType: auth2.AWS_EC2_RSA2048,
				},
				Azure: auth2.AzureAuthConfig{
					Path:     "test-azure-path",
					Role:     "test-azure-role",
					Resource: "test-resource",
				},
				GCP: auth2.GCPAuthConfig{
					Path:                "test-gcp-path",
					Role:                "test-gcp-role",
					ServiceAccountEmail: "test@example.com",
				},
				Kubernetes: auth2.KubernetesAuthConfig{
					Role:     "test-kubernetes-role",
					Path:     "test-kubernetes-path",
					JWTToken: secret.FromFile(relativeTo(configFile, "./jwt")),
				},
				LDAP: auth2.LDAPAuthConfig{
					Path:     "test-ldap-path",
					Username: "test-ldap-user",
					Password: "test-ldap-pass",
				},
				Token: "test-token",
				UserPass: auth2.UserPassAuthConfig{
					Path:     "test-userpass-path",
					Username: "test-user",
					Password: "test-pass",
				},
			},
		},
		Snapshots: SnapshotConfig{
			Frequency:       time.Hour * 2,
			Retain:          10,
			Timeout:         time.Minute * 2,
			NamePrefix:      "test-",
			NameSuffix:      ".test",
			TimestampFormat: "2006-01-02",
		},
		Uploaders: upload2.UploadersConfig{
			AWS: upload2.AWSUploaderConfig{
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
			Azure: upload2.AzureUploaderConfig{
				AccountName: "test-account",
				AccountKey:  "test-key",
				Container:   "test-container",
				CloudDomain: "blob.core.chinacloudapi.cn",
			},
			GCP: upload2.GCPUploaderConfig{
				Bucket: "test-bucket",
			},
			Local: upload2.LocalUploaderConfig{
				Path: ".",
			},
			Swift: upload2.SwiftUploaderConfig{
				Container: "test-container",
				UserName:  "test-username",
				ApiKey:    "test-api-key",
				AuthUrl:   "https://auth.com",
				Domain:    "https://user.com",
				Region:    "test-region",
				TenantId:  "test-tenant",
				Timeout:   180 * time.Second,
			},
		},
	}

	data := SnapshotterConfig{}
	parser := config.NewParser[*SnapshotterConfig]("VRSA", "")
	err := parser.ReadConfig(&data, configFile)

	assert.NoError(t, err, "ReadConfig(%s) failed unexpectedly", configFile)
	assert.Equal(t, expectedConfig, data)
}

func TestReadConfigSetsDefaultValues(t *testing.T) {
	configFile := "../../testdata/snapshots.yaml"

	expectedConfig := SnapshotterConfig{
		Vault: vault.ClientConfig{
			Url:      "http://127.0.0.1:8200",
			Insecure: false,
			Timeout:  time.Minute,
			Auth: auth2.VaultAuthConfig{
				AppRole: auth2.AppRoleAuthConfig{
					Path:  "approle",
					Empty: true,
				},
				AWS: auth2.AWSAuthConfig{
					Path:             "aws",
					EC2SignatureType: auth2.AWS_EC2_PKCS7,
					Region:           secret.FromEnv("AWS_DEFAULT_REGION"),
					Empty:            true,
				},
				Azure: auth2.AzureAuthConfig{
					Path:  "azure",
					Empty: true,
				},
				GCP: auth2.GCPAuthConfig{
					Path:  "gcp",
					Empty: true,
				},
				Kubernetes: auth2.KubernetesAuthConfig{
					Role:     "test-role",
					Path:     "kubernetes",
					JWTToken: secret.FromFile(relativeTo(configFile, "./jwt")),
				},
				LDAP: auth2.LDAPAuthConfig{
					Path:  "ldap",
					Empty: true,
				},
				UserPass: auth2.UserPassAuthConfig{
					Path:  "userpass",
					Empty: true,
				},
			},
		},
		Snapshots: SnapshotConfig{
			Frequency:       time.Hour,
			Retain:          0,
			Timeout:         time.Minute,
			NamePrefix:      "raft-snapshot-",
			NameSuffix:      ".snap",
			TimestampFormat: "2006-01-02T15-04-05Z-0700",
		},
		Uploaders: upload2.UploadersConfig{
			AWS: upload2.AWSUploaderConfig{
				AccessKeyId:  secret.FromEnv("AWS_ACCESS_KEY_ID"),
				AccessKey:    secret.FromEnv("AWS_SECRET_ACCESS_KEY"),
				SessionToken: secret.FromEnv("AWS_SESSION_TOKEN"),
				Region:       secret.FromEnv("AWS_DEFAULT_REGION"),
				Endpoint:     secret.FromEnv("AWS_ENDPOINT_URL"),
				Empty:        true,
			},
			Azure: upload2.AzureUploaderConfig{
				AccountName: secret.FromEnv("AZURE_STORAGE_ACCOUNT"),
				AccountKey:  secret.FromEnv("AZURE_STORAGE_KEY"),
				CloudDomain: "blob.core.windows.net",
				Empty:       true,
			},
			GCP: upload2.GCPUploaderConfig{Empty: true},
			Local: upload2.LocalUploaderConfig{
				Path: ".",
			},
			Swift: upload2.SwiftUploaderConfig{
				UserName: secret.FromEnv("SWIFT_USERNAME"),
				ApiKey:   secret.FromEnv("SWIFT_API_KEY"),
				Region:   secret.FromEnv("SWIFT_REGION"),
				Timeout:  time.Minute,
				Empty:    true,
			},
		},
	}

	data := SnapshotterConfig{}
	parser := config.NewParser[*SnapshotterConfig]("VRSA", "")
	err := parser.ReadConfig(&data, configFile)

	assert.NoError(t, err, "ReadConfig(%s) failed unexpectedly", configFile)
	assert.Equal(t, expectedConfig, data)
}
