[![Release](https://img.shields.io/github/v/release/Argelbargel/vault-raft-snapshot-agent)](https://github.com/Argelbargel/vault-raft-snapshot-agent/releases/latest)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/vault-raft-snapshot-agent)](https://artifacthub.io/packages/search?repo=vault-raft-snapshot-agent)

# Vault Raft Snapshot Agent

Vault Raft Snapshot Agent is a Go binary that is meant to run alongside every member of a [Vault](https://www.vaultproject.io/) cluster and will take periodic snapshots of the Raft database and write it to the desired location.  It's configuration is meant to somewhat parallel that of the [Consul Snapshot Agent](https://www.consul.io/docs/commands/snapshot/agent.html) so many of the same configuration properties you see there will be present here.

## "High Availability" explained
It works in an "HA" way as follows:
1) Each running daemon checks the IP address of the machine its running on.
2) If this IP address matches that of the leader node, it will be responsible for performing snapshotting.
3) The other binaries simply continue checking, on each snapshot interval, to see if they have become the leader.

In this way, the daemon will always run on the leader Raft node.

Another way to do this, which would allow us to run the snapshot agent anywhere, is to simply have the daemons form their own Raft cluster, but this approach seemed much more cumbersome.

## Running

### Container-Image
You can run the agent with the supplied container-image, e.g. via docker:
```
docker run -v <path to snapshot.json>:/etc/vault.d/snapshot.json" ghcr.io/argelbargel/vault-raft-snapshot-agent:latest
```

### Helm-Chart
If you're running on kubernetes, you can use the provided [Helm-Charts](https://argelbargel.github.io/vault-raft-snapshot-agent-helm/) to install Vault Raft Snapshot Agent into your cluster.

