package metrics

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestPublishNextSnapshot(t *testing.T) {
	registry := prometheus.NewRegistry()
	publisher := newPrometheusPublisher(registry, nil)

	next := time.Now()

	publisher.PublishNextSnapshot(next)

	expected := fmt.Sprintf(
		`# HELP vrsa_next_snapshot_time Unix timestamp of the next scheduled snapshot time
# TYPE vrsa_next_snapshot_time gauge
vrsa_next_snapshot_time %f
`, float64(next.Unix()))

	err := testutil.CollectAndCompare(registry, strings.NewReader(expected), "vrsx_next_snapshot_time")
	if err != nil {
		t.Errorf("%s", err.Error())
	}
}

func TestPublishSuccess(t *testing.T) {
	registry := prometheus.NewRegistry()
	publisher := newPrometheusPublisher(registry, nil)

	time := time.Now()
	size := int64(1000)

	publisher.PublishSuccess(time, size)

	expected := fmt.Sprintf(
		`# HELP vrsa_last_snapshot_time Unix timestamp of the last snapshot time
# TYPE vrsa_last_snapshot_time gauge
vrsa_last_snapshot_time %f
# HELP vrsa_last_successful_snapshot_time Unix timestamp of the last successful snapshot time
# TYPE vrsa_last_successful_snapshot_time gauge
vrsa_last_successful_snapshot_time %f
# HELP vrsa_last_snapshot_success Returns 1 if the last snapshot was successful and 0 if not
# TYPE vrsa_last_snapshot_success gauge
vrsa_last_snapshot_success 1.0
`, float64(time.Unix()), float64(time.Unix()))

	err := testutil.CollectAndCompare(registry, strings.NewReader(expected), "vrsa_last_snapshot_time", "vrsa_last_successful_snapshot_time", "vrsa_last_snapshot_success")
	if err != nil {
		t.Errorf("%s", err.Error())
	}
}

func TestPublishFailure(t *testing.T) {
	registry := prometheus.NewRegistry()
	publisher := newPrometheusPublisher(registry, nil)

	time := time.Now()

	publisher.PublishFailure(time)

	expected := fmt.Sprintf(
		`# HELP vrsa_last_snapshot_time Unix timestamp of the last snapshot time
# TYPE vrsa_last_snapshot_time gauge
vrsa_last_snapshot_time %f
# HELP vrsa_last_snapshot_success Returns 1 if the last snapshot was successful and 0 if not
# TYPE vrsa_last_snapshot_success gauge
vrsa_last_snapshot_success 0.0
`, float64(time.Unix()))

	err := testutil.CollectAndCompare(registry, strings.NewReader(expected), "vrsa_last_snapshot_time", "vrsa_last_snapshot_success")
	if err != nil {
		t.Errorf("%s", err.Error())
	}
}

func TestServer(t *testing.T) {
	port, err := GetFreePort()
	assert.NoError(t, err, "should acquire free port")

	config := &PrometheusPublisherConfig{
		Port: port,
		Path: "/test",
	}

	last := time.Now()
	next := time.Now().Add(60)
	size := int64(1000)

	publisher := createPrometheusPublisher(context.Background(), config)
	publisher.Start()

	defer func() {
		publisher.Shutdown()
	}()

	publisher.PublishSuccess(last, size)
	publisher.PublishNextSnapshot(next)
	ch := make(chan prometheus.Metric)
	go func() {
		publisher.nextSnapshotTime.Collect(ch)
	}()

	<-ch

	c := http.Client{Timeout: time.Duration(1) * time.Second}
	resp, err := c.Get(fmt.Sprintf("http://localhost:%d/test", port))
	if err != nil {
		fmt.Printf("Error %s", err)
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Equal(t,
		fmt.Sprintf(
			`# HELP vrsa_last_snapshot_size Size of the last snapshot in bytes
# TYPE vrsa_last_snapshot_size gauge
vrsa_last_snapshot_size %d
# HELP vrsa_last_snapshot_success Returns 1 if the last snapshot was successful and 0 if not
# TYPE vrsa_last_snapshot_success gauge
vrsa_last_snapshot_success 1
# HELP vrsa_last_snapshot_time Unix timestamp of the last snapshot time
# TYPE vrsa_last_snapshot_time gauge
vrsa_last_snapshot_time %v
# HELP vrsa_last_successful_snapshot_time Unix timestamp of the last successful snapshot time
# TYPE vrsa_last_successful_snapshot_time gauge
vrsa_last_successful_snapshot_time %v
# HELP vrsa_next_snapshot_time Unix timestamp of the next scheduled snapshot time
# TYPE vrsa_next_snapshot_time gauge
vrsa_next_snapshot_time %v
`,
			size, float64(last.Unix()), float64(last.Unix()), float64(next.Unix()),
		),
		fmt.Sprintf("%s", body),
	)
}

func GetFreePort() (port int, err error) {
	a, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", a)
	if err != nil {
		return 0, err
	}

	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}
