package storage

import (
	"time"
)

// StoragesConfig specified the configuration-section for the storages to which snapshots are uploaded
type StoragesConfig struct {
	AWS   AWSStorageConfig   `default:"{\"Empty\": true}"`
	Azure AzureStorageConfig `default:"{\"Empty\": true}"`
	GCP   GCPStorageConfig   `default:"{\"Empty\": true}"`
	Local LocalStorageConfig `default:"{\"Empty\": true}"`
	Swift SwiftStorageConfig `default:"{\"Empty\": true}"`
}

// StorageConfigDefaults specified the default values of storageConfig for all controllers
type StorageConfigDefaults struct {
	Frequency       time.Duration `default:"1h"`
	Retain          int
	Timeout         time.Duration `default:"60s"`
	NamePrefix      string        `default:"raft-snapshot-"`
	NameSuffix      string        `default:".snap"`
	TimestampFormat string        `default:"2006-01-02T15-04-05Z-0700"`
}

// storageConfig specified the values for a single controller.
// It is the base for all storage-specific configurations
type storageConfig struct {
	Frequency       time.Duration
	Retain          int `default:"-1"`
	Timeout         time.Duration
	NamePrefix      string
	NameSuffix      string
	TimestampFormat string
}

func (c storageConfig) FrequencyOrDefault(defaults StorageConfigDefaults) time.Duration {
	if c.Frequency > 0 {
		return c.Frequency
	}
	return defaults.Frequency
}

func (c storageConfig) RetainOrDefault(defaults StorageConfigDefaults) int {
	if c.Retain >= 0 {
		return c.Retain
	}
	return defaults.Retain
}

func (c storageConfig) TimeoutOrDefault(defaults StorageConfigDefaults) time.Duration {
	if c.Timeout > 0 {
		return c.Timeout
	}
	return defaults.Timeout
}

func (c storageConfig) NamePrefixOrDefault(defaults StorageConfigDefaults) string {
	if c.NamePrefix != "" {
		return c.NamePrefix
	}
	return defaults.NamePrefix
}

func (c storageConfig) NameSuffixOrDefault(defaults StorageConfigDefaults) string {
	if c.NameSuffix != "" {
		return c.NameSuffix
	}
	return defaults.NameSuffix
}

func (c storageConfig) TimestampFormatOrDefault(defaults StorageConfigDefaults) string {
	if c.TimestampFormat != "" {
		return c.TimestampFormat
	}
	return defaults.TimestampFormat
}
