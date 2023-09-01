package upload

import (
	"context"
	"fmt"
	"io"
	"slices"
	"time"
)

type UploadersConfig struct {
	AWS   AWSConfig   `default:"{\"Empty\": true}" mapstructure:"aws"`
	Azure AzureConfig `default:"{\"Empty\": true}" mapstructure:"azure"`
	GCP   GCPConfig   `default:"{\"Empty\": true}" mapstructure:"google"`
	Local LocalConfig `default:"{\"Empty\": true}" mapstructure:"local"`
}

func (c UploadersConfig) HasUploaders() bool {
	return !(c.AWS.Empty && c.Azure.Empty && c.GCP.Empty && c.Local.Empty)
}

type Uploader interface {
	Destination() string
	Upload(ctx context.Context, reader io.Reader, time time.Time, retain int) error
}

func CreateUploaders(config UploadersConfig) ([]Uploader, error) {
	var uploaders []Uploader

	if !config.AWS.Empty {
		aws, err := createAWSUploader(config.AWS)
		if err != nil {
			return nil, err
		}
		uploaders = append(uploaders, aws)
	}

	if !config.Azure.Empty {
		azure, err := createAzureUploader(config.Azure)
		if err != nil {
			return nil, err
		}
		uploaders = append(uploaders, azure)
	}

	if !config.GCP.Empty {
		gcp, err := createGCPUploader(config.GCP)
		if err != nil {
			return nil, err
		}
		uploaders = append(uploaders, gcp)
	}

	if !config.Local.Empty {
		local, err := createLocalUploader(config.Local)
		if err != nil {
			return nil, err
		}
		uploaders = append(uploaders, local)
	}

	return uploaders, nil
}

const (
	snapshotFileName string = "raft_snapshot-"
	snapshotFileExt  string = ".snap"
)

type uploaderImpl[T any] interface {
	uploadSnapshot(ctx context.Context, name string, data io.Reader) error
	deleteSnapshot(ctx context.Context, snapshot T) error
	listSnapshots(ctx context.Context, prefix string, ext string) ([]T, error)
	compareSnapshots(a, b T) int
}

type uploader[T any] struct {
	impl uploaderImpl[T]
}

func (u uploader[T]) Destination() string {
	return ""
}

func (u uploader[T]) Upload(ctx context.Context, reader io.Reader, time time.Time, retain int) error {
	name := fmt.Sprintf("%s-%d%s", snapshotFileName, time.UnixNano(), snapshotFileExt)
	if err := u.impl.uploadSnapshot(ctx, name, reader); err != nil {
		return fmt.Errorf("error uploading snapshot to %s: %w", u.Destination(), err)
	}

	if retain > 0 {
		return u.deleteSnapshots(ctx, retain)
	}

	return nil
}

func (u uploader[T]) deleteSnapshots(ctx context.Context, retain int) error {
	snapshots, err := u.impl.listSnapshots(ctx, snapshotFileName, snapshotFileExt)
	if err != nil {
		return fmt.Errorf("error getting snapshots from %s: %w", u.Destination(), err)
	}

	if len(snapshots) > retain {
		slices.SortFunc(snapshots, func(a, b T) int { return u.impl.compareSnapshots(a, b) * 1 })

		for _, s := range snapshots[retain:] {
			if err := u.impl.deleteSnapshot(ctx, s); err != nil {
				return fmt.Errorf("error deleting snapshot from %s: %w", u.Destination(), err)
			}
		}
	}
	return nil
}