### systemd-service
The recommended way of running this daemon is using systemctl, since it handles restarts and failure scenarios quite well.  To learn more about systemctl, checkout [this article](https://www.digitalocean.com/community/tutorials/how-to-use-systemctl-to-manage-systemd-services-and-units).  begin, create the following file at `/etc/systemd/system/snapshot.service`:

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

Your configuration is assumed to exist at `/etc/vault.d/snapshot.json` and the actual daemon binary at `/usr/local/bin/vault-raft-snapshot-agent`.

Then just run:

```
sudo systemctl enable snapshot
sudo systemctl start snapshot
```

If your configuration is right and Vault is running on the same host as the agent you will see one of the following:

`Not running on leader node, skipping.` or `Successfully created <type> snapshot to <location>`, depending on if the daemon runs on the leader's host or not.

## Configuration

The default location of the configuration-file is `/etc/vault.d/snapshots.json`. You may change this path by running the agent with `vault-raft-snapshot-agent --config <path>`.
Vault Raft Snapshot Agent uses [viper]() as configuration-backend so you can write your configuration in either json, yaml or toml.

### Example configuration (yaml)
```
vault:
  # Url of the (leading) vault-server
  url: http://vault-server:8200
  auth:
    # configures kubernetes auth
    kubernetes:
      role: "test-role"
snapshots:
  # configures how often snapshots are made, default 1h
  frequency: "4h"
  # configures how many snapshots are retained, default 0
  retain: 10
uploaders:
  # configures local storage of snapshots
  local:
    path: /snapshots
```

(for a complete example with all configuration-options see [complete.yaml](./testdata/complete.yaml))

### Vault configuration
```
vault:
  url: <http(s)-url to vault-server>
  insecure: <true|false>
```

- `url` specifies the url of the vault-server. Should be the cluster-leader, otherwise no snapshots get taken until the server the url points to is made leader. Defaults to "https://127.0.0.1:8200". Alternativly you can specify the url with the environment-variable `VAULT_ADDR`.
- `insecure` specifies whether insecure https connections are allowed or not. Set to `true` when you use self-signed certificates

#### Vault authentication
```
vault:
  auth:
    # only one of these options should be used!
    approle:
      id: "<approle-id>
      secret: "<approle-secret-id>"
    kubernetes:
      role: "test"
    token: <auth-token>
```

Only one of the following authentication options should be specified. If multiple options are specified *one* of them is used with the following priority: `approle`, `kubernetes`, `token`. If no option is specified, Vault Raft Snapshot Agent tries to access vault unauthenticated (which should fail outside of test- or develop-environments)

To allow Vault Raft Snapshot Agent to take snapshots, you must add a policy that allows read-access to the snapshot-apis. This involves the following:

`vault login` with an admin user.
Create the following policy `vault policy write snapshots ./my_policies/snapshots.hcl` where `snapshots.hcl` is:

```hcl
path "/sys/storage/raft/snapshot"
{
  capabilities = ["read"]
}
```

This policy must be associated with the app- or kubernetes role you specify in you're configuration (see below).

and copy your secret and role ids, and place them into the snapshot file.  The snapshot agent will use them to request client tokens, so that it can interact with your Vault cluster.  The above policy is the minimum required policy to be able to generate snapshots.  The snapshot agent will automatically renew the token when it is going to expire.


##### AppRole authentication (`approle`)
An AppRole allows the snapshot agent to automatically rotate tokens to avoid long-lived credentials. To learn more about AppRole's, see [the Vault docs](https://www.vaultproject.io/docs/auth/approle)

- `id` Specifies the role_id used to call the Vault API.  See the authentication steps below.
- `secret` Specifies the secret_id used to call the Vault API.
- `path` Specifies the backend-name used to select the login-endpoint (`auth/<path>/login`).  Defaults to `approle``.

To allow the App-Role access to the snapshots you should run the following commands on your vault-cluster:
```
vault write auth/approle/role/snapshot token_policies="snapshots"
vault read auth/approle/role/snapshot/<your role-id>
vault write -f auth/approle/role/snapshot/<your secret-id>
```

##### Kubernetes authentication
To enable Kubernetes authentication mode, you should follow the steps from [the Vault docs](https://www.vaultproject.io/docs/auth/kubernetes#configuration) and create the appropriate policies and roles.

- `role` Specifies vault k8s auth role
- `path` Specifies the backend-name used to select the login-endpoint (`auth/<path>/login`).  Defaults to `kubernetes``.
- `jwtPath` Specifies the path to the file with the JWT-Token for the kubernetes Service-Account, Defaults to `/var/run/secrets/kubernetes.io/serviceaccount/token`

To allow kubernetes access to the snapshots you should run the following commands on your vault-cluster:
```
  kubectl -n <your-vault-namespace> exec -it <vault-pod-name> -- vault write auth/<kubernetes.path>/role/<kubernetes.role> bound_service_account_names=*  bound_service_account_namespaces=<namespace of your vault-raft-snapshot-agent-pod> policies=snapshots ttl=24h
```
Depending on your setup you can restrict access to specific service-account-names and/or namespaces.

##### Token authentication
- `token` Specifies the token used to login


### Snapshot configuration
```
snapshots:
  frequency: "<duration>"
  timeout: "<duration>"
  retain: <int>
```

- `frequency` How often to run the snapshot agent.  Examples: `30s`, `1h`.  See https://golang.org/pkg/time/#ParseDuration for a full list of valid time units. Defaults to `1h`
- `retain` The number of backups to retain. Defaults to `0` which means all snapshots will be retained
- `timeout` Timeout for creating snapshots. Examples: `30s`, `1h`. Default: `60s`. See https://golang.org/pkg/time/#ParseDuration for a full list of valid time units., Defaults to `60s`

### Uploader configuration
```
uploaders:
  # you can configure any of these options (exactly once)
  aws:
    endpoint: <endpoint>
    region: <region>
    bucket: <bucket>
    credentials:
      key: <key>
      secret: <secret>
  azure:
    accountName: <name>
    accountKey: <key>
    container: <container>
  google:
    bucket: <bucket>
  local:
    path: <path>
```

Note that if you specify more than one storage option, *all* options will be written to.  For example, specifying `local` and `aws` will write to both locations. Each options can be specified exactly one - thus is is currently not possible to e.g. upload to multiple aws regions by specifying multiple `aws`-entries.

#### AWS S3 Upload
- `endpoint` - S3 compatible storage endpoint (ex: http://127.0.0.1:9000)
- `region` - S3 region as is required for programmatic interaction with AWS
- `bucket` - bucket to store snapshots in (required for AWS writes to work)
- `keyPrefix` - Prefix to store s3 snapshots in.  Defaults to empty string
- `useServerSideEncryption` (`true|false`) -  Encryption is **off** by default. Set to true to turn on AWS' AES256 encryption. Support for AWS KMS keys is not currently supported.
- `forcePathStyle` - Needed if your S3 Compatible storage support only path-style or you would like to use S3's FIPS Endpoint.

##### AWS authentication
```
uploaders:
  aws:
    credentials:
      key: <key>
      secret: <secret>
```
- `key` - specifies the access key. It's recommended to use the standard `AWS_ACCESS_KEY_ID` env var, though
- `secret` - specifies the secret It's recommended to use the standard `SECRET_ACCESS_KEY` env var, though


#### Azure Storage
- `accountName` - The account name of the storage account
- `accountKey` - The account key of the storage account
- `containerName` The name of the blob container to write to

#### Google Storage
`bucket` - The Google Storage Bucket to write to.  Auth is expected to be default machine credentials.

#### Local Storage

`path` - Fully qualified path, not including file name, for where the snapshot should be written.  i.e. `/raft/snapshots`


## License
- Source code is licensed under MIT

## Contributors
- Vault Raft Snapshot Agent was originally developed by [@Lucretius](https://github.com/Lucretius/vault_raft_snapshot_agent/)
- This build contains improvements donne by [@Bootsport](https://github.com/Boostport/vault_raft_snapshot_agent/)
