package livestore

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/grafana/dskit/flagext"
	"github.com/grafana/dskit/services"
	"github.com/grafana/tempo/pkg/ingest"
	"github.com/grafana/tempo/pkg/ingest/testkafka"
	"github.com/grafana/tempo/pkg/util/test"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
	"go.uber.org/atomic"
)

const (
	testTopic         = "test-topic"
	testConsumerGroup = "test-consumer-group"
	testPartition     = int32(0)
)

// TestPartitionReaderCommitNow verifies that commitNow commits the offset to Kafka.
func TestPartitionReaderCommitNow(t *testing.T) {
	k, address := testkafka.CreateCluster(t, 1, testTopic)

	kafkaCommits := atomic.NewInt32(0)
	k.ControlKey(kmsg.OffsetCommit, func(kmsg.Request) (kmsg.Response, error, bool) {
		kafkaCommits.Inc()
		return nil, nil, false
	})

	client := testkafka.NewKafkaClient(t, address, testTopic)
	testkafka.SendReq(t.Context(), t, client, ingest.Encode, testTenantID)

	consumed := make(chan kadm.Offset, 1)
	consumeFn := func(_ context.Context, rs recordIter, _ time.Time) (*kadm.Offset, error) {
		var lastRecord *kgo.Record
		for !rs.Done() {
			lastRecord = rs.Next()
		}
		offset := kadm.NewOffsetFromRecord(lastRecord)
		consumed <- offset
		return &offset, nil
	}

	r := defaultPartitionReader(t, address, consumeFn)

	// Wait for consumption
	offset := <-consumed

	// Before commitNow, nothing should have been committed
	assert.Equal(t, int32(0), kafkaCommits.Load(), "no auto-commits should happen")

	// After commitNow, offset is committed
	require.NoError(t, r.commitNow(t.Context(), offset))
	assert.Eventually(t, func() bool { return kafkaCommits.Load() >= 1 }, time.Second*2, 10*time.Millisecond)
	assert.Equal(t, int64(0), r.lag.Load())

	t.Cleanup(func() { require.NoError(t, services.StopAndAwaitTerminated(context.Background(), r)) })
}

// TestPartitionReaderNoAutoCommit verifies that consuming records does NOT trigger automatic commits.
func TestPartitionReaderNoAutoCommit(t *testing.T) {
	k, address := testkafka.CreateCluster(t, 1, testTopic)

	kafkaCommits := atomic.NewInt32(0)
	k.ControlKey(kmsg.OffsetCommit, func(kmsg.Request) (kmsg.Response, error, bool) {
		kafkaCommits.Inc()
		return nil, nil, false
	})

	client := testkafka.NewKafkaClient(t, address, testTopic)
	testkafka.SendReq(t.Context(), t, client, ingest.Encode, testTenantID)

	consumed := make(chan struct{})
	consumeFn := func(_ context.Context, rs recordIter, _ time.Time) (*kadm.Offset, error) {
		defer close(consumed)
		var lastRecord *kgo.Record
		for !rs.Done() {
			lastRecord = rs.Next()
		}
		offset := kadm.NewOffsetFromRecord(lastRecord)
		return &offset, nil
	}

	r := defaultPartitionReader(t, address, consumeFn)
	defer func() { require.NoError(t, services.StopAndAwaitTerminated(context.Background(), r)) }()

	<-consumed // Record has been consumed

	// Wait a bit and verify no auto-commits happened
	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, int32(0), kafkaCommits.Load(), "consuming should not auto-commit")
}

func TestPartitionReaderLag(t *testing.T) {
	k, address := testkafka.CreateCluster(t, 1, testTopic)

	kafkaCommits := atomic.NewInt32(0)
	k.ControlKey(kmsg.OffsetCommit, func(kmsg.Request) (kmsg.Response, error, bool) {
		kafkaCommits.Inc()
		return nil, nil, false
	})

	var counter int
	consumeFn := func(_ context.Context, rs recordIter, _ time.Time) (*kadm.Offset, error) {
		counter++
		if counter <= 1 { // allow to process one record to store lag
			lastRecord := rs.Next()
			offset := kadm.NewOffsetFromRecord(lastRecord)
			return &offset, nil
		}
		return nil, errors.New("error consuming records")
	}

	r := defaultPartitionReader(t, address, consumeFn)

	client := testkafka.NewKafkaClient(t, address, testTopic)
	records := 10
	for range records {
		testkafka.SendReq(t.Context(), t, client, ingest.Encode, testTenantID)
	}

	time.Sleep(time.Second)
	assert.Equal(t, int32(0), kafkaCommits.Load(), "no auto-commits should happen")
	assert.Equal(t, int64(records)-1, r.lag.Load(), "only one record should be in watermark")

	t.Cleanup(func() { require.NoError(t, services.StopAndAwaitTerminated(context.Background(), r)) })
}

