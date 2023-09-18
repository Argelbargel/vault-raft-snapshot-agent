package upload

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"
)

type UploadersConfig struct {
	AWS   AWSUploaderConfig   `default:"{\"Empty\": true}"`
	Azure AzureUploaderConfig `default:"{\"Empty\": true}"`
	GCP   GCPUploaderConfig   `default:"{\"Empty\": true}"`
	Local LocalUploaderConfig `default:"{\"Empty\": true}"`
	Swift SwiftUploaderConfig `default:"{\"Empty\": true}"`
}

type Uploader interface {
	Destination() string
	Upload(ctx context.Context, snapshot io.Reader, prefix string, timestamp string, suffix string, retain int) error
}

func CreateUploaders(config UploadersConfig) ([]Uploader, error) {
	var uploaders []Uploader

	if !config.AWS.Empty {
		uploaders = append(uploaders, createAWSUploader(config.AWS))
	}

	if !config.Azure.Empty {
		uploaders = append(uploaders, createAzureUploader(config.Azure))
	}

	if !config.GCP.Empty {
		uploaders = append(uploaders, createGCPUploader(config.GCP))
	}

	if !config.Local.Empty {
		uploaders = append(uploaders, createLocalUploader(config.Local))
	}

	if !config.Swift.Empty {
		uploaders = append(uploaders, createSwiftUploader(config.Swift))
	}

	return uploaders, nil
}

type uploaderImpl[CONF any, CLIENT any, OBJ any] interface {
	connect(ctx context.Context, config CONF) (CLIENT, error)
	destination(config CONF) string
	uploadSnapshot(ctx context.Context, client CLIENT, name string, data io.Reader) error
	deleteSnapshot(ctx context.Context, client CLIENT, snapshot OBJ) error
	listSnapshots(ctx context.Context, client CLIENT, prefix string, ext string) ([]OBJ, error)
	compareSnapshots(a, b OBJ) int
}

type uploader[CONF any, CLIENT any, OBJ any] struct {
	config CONF
	impl   uploaderImpl[CONF, CLIENT, OBJ]
}

func (u uploader[CNF, CLI, O]) Destination() string {
	return u.impl.destination(u.config)
}

func (u uploader[CNF, CLI, O]) Upload(ctx context.Context, snapshot io.Reader, prefix string, timestamp string, suffix string, retain int) error {
	client, err := u.impl.connect(ctx, u.config)
	if err != nil {
		return fmt.Errorf("could not connect to %s: %s", u.Destination(), err)
	}

	name := strings.Join([]string{prefix, timestamp, suffix}, "")
	if err := u.impl.uploadSnapshot(ctx, client, name, snapshot); err != nil {
		return fmt.Errorf("error uploading snapshot to %s: %w", u.Destination(), err)
	}

	if retain > 0 {
		return u.deleteSnapshots(ctx, client, prefix, suffix, retain)
	}

	return nil
}

func (u uploader[CNF, CLI, O]) deleteSnapshots(ctx context.Context, client CLI, prefix string, suffix string, retain int) error {
	snapshots, err := u.impl.listSnapshots(ctx, client, prefix, suffix)
	if err != nil {
		return fmt.Errorf("error getting snapshots from %s: %w", u.Destination(), err)
	}

	if len(snapshots) > retain {
		slices.SortFunc(snapshots, func(a, b O) int { return u.impl.compareSnapshots(a, b) * -1 })

		for _, s := range snapshots[retain:] {
			if err := u.impl.deleteSnapshot(ctx, client, s); err != nil {
				return fmt.Errorf("error deleting snapshot from %s: %w", u.Destination(), err)
			}
		}
	}
	return nil
}
