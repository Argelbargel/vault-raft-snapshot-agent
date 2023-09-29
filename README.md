[![Release](https://img.shields.io/github/v/release/Argelbargel/vault-raft-snapshot-agent)](https://github.com/Argelbargel/vault-raft-snapshot-agent/releases/latest)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/vault-raft-snapshot-agent)](https://artifacthub.io/packages/search?repo=vault-raft-snapshot-agent)

# Vault Raft Snapshot Agent

Vault Raft Snapshot Agent is a Go binary that takes periodic snapshots of a [Vault](https://www.vaultproject.io/) HA
cluster using
the [integrated raft storage backend](https://developer.hashicorp.com/vault/docs/concepts/integrated-storage). It can
store the snapshots locally or upload them to a remote storage backend like AWS S3 as backup in case of system failure
or user errors.

## Running

### Container-Image

You can run the agent with the supplied container-image, e.g. via docker:

```
docker run -v <path to snapshot.json>:/etc/vault.d/snapshot.json" ghcr.io/argelbargel/vault-raft-snapshot-agent:latest
```

### Helm-Chart

If you're running on kubernetes, you can use the
provided [Helm-Charts](https://argelbargel.github.io/vault-raft-snapshot-agent-helm/) to install Vault Raft Snapshot
Agent into your cluster.

### systemd-service

The recommended way of running this daemon is using systemctl, since it handles restarts and failure scenarios quite
well. To learn more about systemctl,
checkout [this article](https://www.digitalocean.com/community/tutorials/how-to-use-systemctl-to-manage-systemd-services-and-units).
begin, create the following file at `/etc/systemd/system/snapshot.service`:

```
[Unit]
Description="An Open Source Snapshot Service for Raft"
Documentation=https://github.com/Argelbargel/vault-raft-snapshot-agent/
Requires=network-online.target
After=network-online.target
ConditionFileNotEmpty=/etc/vault.d/snapshot.json

[Service]
Type=simple
User=vault
Group=vault
ExecStart=/usr/local/bin/vault-raft-snapshot-agent
ExecReload=/usr/local/bin/vault-raft-snapshot-agent
KillMode=process
Restart=on-failure
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

Your configuration is assumed to exist at `/etc/vault.d/snapshot.json` and the actual daemon binary
at `/usr/local/bin/vault-raft-snapshot-agent`.

Then just run:

```
sudo systemctl enable snapshot
sudo systemctl start snapshot
```

If your configuration is right and Vault is running on the same host as the agent you will see one of the following:

`Not running on leader node, skipping.` or `Successfully created <type> snapshot to <location>`, depending on if the
daemon runs on the leader's host or not.

## Command-Line Options and Logging

Most of the agents' configuration is done via its [configuration-file or environment variables](#configuration).
The location of a custom configuration-file and logging are specified via the command-line:

| Long option             | Short option  | Description                                                                                                                                                                 |
| ----------------------- | ------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `--config <file>`       | `-c <file>`   | <a id="cli-config"></a>load configuration from `<file>`; if not specified, searches for `snapshots.\[json\|toml\|yaml\]` in `/etc/vault.d` or the current working directory |
| `--log-format <format>` | `-f <format>` | <a id="cli-log-format"></a>format for log-output; possible values are `default`, `json`, `text` (default: `default`)                                                        |
| `--log-level <level>`   | `-l <level>`  | <a id="cli-log-level"></a>log-level; possible values are `debug`, `info`, `warn` or `error` (default: `info`)                                                               |
| `--log-output <output>` | `-o <output>` | <a id="cli-log-output"></a>output-target for logs; possible values are `stderr`, `stdout` or `<path-to-logfile>` (default: `stderr`)                                        |
| `--help,`               | `-h`          | show help                                                                                                                                                                   |
| `--version`             | `-v`          | prints version-information and exists                                                                                                                                       |

### Structured Logging

Vault Raft Snapshot Agent uses go's [slog package](https://pkg.go.dev/log/slog) to provide structured logging
capabilities.
Log format `text` uses [TextHandler](https://pkg.go.dev/log/slog#TextHandler), `json`
uses [JSONHandler](https://pkg.go.dev/log/slog#JSONHandler).
If no log format or `default` is specified the default log format is used which outputs the timestamp followed by the
message followed by additional key=value-pairs if any are present.

## Environment variables

You can specify most [command-line options](#command-line-options-and-logging) via environment-variables:

| Environment variable       | Corresponding command-line-option |
| -------------------------- | --------------------------------- |
| `VRSA_CONFIG_FILE=<file>`  | [--config-file](#cli-config)      |
| `VRSA_LOG_FORMAT=<format>` | [--log-format](#cli-log-format)   |
| `VRSA_LOG_LEVEL=<level>`   | [--log-level](#cli-log-level)     |
| `VRSA_LOG_OUTPUT=<output>` | [--log-output](#cli-log-output)   |

Vault Raft Snapshot Agent supports static configuration via environment variables:

- for setting the [address of the vault-server](#cnf-vault-url) you can use `VAULT_ADDR`.
- any other [configuration option](#configuration) can be set by prefixing `VRSA_` to the upper-cased path to the key  
  and replacing `.` with `_`.
  For example `VRSA_SNAPSHOTS_FREQUENCY=<value>` configures the [snapshot-frequency](#cnf-snapshots-frequency) and
  `VRSA_VAULT_AUTH_TOKEN=<value>` configures the [token authentication](#cnf-vault-auth-token) for vault.

Other than the [external property sources](#secrets-and-external-property-sources), these environment variables are read
once at startup only and the
configuration will not be reloaded when their values change.

**Options specified via environment-variables take precedence before the values specified in the configuration file -
even those specified as [external property sources](#secrets-and-external-property-sources)!**

## Configuration

Vault Raft Snapshot Agent uses [viper](https://github.com/spf13/viper) as configuration-backend, so you can write your
configuration in either json, yaml or toml.

The Agent monitors the configuration-file for changes and reloads the configuration automatically when the file changes.

#### Example configuration (yaml)

```
vault:
  # Url of the (leading) vault-server
  url: https://vault-server:8200
  auth:
    # configures kubernetes auth
    kubernetes:
      role: "test-role"
snapshots:
  # configures how often snapshots are made, default 1h
  frequency: "4h"
  # configures how many snapshots are retained, default 0
  retain: 10
  storages:
    # configures local storage of snapshots
    local:
      path: /snapshots
```

(for a complete example with all configuration-options see [complete.yaml](./testdata/complete.yaml))

### Secrets and external property-sources

Vault Raft Snapshot allows you to specify dynamic sources for properties containing secrets which you either do not want
to write into the configuration file or which might change while the agent is running. For these properties you may
specify either an environment variable as source using `env://<variable-name>` or a file-source containing the value for
the secret using `file://<file-path>`, where `<file-path>` may be either an absolute path or a path relative to the
configuration file. Any value not prefixed with `env://` or `file://` will be used as is.

**Dynamic properties are validated at startup only, so if e.g. you delete the source-file for a property required to
authenticate with vault or connect to a remote storage while the agent is running, the next login to vault or upload
to that storage will fail (gracefully)!**

### Vault configuration

```
vault:
  url: <http(s)-url to vault-server>
  insecure: <true|false>
  timeout: <duration>
```

| Key                             | Type                                                   | Required/*Default*       | Description                                                                                                          |
| ------------------------------- | ------------------------------------------------------ | ------------------------ | -------------------------------------------------------------------------------------------------------------------- |
| <a id="cnf-vault-url"></a>`url` | URL                                                    | *https://127.0.0.1:8200* | specifies the url of the vault-server                                                                                |
| `insecure`                      | Boolean                                                | *false*                  | specifies whether insecure https connections are allowed or not. Set to `true` when you use self-signed certificates |
| `timeout`                       | [Duration](https://golang.org/pkg/time/#ParseDuration) | *60s*                    | timeout for the vault-http-client; increase for large raft databases (and increase `snapshots.timeout` accordingly!) |

**`vault.url` should point to the cluster-leader, otherwise no snapshots get taken until the server the url points to is
elected leader!** When running Vault on Kubernetes installed by
the [default helm-chart](https://developer.hashicorp.com/vault/docs/platform/k8s/helm), this should be
`http(s)://vault-active.<vault-namespace>.svc.cluster.local:<vault-server service-port>`.|

### Vault authentication

To allow Vault Raft Snapshot Agent to take snapshots, you must add a policy that allows read-access to the
snapshot-apis. This involves the following:

1. `vault login` with an admin user.
2. Create the following policy `vault policy write snapshots ./my_policies/snapshots.hcl` where `snapshots.hcl` is:

```hcl
path "/sys/storage/raft/snapshot"
{
  capabilities = ["read"]
}
```

The above policy is the minimum required policy to be able to generate snapshots. This policy must be associated with
the app- or kubernetes-role you specify in you're configuration (see below).

Only one of the following authentication options should be specified. If multiple options are specified *one* of them is
used with the following priority: `approle`, `aws`, `azure`, `gcp`,
`kubernetes`, `ldap`,  `token`, `userpass`. If no option is specified, Vault Raft Snapshot Agent tries to access vault
unauthenticated (which should fail outside of test- or develop-environments)

Vault Raft Snapshot Agent automatically renews the authentication when it expires.

#### AppRole authentication

Authentication via AppRole (see [the Vault docs](https://www.vaultproject.io/docs/auth/approle))

##### Minimal configuration

```
vault:
  auth:
    approle:
      role: "<role-id>"
      secret: "<secret-id>"
```

##### Configuration options

| Key      | Type                                             | Required/*Default* | Description                                                                          |
| -------- | ------------------------------------------------ | ------------------ | ------------------------------------------------------------------------------------ |
| `role`   | [Secret](#secrets-and-external-property-sources) | **required**       | specifies the role_id used to call the Vault API. See the authentication steps below |
| `secret` | [Secret](#secrets-and-external-property-sources) | **required**       | specifies the secret_id used to call the Vault API.                                  |
| `path`   | String                                           | *approle*          | specifies the backend-name used to select the login-endpoint (`auth/<path>/login`)   |

To allow the App-Role access to the snapshots you should run the following commands on your vault-cluster:

```
vault write auth/<path>/role/snapshot token_policies=snapshots
vault read auth/<path>/role/snapshot/role-id
# prints role-id and meta-data
vault write -f auth/<path>/role/snapshot/secret-id
# prints the secret id and it's metadata
```

#### AWS authentication

Uses AWS for authentication (see the [Vault docs](https://developer.hashicorp.com/vault/docs/auth/aws)).

##### Minimal configuration

```
vault:
  auth:
    aws:
      role: "<role>"
```

##### Configuration options

| Key                 | Type                                             | Required/*Default*         | Description                                                                                          |
| ------------------- | ------------------------------------------------ | -------------------------- | ---------------------------------------------------------------------------------------------------- |
| `role`              | String                                           | **required**               | specifies the role used to call the Vault API. See the authentication steps below                    |
| `ec2Nonce`          | [Secret](#secrets-and-external-property-sources) |                            | enables EC2 authentication and sets the required nonce                                               |
| `ec2SignatureType`  | String                                           | *pkcs7*                    | changes the signature-type for EC2 authentication; valid values are `identity`, `pkcs7` and `rs2048` |
| `iamServerIdHeader` | String                                           |                            | specifies the server-id-header when using IAM authentication type                                    |
| `region`            | [Secret](#secrets-and-external-property-sources) | *env://AWS_DEFAULT_REGION* | specifies the aws region to use.                                                                     |
| `path`              | String                                           | *aws*                      | specifies the backend-name used to select the login-endpoint (`auth/<path>/login`)                   |

AWS authentication uses the IAM authentication type by default unless `ec2Nonce` is set. *The credentials for IAM
authentication **must** be
provided via environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`
or `AWS_SHARED_CREDENTIALS_FILE`;
`AWS_SHARED_CREDENTIALS_FILE` must be specified as an absolute path).*

To allow the access to the snapshots you should run the following commands on your vault-cluster:

```
# for AWS EC2 authentication
vault write auth/<path>/role/<role> auth_type=ec2 bound_ami_id=<ami-id> policies=snapshots max_ttl=500h

# for IAM authentication
vault write auth/<path>/role/<role> auth_type=iam bound_iam_principal_arn=<princial-arn> policies=snapshots max_ttl=500h
```

#### Azure authentication

Authentication using Azure (see [the Vault docs](https://developer.hashicorp.com/vault/docs/auth/azure)).

##### Minimal configuration

```
vault:
  auth:
    azure:
      role: "<role-id>"
```

##### Configuration options

| Key        | Type   | Required/*Default* | Description                                                                        |
| ---------- | ------ | ------------------ | ---------------------------------------------------------------------------------- |
| `role`     | String | **required**       | specifies the role used to call the Vault API. See the authentication steps below  |
| `resource` | String |                    | optional azure resource                                                            |
| `path`     | String | *azure*            | specifies the backend-name used to select the login-endpoint (`auth/<path>/login`) |

To allow the access to the snapshots you should run the following commands on your vault-cluster:

```
vault write auth/<path>/role/<role> \
    policies="snapshots" \
    bound_subscription_ids=<subscription-ids> \
    bound_resource_groups=<resource-group>
```

#### Google Cloud authentication

Authentication using Google Cloud GCE or IAM authentication (
see [the Vault docs](https://developer.hashicorp.com/vault/docs/auth/gcp)).

##### Minimal configuration

```
vault:
  auth:
    gcp:
      role: "<role>"
```

##### Configuration options

| Key                   | Type   | Required/*Default* | Description                                                                        |
| --------------------- | ------ | ------------------ | ---------------------------------------------------------------------------------- |
| `role`                | String | **required**       | specifies the role used to call the Vault API. See the authentication steps below  |
| `serviceAccountEmail` | String |                    | activates IAM authentication and specifies the service-account to use              |
| `path`                | String | *gcp*              | specifies the backend-name used to select the login-endpoint (`auth/<path>/login`) |

Google Cloud authentication uses the GCE authentication type by default unless `serviceAccountEmail` is set.

To allow the access to the snapshots you should run the following commands on your vault-cluster:

```
# for IAM authentication type
vault write auth/<path>/role/<role> \
    type="iam" \
    policies="snapshots" \
    bound_service_accounts="<service-account-email>"

# for GCE authentication type
vault write auth/<path>/role/<role> \
    type="gce" \
    policies="snapshots" \
    bound_projects="<projects>" \
    bound_zones="<zones>" \
    bound_labels="<labels>" \
    bound_service_accounts="<service-acoount-email>"
```

#### Kubernetes authentication

To enable Kubernetes authentication mode, you should follow the steps
from [the Vault docs](https://www.vaultproject.io/docs/auth/kubernetes#configuration)
and create the appropriate policies and roles.

##### Minimal configuration

```
vault:
  auth:
    kubernetes:
      role: "test"
```

##### Configuration options

| Key        | Type                                             | Required/*Default*                                           | Description                                                                                     |
| ---------- | ------------------------------------------------ | ------------------------------------------------------------ | ----------------------------------------------------------------------------------------------- |
| `role`     | String                                           | **required**                                                 | specifies vault k8s auth role                                                                   |
| `jwtToken` | [Secret](#secrets-and-external-property-sources) | *file:///var/run/secrets/kubernetes.io/serviceaccount/token* | specifies the JWT-Token for the kubernetes service-account, *must resolve to a non-empty value* |
| `path`     | String                                           | *kubernetes*                                                 | specifies the backend-name used to select the login-endpoint (`auth/<path>/login`)              |

To allow kubernetes access to the snapshots you should run the following commands on your vault-cluster:

```
  kubectl -n <your-vault-namespace> exec -it <vault-pod-name> -- vault write auth/<path>/role/<kubernetes.role> bound_service_account_names=*  bound_service_account_namespaces=<namespace of your vault-raft-snapshot-agent-pod> policies=snapshots ttl=24h
```

Depending on your setup you can restrict access to specific service-account-names and/or namespaces.

#### LDAP authentication

Authentication using LDAP (see [the Vault docs](https://developer.hashicorp.com/vault/docs/auth/ldap)).

##### Minimal configuration

```
vault:
  auth:
    ldap:
      role: "test"
```

##### Configuration options

| Key        | Type                                             | Required/*Default* | Description                                                                        |
| ---------- | ------------------------------------------------ | ------------------ | ---------------------------------------------------------------------------------- |
| `username` | [Secret](#secrets-and-external-property-sources) | **required**       | the username                                                                       |
| `password` | [Secret](#secrets-and-external-property-sources) | **required**       | the password                                                                       |
| `path`     | String                                           | *ldap*             | specifies the backend-name used to select the login-endpoint (`auth/<path>/login`) |

To allow access to the snapshots you should run the following commands on your vault-cluster:

```
# allow access for a specific user
vault write auth/<path>/users/<username> policies=snapshot

# allow access based on group
vault write auth/<path>/groups/<group> policies=snapshots
```

#### Token authentication

##### Minimal configuration

```
vault:
  auth:
    token: <token>
```

##### Configuration options

| Key                                      | Type                                             | Required/*Default* | Description                        |
| ---------------------------------------- | ------------------------------------------------ | ------------------ | ---------------------------------- |
| <a id="cnf-vault-auth-token"></a>`token` | [Secret](#secrets-and-external-property-sources) | **required**       | specifies the token used to log in |

#### User and Password authentication

Authentication using username and password (
see [the Vault docs](https://developer.hashicorp.com/vault/docs/auth/userpass)).

##### Minimal configuration

```
vault:
  auth:
    userpass:
      username: "<username>"
      password: "<password>"
```

##### Configuration options

| Key        | Type                                             | Required/*Default* | Description                                                                        |
| ---------- | ------------------------------------------------ | ------------------ | ---------------------------------------------------------------------------------- |
| `username` | [Secret](#secrets-and-external-property-sources) | **required**       | the username                                                                       |
| `password` | [Secret](#secrets-and-external-property-sources) | **required**       | the password                                                                       |
| `path`     | String                                           | *userpass*         | specifies the backend-name used to select the login-endpoint (`auth/<path>/login`) |

To allow access to the snapshots you should run the following commands on your vault-cluster:

```
vault write auth/<path>/users/<username> \
    password=<password> \
    policies=snapshots
```

### Snapshot configuration

```
snapshots:
  frequency: <duration>
  timeout: <duration>
  retain: <int>
  namePrefix: <prefix>
  nameSuffix: <suffix>
  timestampFormat: <format>
```

#### Configuration options

| Key                                             | Type                                                                  | Required/*Default*          | Description                                                                                                                                                             |
| ----------------------------------------------- | --------------------------------------------------------------------- | --------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| <a id="cnf-snapshots-frequency"></a>`frequency` | [Duration](https://golang.org/pkg/time/#ParseDuration)                | *1h*                        | how often to run the snapshot agent                                                                                                                                     |
| `retain`                                        | Integer                                                               | *0*                         | the number of snapshots to retain. For example, if you set `retain: 2`, the two most recent snapshots will be kept in storage. `0` means all snapshots will be retained |
| `timeout`                                       | [Duration](https://golang.org/pkg/time/#ParseDuration)                | *60s*                       | timeout for creating snapshots                                                                                                                                          |
| `namePrefix`                                    | String                                                                | *raft-snapshot-*            | prefix of the uploaded snapshots                                                                                                                                        |
| `nameSuffix`                                    | String                                                                | *.snap*                     | suffix/extension of the uploaded snapshots                                                                                                                              |
| `timestampFormat`                               | [Go Time.Format Layout-String]((https://pkg.go.dev/time#Time.Format)) | *2006-01-02T15-04-05Z-0700* | timestamp-format for the uploaded snapshots' timestamp; you can test your layout-string at the [Go Playground](https://go.dev/play/p/PxX7LmcPha0)                       |

The name of the snapshots is created by concatenating `namePrefix`, the timestamp formatted according
to `timestampFormat` and `nameSuffix`, e.g. the defaults would generate
`raft-snapshot-2023-09-01T15-30-00Z+0200.snap` for a snapshot taken at 15:30:00 on 09/01/2023 when the timezone is
CEST (GMT + 2h).

These options can be overridden for a specific storage:

```
snapshots:
  frequency: 1h
  retain: 24
  storages:
    local:
      path: /snapshots
    aws:
      frequency: 24h
      retain: 365
      timestampFormat: 2006-01-02
      #...
```

In this example the agent would take and store a snapshot to the local-storage every hour, retaining 24 snapshots and
store a daily snapshot on aws remote storage, retaining the last 365 snapshots with a appropriate shorter timestamp.

*Note: as the agent uses the default frequency in case of failures, you should always configure the shorter frequency in
the defaults and specify longer frequencies for specific storages if required!*

### Storage configuration

Note that if you specify more than one storage option, *all* specified storages will be written to. For example,
specifying `local` and `aws` will write to both locations.
When using multiple remote storages, increase the timeout allowed via `snapahots.timeout` for larger raft databases.
Each option can be specified exactly once;
it is currently not possible to e.g. upload to multiple aws regions by specifying multiple `aws`-storage-options.

#### AWS S3 Storage

Uploads snapshots to AWS` S3. This storage uses
the [AWS Go SDK](https://pkg.go.dev/github.com/aws/aws-sdk-go/service/s3). Use this storage for AWS-hosted S3 services that use an AWS S3-API compatible addressing-scheme (e.g. https://<bucket>.<endpoint>). For other S3 implementations, try the [generic s3 storage](#genericminio-s3-storage)

##### Minimal Configuration

```
snapshots:
  storage
    aws:
      bucket: <bucket>
```

##### Configuration Options

| Key                       | Type                                             | Required/*Default*            | Description                                                                                                       |
| ------------------------- | ------------------------------------------------ | ----------------------------- | ----------------------------------------------------------------------------------------------------------------- |
| `bucket`                  | String                                           | **required**                  | bucket to store snapshots in                                                                                      |
| `accessKeyId`             | [Secret](#secrets-and-external-property-sources) | *env://AWS_ACCESS_KEY_ID*     | specifies the access key                                                                                          |
| `accessKey`               | [Secret](#secrets-and-external-property-sources) | *env://AWS_SECRET_ACCESS_KEY* | specifies the secret access key; **must resolve to non-empty value if accessKeyId resolves to a non-empty value** |
| `sessionToken`            | [Secret](#secrets-and-external-property-sources) | *env://AWS_SESSION_TOKEN*     | specifies the session token                                                                                       |
| `region`                  | [Secret](#secrets-and-external-property-sources) | *env://AWS_DEFAULT_REGION*    | S3 region if it is required                                                                                       |
| `keyPrefix`               | String                                           |                               | prefix to store s3 snapshots in                                                                                   |
| `endpoint`                | [Secret](#secrets-and-external-property-sources) | *env://AWS_ENDPOINT_URL*      | S3 compatible storage endpoint (ex: http://127.0.0.1:9000)                                                        |
| `useServerSideEncryption` | Boolean                                          | *false*                       | Set to true to turn on AWS' AES256 encryption. Support for AWS KMS keys is not currently supported                |
| `forcePathStyle`          | Boolean                                          | *false*                       | needed if your S3 Compatible storage supports only path-style, or you would like to use S3's FIPS Endpoint        |

Any common [snapshot configuration option](#snapshot-configuration) overrides the global snapshot-configuration.

#### Azure Storage

##### Minimal Configuration

```
snapshots:
  storages:
    azure:
      container: <container>
```

##### Configuration Options

| Key           | Type                                             | Required/*Default*            | Description                                                                  |
| ------------- | ------------------------------------------------ | ----------------------------- | ---------------------------------------------------------------------------- |
| `container`   | String                                           | **required**                  | the name of the blob container to write to                                   |
| `accountName` | [Secret](#secrets-and-external-property-sources) | *env://AZURE_STORAGE_ACCOUNT* | the account name of the storage account; **must resolve to non-empty value** |
| `accountKey`  | [Secret](#secrets-and-external-property-sources) | *env://AZURE_STORAGE_KEY*     | the account key of the storage account; **must resolve to non-empty value**  |
| `cloudDomain` | String                                           | *blob.core.windows.net*       | domain of the cloud-service to use                                           |

Any common [snapshot configuration option](#snapshot-configuration) overrides the global snapshot-configuration.

#### Google Cloud Storage

##### Minimal Configuration

```
snapshots:
  storages:
    gcp:
      bucket: <bucket>
```

##### Configuration Options

| Key      | Type   | Required/*Default* | Description                                                                               |
| -------- | ------ | ------------------ | ----------------------------------------------------------------------------------------- |
| `bucket` | String | **required**       | the Google Storage Bucket to write to. Auth is expected to be default machine credentials |

Any option common [snapshot configuration option](#snapshot-configuration) overrides the global snapshot-configuration.

#### Local Storage

##### Minimal Configuration

```
snapshots:
  storages:
    local:
      path: <path>
```

##### Configuration Options

| Key    | Type   | Required/*Default* | Description                                                                                                     |
| ------ | ------ | ------------------ | --------------------------------------------------------------------------------------------------------------- |
| `path` | String | **required**       | fully qualified path, not including file name, for where the snapshot should be written. i.e. `/raft/snapshots` |

Any common [snapshot configuration option](#snapshot-configuration) overrides the global snapshot-configuration.

#### Openstack Swift Storage

##### Minimal Configuration

```
snapshots:
  storages:
    swift:
      container: <container>
      authUrl: <auth-url>
```

| Key         | Type                                                   | Required/*Default*     | Description                                                               |
| ----------- | ------------------------------------------------------ | ---------------------- | ------------------------------------------------------------------------- |
| `container` | String                                                 | **required**           | the name of the container to write to                                     |
| `authUrl`   | URL                                                    | **required**           | the auth-url to authenticate against                                      |
| `username`  | [Secret](#secrets-and-external-property-sources)       | *env://SWIFT_USERNAME* | the username used for authentication; **must resolve to non-empty value** |
| `apiKey`    | [Secret](#secrets-and-external-property-sources)       | *env://SWIFT_API_KEY*  | the api-key used for authentication; **must resolve to non-empty value**  |
| `region`    | [Secret](#secrets-and-external-property-sources)       | *env://SWIFT_REGION*   | optional region to use eg "LON", "ORD"                                    |
| `domain`    | URL                                                    |                        | optional user's domain name                                               |
| `tenantId`  | String                                                 |                        | optional id of the tenant                                                 |
| `timeout`   | [Duration](https://golang.org/pkg/time/#ParseDuration) | *60s*                  | timeout for snapshot-uploads                                              |

Any common [snapshot configuration option](#snapshot-configuration) overrides the global snapshot-configuration.

#### Generic/MinIO S3 Storage

Uploads snapshots to any S3-compatible server. This storage uses
the [MinIO Go Client SDK](https://github.com/minio/minio-go). If your self-hosted S3-server does not support the default adressing-scheme of [AWS S3](#aws-s3-storage), then this storage might still work.

##### Minimal Configuration

```
snapshots:
  storage
    s3:
      endpoint: <endpoint>
      bucket: <bucket>
```

##### Configuration Options

| Key            | Type                                             | Required/*Default*           | Description                                                                                                       |
| -------------- | ------------------------------------------------ | ---------------------------- | ----------------------------------------------------------------------------------------------------------------- |
| `endpoint`     | String                                           | **required**                 | S3 compatible storage endpoint (ex: my-storage.example.com)                                                       |
| `bucket`       | String                                           | **required**                 | bucket to store snapshots in                                                                                      |
| `accessKeyId`  | [Secret](#secrets-and-external-property-sources) | *env://S3_ACCESS_KEY_ID*     | specifies the access key                                                                                          |
| `accessKey`    | [Secret](#secrets-and-external-property-sources) | *env://S3_SECRET_ACCESS_KEY* | specifies the secret access key; **must resolve to non-empty value if accessKeyId resolves to a non-empty value** |
| `sessionToken` | [Secret](#secrets-and-external-property-sources) | *env://S3_SESSION_TOKEN*     | specifies the session token                                                                                       |
| `region`       | [Secret](#secrets-and-external-property-sources) |                              | S3 region if it is required                                                                                       |
| `insecure`     | Boolean                                          | *false*                      | whether to connect using https (false) or not                                                                     |

Any common [snapshot configuration option](#snapshot-configuration) overrides the global snapshot-configuration.

## License

- Source code is licensed under MIT

## Contributors

- Vault Raft Snapshot Agent was originally developed
  by [@Lucretius](https://github.com/Lucretius/vault_raft_snapshot_agent/)
- contains improvements done by [@F21](https://github.com/Boostport/vault_raft_snapshot_agent/)
- enhancements for azure-uploader by [@vikramhansawat](https://github.com/Lucretius/vault_raft_snapshot_agent/pull/26)
- support for additional authentication methods based on code
  from [@alexeiser](https://github.com/Lucretius/vault_raft_snapshot_agent/pull/25)
- support for Openstack Swift Storage based on code
  from [@Pyjou](https://github.com/Lucretius/vault_raft_snapshot_agent/pull/19)
