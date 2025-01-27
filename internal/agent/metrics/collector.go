package metrics

import (
	"context"
	"errors"
	"time"
)

type CollectorConfig struct {
	Prometheus *PrometheusPublisherConfig
}

type Publisher interface {
	PublishNextSnapshot(next time.Time)
	PublishSuccess(timestamp time.Time, size int64)
	PublishFailure(timestamp time.Time)
	Shutdown() error
	Start() error
}

type Collector struct {
	publishers []Publisher
}

func CreateCollector(ctx context.Context, config CollectorConfig) *Collector {
	collector := &Collector{}

	if config.Prometheus != nil {
		collector.AddPublisher(createPrometheusPublisher(ctx, config.Prometheus))
	}
	return collector
}

// adds a Publisher to the collector
// Allows adding of publisher-implementations for testing
func (c *Collector) AddPublisher(publisher Publisher) {
	c.publishers = append(c.publishers, publisher)
}

func (c *Collector) Collect(timestamp time.Time, size int64, next time.Time) {
	for _, publisher := range c.publishers {
		if size > 0 {
			publisher.PublishSuccess(timestamp, size)
		} else {
			publisher.PublishFailure(timestamp)
		}
		publisher.PublishNextSnapshot(next)
	}
}

func (c *Collector) Shutdown() error {
	var errs []error
	for _, publisher := range c.publishers {
		errs = append(errs, publisher.Shutdown())
	}
	return errors.Join(errs...)
}

func (c *Collector) Start(nextSnapshot time.Time) error {
	for _, publisher := range c.publishers {
		if err := publisher.Start(); err != nil {
			return err
		}
		publisher.PublishNextSnapshot(nextSnapshot)
	}
	return nil
}
