package metrics

import (
	"errors"
	"time"

	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStartCallsPublisherMethods(t *testing.T) {
	publisher1 := &PublisherStub{}
	publisher2 := &PublisherStub{}

	collector := &Collector{}
	collector.AddPublisher(publisher1)
	collector.AddPublisher(publisher2)

	next := time.Now()

	err := collector.Start(next)

	assert.NoError(t, err, "Start should not return an error")
	assert.True(t, publisher1.started, "publisher1 should be started")
	assert.True(t, publisher2.started, "publisher2 should be started")

	assert.Equal(t, next, publisher1.nextSnapshotTime)
	assert.Equal(t, next, publisher2.nextSnapshotTime)
}

func TestStartReturnsOnFirstPublisherError(t *testing.T) {
	publisher1 := &PublisherStub{
		startError: errors.New("p1"),
	}
	publisher2 := &PublisherStub{
		startError: errors.New("p2"),
	}

	collector := &Collector{}
	collector.AddPublisher(publisher1)
	collector.AddPublisher(publisher2)

	err := collector.Start(time.Now())

	assert.Equal(t, publisher1.startError, err, "Start should return error returned by publisher1")
	assert.True(t, publisher1.started, "publisher1 should be started")
	assert.False(t, publisher2.started, "publisher2 should NOT be started")

	assert.Empty(t, publisher1.nextSnapshotTime, "nextSnapshotTime of publisher1 should not be set")
	assert.Empty(t, publisher1.nextSnapshotTime, "nextSnapshotTime of publisher2 should not be set")
}

func TestShutdownCallsPublisherMethods(t *testing.T) {
	publisher1 := &PublisherStub{}
	publisher2 := &PublisherStub{}

	collector := &Collector{}
	collector.AddPublisher(publisher1)
	collector.AddPublisher(publisher2)

	err := collector.Shutdown()

	assert.NoError(t, err, "Shutdown should not return an error")
	assert.True(t, publisher1.shutdown, "publisher1 should be shutdown")
	assert.True(t, publisher2.shutdown, "publisher2 should be shutdown")
}

func TestShutdownCollectsPublisherErrors(t *testing.T) {
	publisher1 := &PublisherStub{
		shutdownError: errors.New("p1"),
	}
	publisher2 := &PublisherStub{
		shutdownError: errors.New("p2"),
	}

	collector := &Collector{}
	collector.AddPublisher(publisher1)
	collector.AddPublisher(publisher2)

	err := collector.Shutdown()

	assert.Equal(t, err, errors.Join(publisher1.shutdownError, publisher2.shutdownError), "Shutdown should collect shutdown errors")
	assert.True(t, publisher1.shutdown, "publisher1 should be shutdown")
	assert.True(t, publisher2.shutdown, "publisher2 should be shutdown")
}

func TestCollectCallsPublisherMethodsForSuccess(t *testing.T) {
	publisher1 := &PublisherStub{}
	publisher2 := &PublisherStub{}

	collector := &Collector{}
	collector.AddPublisher(publisher1)
	collector.AddPublisher(publisher2)

	time := time.Now()
	next := time.Add(60)
	size := int64(1000)

	collector.Collect(time, size, next)

	assert.Equal(t, time, publisher1.lastSnapshotTime, "lastSnapshotTime of publisher1 should be equal to that collected")
	assert.Equal(t, time, publisher2.lastSnapshotTime, "lastSnapshotTime of publisher1 should be equal to that collected")

	assert.True(t, publisher1.success, "publisher1 should report success")
	assert.True(t, publisher2.success, "publisher2 should report success")

	assert.Equal(t, size, publisher1.size, "publisher1 should report correct size")
	assert.Equal(t, size, publisher2.size, "publisher2 should report correct size")

	assert.Equal(t, next, publisher1.nextSnapshotTime, "publisher1 should report correct next snapshot time")
	assert.Equal(t, next, publisher2.nextSnapshotTime, "publisher2 should report correct next snapshot time")
}

func TestCollectCallsPublisherMethodsFoFailure(t *testing.T) {
	publisher1 := &PublisherStub{}
	publisher2 := &PublisherStub{}

	collector := &Collector{}
	collector.AddPublisher(publisher1)
	collector.AddPublisher(publisher2)

	time := time.Now()
	next := time.Add(60)

	collector.Collect(time, 0, next)

	assert.Equal(t, time, publisher1.lastSnapshotTime, "lastSnapshotTime of publisher1 should be equal to that collected")
	assert.Equal(t, time, publisher2.lastSnapshotTime, "lastSnapshotTime of publisher1 should be equal to that collected")

	assert.False(t, publisher1.success, "publisher1 should NOT report success")
	assert.False(t, publisher2.success, "publisher2 should NOT report success")

	assert.Empty(t, publisher1.size, "publisher1 should report empty size")
	assert.Empty(t, publisher2.size, "publisher2 should report empty size")

	assert.Equal(t, next, publisher1.nextSnapshotTime, "publisher1 should report correct next snapshot time")
	assert.Equal(t, next, publisher2.nextSnapshotTime, "publisher2 should report correct next snapshot time")
}



type PublisherStub struct {
	lastSnapshotTime time.Time
	size             int64
	nextSnapshotTime time.Time
	success          bool
	started          bool
	shutdown         bool
	startError       error
	shutdownError    error
}

func (p *PublisherStub) Start() error {
	p.started = true
	return p.startError
}

func (p *PublisherStub) Shutdown() error {
	p.shutdown = true
	return p.shutdownError
}

func (p *PublisherStub) PublishNextSnapshot(next time.Time) {
	p.nextSnapshotTime = next
}

func (p *PublisherStub) PublishSuccess(timestamp time.Time, size int64) {
	p.lastSnapshotTime = timestamp
	p.success = true
	p.size = size
}

func (p *PublisherStub) PublishFailure(timestamp time.Time) {
	p.lastSnapshotTime = timestamp
	p.success = false
}
