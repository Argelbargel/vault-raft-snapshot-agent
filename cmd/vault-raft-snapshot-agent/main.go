/*
Vault Raft Snapshot Agent periodically takes snapshots of Vault's raft database.
It uploads those snaphots to one or more storage locations like a local harddrive
or an AWS S3 Bucket.

Usage:

	vault-raft-snapshot-agent [flags] [options]

The flags are:

	-v, -version
		Prints version information and exits

The options are:

	-c -config <file>
		Specifies the config-file to use.

	-f -log-format [default|json|text]
		Specifies the log-format (default: default)

	-l -log-level [debug|info|warn|error]
		Specifies the log-level (default: info)

	-o -log-output [stderr|stdout|<file>]
		Specifies the output to log to (default: stderr)

If no config file is explicitly specified, the program looks for configuration-files
with the name `snapshot` and the extensions supported by [viper]
in the current working directory or in /etc/vault.d/snapshots.

For details on how to configure the program see the [README]

[viper]: https://github.com/spf13/viper
[README]: https://github.com/Argelbargel/vault-raft-snapshot-agent/README.md
*/
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/urfave/cli/v2"

	internal "github.com/Argelbargel/vault-raft-snapshot-agent/internal/app/vault_raft_snapshot_agent"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/app/vault_raft_snapshot_agent/logging"
)

var Version = "development"
var Platform = "linux/amd64"

var snapshotterOptions = internal.SnapshotterOptions{
	ConfigFileName:        "snapshots",
	ConfigFileSearchPaths: []string{"/etc/vault.d/", "."},
	EnvPrefix:             "VRSA",
}

const (
	optionConfig    = "config"
	optionLogFormat = "log-format"
	optionLogOutput = "log-output"
	optionLogLevel  = "log-level"
)

type quietBoolFlag struct {
	cli.BoolFlag
}

func (qbf *quietBoolFlag) String() string {
	return cli.FlagStringer(qbf)
}

func (qbf *quietBoolFlag) GetDefaultText() string {
	return ""
}

func main() {
	cli.VersionPrinter = func(ctx *cli.Context) {
		fmt.Printf("%s (%s), version: %s\n", ctx.App.Name, Platform, ctx.App.Version)
	}

	cli.VersionFlag = &quietBoolFlag{
		cli.BoolFlag{
			Name:    "version",
			Aliases: []string{"v"},
			Usage:   "prints version-information and exists",
		},
	}

	app := &cli.App{
		Name:        "vault-raft-snapshot-agent",
		Version:     Version,
		Description: "takes periodic snapshot of vault's raft-db",
		Flags: []cli.Flag{
			&cli.PathFlag{
				Name:    optionConfig,
				Aliases: []string{"c"},
				Usage:   fmt.Sprintf("load configuration from `FILE`; if not specified, searches for %s.[json|toml|yaml] in /etc/vault.d or the current working directory", snapshotterOptions.ConfigFileName),
				EnvVars: []string{snapshotterOptions.EnvPrefix + "_CONFIG_FILE"},
			},
			&cli.StringFlag{
				Name:    optionLogFormat,
				Aliases: []string{"f"},
				Usage:   "format for log-output; possible values are 'default', 'text', 'json'",
				EnvVars: []string{snapshotterOptions.EnvPrefix + "_LOG_FORMAT"},
				Value:   logging.FormatDefault,
			},
			&cli.StringFlag{
				Name:    optionLogOutput,
				Aliases: []string{"o"},
				Usage:   "output-target for logs; possible values are 'stderr', 'stdout' or <path-to-logfile>",
				EnvVars: []string{snapshotterOptions.EnvPrefix + "_LOG_OUTPUT"},
				Value:   logging.OutputStderr,
			},
			&cli.StringFlag{
				Name:    optionLogLevel,
				Aliases: []string{"l"},
				Usage:   "log-level for logs; possible values are 'debug', 'info', 'warn' or 'error'",
				EnvVars: []string{snapshotterOptions.EnvPrefix + "_LOG_LEVEL"},
				Value:   logging.LevelInfo,
			},
		},
		Action: func(ctx *cli.Context) error {
			err := logging.Configure(ctx.String(optionLogOutput), ctx.String(optionLogFormat), ctx.String(optionLogLevel))
			if err != nil {
				log.Fatalf("could not configure logging: %s", err)
			}
			return startSnapshotter(ctx.Path(optionConfig))
		},
	}
	app.CustomAppHelpTemplate = `Usage: {{.HelpName}} [options]
{{.Description}}

Options:
{{range $index, $option := .VisibleFlags}}{{if $index}}
{{end}}{{$option}}{{end}}`

	if err := app.Run(os.Args); err != nil {
		logging.Fatal("Could not start agent", "error", err)
	}
}

func startSnapshotter(configFile cli.Path) error {
	snapshotterOptions.ConfigFilePath = configFile
	snapshotter, err := internal.CreateSnapshotter(snapshotterOptions)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		cancel()
	}()

	runSnapshotter(ctx, snapshotter)
	return nil
}

func runSnapshotter(ctx context.Context, snapshotter *internal.Snapshotter) {
	for {
		timeout, _ := snapshotter.TakeSnapshot(ctx)
		select {
		case <-timeout.C:
			continue
		case <-ctx.Done():
			os.Exit(0)
		}
	}
}
