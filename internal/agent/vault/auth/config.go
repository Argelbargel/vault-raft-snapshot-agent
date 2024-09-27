package auth


type VaultAuthConfig struct {
	AppRole    *AppRoleAuthConfig
	AWS        *AWSAuthConfig
	Azure      *AzureAuthConfig
	GCP        *GCPAuthConfig
	Kubernetes *KubernetesAuthConfig
	LDAP       *LDAPAuthConfig
	UserPass   *UserPassAuthConfig
	Token      *Token
}
