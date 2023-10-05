package storage

import (
	"time"
)

// StoragesConfig specified the configuration-section for the storages to which snapshots are uploaded
type StoragesConfig struct {
	AWS   *AWSStorageConfig
	Azure *AzureStorageConfig
	GCP   *GCPStorageConfig
	Local *LocalStorageConfig
	Swift *SwiftStorageConfig
	S3    *S3StorageConfig
}

// StorageConfigDefaults specified the default values of StorageControllerConfig for all factories
type StorageConfigDefaults struct {
	Frequency       time.Duration `default:"1h"`
	Retain          int
	Timeout         time.Duration `default:"60s"`
	NamePrefix      string        `default:"raft-snapshot-"`
	NameSuffix      string        `default:".snap"`
	TimestampFormat string        `default:"2006-01-02T15-04-05Z-0700"`
}

// StorageControllerConfig specifies the values for a single controller.
// It is the base for all storage-specific configurations
type StorageControllerConfig struct {
	Frequency       time.Duration
	Retain          *int
	Timeout         time.Duration
	NamePrefix      string
	NameSuffix      string
	TimestampFormat string
}

func (c StorageControllerConfig) frequencyOrDefault(defaults StorageConfigDefaults) time.Duration {
	if c.Frequency > 0 {
		return c.Frequency
	}
	return defaults.Frequency
}

func (c StorageControllerConfig) retainOrDefault(defaults StorageConfigDefaults) int {
	if c.Retain != nil {
		return *c.Retain
	}
	return defaults.Retain
}

func (c StorageControllerConfig) timeoutOrDefault(defaults StorageConfigDefaults) time.Duration {
	if c.Timeout > 0 {
		return c.Timeout
	}
	return defaults.Timeout
}

func (c StorageControllerConfig) namePrefixOrDefault(defaults StorageConfigDefaults) string {
	if c.NamePrefix != "" {
		return c.NamePrefix
	}
	return defaults.NamePrefix
}

func (c StorageControllerConfig) nameSuffixOrDefault(defaults StorageConfigDefaults) string {
	if c.NameSuffix != "" {
		return c.NameSuffix
	}
	return defaults.NameSuffix
}

func (c StorageControllerConfig) timestampFormatOrDefault(defaults StorageConfigDefaults) string {
	if c.TimestampFormat != "" {
		return c.TimestampFormat
	}
	return defaults.TimestampFormat
}
