package metrics

import (
	"context"
	"errors"
	"fmt"
	"time"

	"net"
	"net/http"

	"github.com/Argelbargel/vault-raft-snapshot-agent/internal/agent/logging"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type PrometheusPublisherConfig struct {
	Port int    `default:"2112" validate:"required"`
	Path string `default:"/metrics" validate:"required"`
}

type prometheusPublisher struct {
	server                     *http.Server
	lastSnapshotTime           prometheus.Gauge
	lastSuccessfulSnapshotTime prometheus.Gauge
	lastSnapshotSuccess        prometheus.Gauge
	nextSnapshotTime           prometheus.Gauge
	lastSnapshotSize           prometheus.Gauge
}

func createPrometheusPublisher(ctx context.Context, config *PrometheusPublisherConfig) *prometheusPublisher {
	registry := prometheus.NewRegistry()

	mux := http.NewServeMux()
	mux.Handle(config.Path, promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", config.Port),
		Handler: mux,
		BaseContext: func(l net.Listener) context.Context {
			return ctx
		},
	}

	return newPrometheusPublisher(registry, server)
}

func newPrometheusPublisher(registry *prometheus.Registry, server *http.Server) *prometheusPublisher {
	return &prometheusPublisher{
		server: server,
		lastSnapshotTime: promauto.With(registry).NewGauge(
			prometheus.GaugeOpts{
				Name: "vrsa_last_snapshot_time",
				Help: "Unix timestamp of the last snapshot time",
			},
		),
		lastSuccessfulSnapshotTime: promauto.With(registry).NewGauge(
			prometheus.GaugeOpts{
				Name: "vrsa_last_successful_snapshot_time",
				Help: "Unix timestamp of the last successful snapshot time",
			},
		),
		lastSnapshotSuccess: promauto.With(registry).NewGauge(
			prometheus.GaugeOpts{
				Name: "vrsa_last_snapshot_success",
				Help: "Returns 1 if the last snapshot was successful and 0 if not",
			},
		),
		nextSnapshotTime: promauto.With(registry).NewGauge(
			prometheus.GaugeOpts{
				Name: "vrsa_next_snapshot_time",
				Help: "Unix timestamp of the next scheduled snapshot time",
			},
		),
		lastSnapshotSize: promauto.With(registry).NewGauge(
			prometheus.GaugeOpts{
				Name: "vrsa_last_snapshot_size",
				Help: "Size of the last snapshot in bytes",
			},
		),
	}
}

func (p *prometheusPublisher) PublishNextSnapshot(next time.Time) {
	p.nextSnapshotTime.Set(float64(next.Unix()))
}

func (p *prometheusPublisher) PublishSuccess(timestamp time.Time, size int64) {
	p.lastSnapshotTime.Set(float64(timestamp.Unix()))
	p.lastSuccessfulSnapshotTime.Set(float64(timestamp.Unix()))
	p.lastSnapshotSize.Set(float64(size))
	p.lastSnapshotSuccess.Set(1.0)
}

func (p *prometheusPublisher) PublishFailure(timestamp time.Time) {
	p.lastSnapshotTime.Set(float64(timestamp.Unix()))
	p.lastSnapshotSuccess.Set(0.0)
}

func (p *prometheusPublisher) Start() error {
	go func() {
		err := p.server.ListenAndServe()
		if !errors.Is(err, http.ErrServerClosed) {
			logging.Fatal("failed to serve prometheus metrics", "error", err)
		}
	}()
	return nil
}

func (p *prometheusPublisher) Shutdown() error {
	err := p.server.Shutdown(context.Background())
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