func TestFetchLastCommittedOffsetForceFromLookback(t *testing.T) {
	lookback := time.Hour

	t.Run("committed offset exists, forceFromLookback=false uses committed offset", func(t *testing.T) {
		_, address := testkafka.CreateCluster(t, 1, testTopic)
		client := testkafka.NewKafkaClient(t, address, testTopic)

		// Produce a record and commit an offset
		testkafka.SendReq(t.Context(), t, client, ingest.Encode, testTenantID)

		adm := kadm.NewClient(client)
		offsets := make(kadm.Offsets)
		offsets.Add(kadm.Offset{
			Topic:     testTopic,
			Partition: testPartition,
			At:        0,
		})
		_, err := adm.CommitOffsets(t.Context(), testConsumerGroup, offsets)
		require.NoError(t, err)

		l := test.NewTestingLogger(t)
		cfg := ingest.KafkaConfig{}
		flagext.DefaultValues(&cfg)
		cfg.Address = address
		cfg.Topic = testTopic
		cfg.ConsumerGroup = testConsumerGroup

		readerClient, err := ingest.NewReaderClient(cfg, ingest.NewReaderClientMetrics(liveStoreServiceName, prometheus.NewRegistry()), l)
		require.NoError(t, err)

		r, err := newPartitionReader(readerClient, testPartition, cfg, lookback, false, nil, l, newPartitionReaderMetrics(testPartition, prometheus.NewRegistry()))
		require.NoError(t, err)

		offset, err := r.fetchLastCommittedOffset(t.Context())
		require.NoError(t, err)

		// Should use committed offset (At(0)), not lookback
		epochOffset := offset.EpochOffset()
		assert.Equal(t, int64(0), epochOffset.Offset)
	})

	t.Run("committed offset exists, forceFromLookback=true ignores committed offset", func(t *testing.T) {
		_, address := testkafka.CreateCluster(t, 1, testTopic)
		client := testkafka.NewKafkaClient(t, address, testTopic)

		// Produce a record and commit an offset
		testkafka.SendReq(t.Context(), t, client, ingest.Encode, testTenantID)

		adm := kadm.NewClient(client)
		offsets := make(kadm.Offsets)
		offsets.Add(kadm.Offset{
			Topic:     testTopic,
			Partition: testPartition,
			At:        0,
		})
		_, err := adm.CommitOffsets(t.Context(), testConsumerGroup, offsets)
		require.NoError(t, err)

		l := test.NewTestingLogger(t)
		cfg := ingest.KafkaConfig{}
		flagext.DefaultValues(&cfg)
		cfg.Address = address
		cfg.Topic = testTopic
		cfg.ConsumerGroup = testConsumerGroup

		readerClient, err := ingest.NewReaderClient(cfg, ingest.NewReaderClientMetrics(liveStoreServiceName, prometheus.NewRegistry()), l)
		require.NoError(t, err)

		r, err := newPartitionReader(readerClient, testPartition, cfg, lookback, true, nil, l, newPartitionReaderMetrics(testPartition, prometheus.NewRegistry()))
		require.NoError(t, err)

		offset, err := r.fetchLastCommittedOffset(t.Context())
		require.NoError(t, err)

		// Should use lookback period (AfterMilli), not the committed offset
		epochOffset := offset.EpochOffset()
		expectedMilli := time.Now().Add(-lookback).UnixMilli()
		assert.InDelta(t, expectedMilli, epochOffset.Offset, 5000, "offset should be near lookback time in millis")
		assert.Equal(t, int32(-1), epochOffset.Epoch, "epoch=-1 indicates AfterMilli offset")
	})

	t.Run("no committed offset, forceFromLookback=true uses lookback period", func(t *testing.T) {
		_, address := testkafka.CreateCluster(t, 1, testTopic)

		l := test.NewTestingLogger(t)
		cfg := ingest.KafkaConfig{}
		flagext.DefaultValues(&cfg)
		cfg.Address = address
		cfg.Topic = testTopic
		cfg.ConsumerGroup = testConsumerGroup

		readerClient, err := ingest.NewReaderClient(cfg, ingest.NewReaderClientMetrics(liveStoreServiceName, prometheus.NewRegistry()), l)
		require.NoError(t, err)

		r, err := newPartitionReader(readerClient, testPartition, cfg, lookback, true, nil, l, newPartitionReaderMetrics(testPartition, prometheus.NewRegistry()))
		require.NoError(t, err)

		offset, err := r.fetchLastCommittedOffset(t.Context())
		require.NoError(t, err)

		// Should use lookback period
		epochOffset := offset.EpochOffset()
		expectedMilli := time.Now().Add(-lookback).UnixMilli()
		assert.InDelta(t, expectedMilli, epochOffset.Offset, 5000, "offset should be near lookback time in millis")
		assert.Equal(t, int32(-1), epochOffset.Epoch, "epoch=-1 indicates AfterMilli offset")
	})
}

func defaultPartitionReader(t *testing.T, address string, consume consumeFn) *PartitionReader {
	l := test.NewTestingLogger(t)

	cfg := ingest.KafkaConfig{}
	flagext.DefaultValues(&cfg)
	cfg.Address = address
	cfg.Topic = testTopic
	cfg.ConsumerGroup = testConsumerGroup

	client, err := ingest.NewReaderClient(
		cfg,
		ingest.NewReaderClientMetrics(liveStoreServiceName, prometheus.NewRegistry()),
		l,
	)
	require.NoError(t, err)

	r, err := newPartitionReader(client, 0, cfg, time.Hour, false, consume, l, newPartitionReaderMetrics(testPartition, prometheus.NewRegistry()))
	require.NoError(t, err)

	err = services.StartAndAwaitRunning(t.Context(), r)
	require.NoError(t, err)

	return r
}
