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
with the name `snapshots` and the extensions supported by [viper]
in the current working directory or in /etc/vault.d/.

For details on how to configure the program see the [README]

[viper]: https://github.com/spf13/viper
[README]: https://github.com/Argelbargel/vault-raft-snapshot-agent/README.md
*/
package main

import (
	"context"
	"fmt"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent"
	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/logging"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/urfave/cli/v2"
)

var Version = "development"
var Platform = "linux/amd64"
var MetricsPort int

var agentOptions = agent.SnapshotAgentOptions{
	ConfigFileName:        "snapshots",
	ConfigFileSearchPaths: []string{"/etc/vault.d/", "."},
	EnvPrefix:             "VRSA",
}

const (
	optionConfig      = "config"
	optionLogFormat   = "log-format"
	optionLogOutput   = "log-output"
	optionLogLevel    = "log-level"
	optionMetricsPort = "metrics-port"
)

var cliFlags = []cli.Flag{
	&cli.PathFlag{
		Name:    optionConfig,
		Aliases: []string{"c"},
		Usage:   fmt.Sprintf("load configuration from `FILE`; if not specified, searches for %s.[json|toml|yaml] in /etc/vault.d or the current working directory", agentOptions.ConfigFileName),
		EnvVars: []string{agentOptions.EnvPrefix + "_CONFIG_FILE"},
	},
	&cli.StringFlag{
		Name:    optionLogFormat,
		Aliases: []string{"f"},
		Usage:   "format for log-output; possible values are 'default', 'text', 'json'",
		EnvVars: []string{agentOptions.EnvPrefix + "_LOG_FORMAT"},
		Value:   logging.FormatDefault,
	},
	&cli.StringFlag{
		Name:    optionLogOutput,
		Aliases: []string{"o"},
		Usage:   "output-target for logs; possible values are 'stderr', 'stdout' or <path-to-logfile>",
		EnvVars: []string{agentOptions.EnvPrefix + "_LOG_OUTPUT"},
		Value:   logging.OutputStderr,
	},
	&cli.StringFlag{
		Name:    optionLogLevel,
		Aliases: []string{"l"},
		Usage:   "log-level for logs; possible values are 'debug', 'info', 'warn' or 'error'",
		EnvVars: []string{agentOptions.EnvPrefix + "_LOG_LEVEL"},
		Value:   logging.LevelInfo,
	},
	&cli.IntFlag{
		Name:        optionMetricsPort,
		Aliases:     []string{"p"},
		Usage:       "Port to serve metrics on",
		EnvVars:     []string{agentOptions.EnvPrefix + "_METRICS_PORT"},
		Value:       2112,
		Destination: &MetricsPort,
	},
}

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
		Flags:       cliFlags,
		Action: func(ctx *cli.Context) error {
			err := logging.Configure(ctx.String(optionLogOutput), ctx.String(optionLogFormat), ctx.String(optionLogLevel))
			if err != nil {
				log.Fatalf("could not configure logging: %s", err)
			}

			agentOptions.ConfigFilePath = ctx.Path(optionConfig)
			return run()
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

func run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		cancel()
	}()

	// serve metrics in a go routine.
	go serveMetrics()
	return runAgent(ctx)
}

func serveMetrics() {
	// serve prometheus metrics
	http.Handle("/metrics", promhttp.Handler())
	err := http.ListenAndServe(fmt.Sprintf(":%d", MetricsPort), nil)
	if err != nil {
		logging.Fatal("failed to setup metrics", "error", err)
	}
}

func runAgent(ctx context.Context) error {
	snapshotAgent, err := agent.CreateSnapshotAgent(ctx, agentOptions)
	if err != nil {
		return err
	}

	for {
		nextSnapshotTicker := snapshotAgent.TakeSnapshot(ctx)
		select {
		case <-ctx.Done():
			os.Exit(0)
		case <-nextSnapshotTicker.C:
			break
		}
	}
}
