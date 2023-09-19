package auth

import (
	"fmt"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/secret"

	"github.com/hashicorp/vault/api/auth/aws"
)

type AWSSignatureType string

const (
	AWS_EC2_PKCS7    AWSSignatureType = "pkcs7"
	AWS_ECS_IDENTITY AWSSignatureType = "identity"
	AWS_EC2_RSA2048  AWSSignatureType = "rsa2048"
)

type AWSAuthConfig struct {
	Path              string        `default:"aws"`
	Region            secret.Secret `default:"env://AWS_DEFAULT_REGION"`
	EC2Nonce          secret.Secret
	Role              string
	EC2SignatureType  AWSSignatureType `default:"pkcs7"`
	IAMServerIDHeader string
	Empty             bool
}

func createAWSAuth(config AWSAuthConfig) vaultAuthMethod[AWSAuthConfig, *aws.AWSAuth] {
	return vaultAuthMethod[AWSAuthConfig, *aws.AWSAuth]{
		config,
		func(config AWSAuthConfig) (*aws.AWSAuth, error) {
			var loginOpts = []aws.LoginOption{aws.WithMountPath(config.Path)}

			if !config.EC2Nonce.IsZero() {
				nonce, err := config.EC2Nonce.Resolve(true)
				if err != nil {
					return nil, err
				}
				loginOpts = append(loginOpts, aws.WithNonce(nonce), aws.WithEC2Auth())
				switch config.EC2SignatureType {
				case "":
				case AWS_EC2_PKCS7:
				case AWS_ECS_IDENTITY:
					loginOpts = append(loginOpts, aws.WithIdentitySignature())
				case AWS_EC2_RSA2048:
					loginOpts = append(loginOpts, aws.WithRSA2048Signature())
				default:
					return nil, fmt.Errorf("unknown signature-type %s", config.EC2SignatureType)
				}
			} else {
				loginOpts = append(loginOpts, aws.WithIAMAuth())
				if config.IAMServerIDHeader != "" {
					loginOpts = append(loginOpts, aws.WithIAMServerIDHeader(config.IAMServerIDHeader))
				}
			}

			region, err := config.Region.Resolve(false)
			if err != nil {
				return nil, err
			}

			if region != "" {
				loginOpts = append(loginOpts, aws.WithRegion(region))
			}

			if config.Role != "" {
				loginOpts = append(loginOpts, aws.WithRole(config.Role))
			}

			return aws.NewAWSAuth(loginOpts...)
		},
	}
}
