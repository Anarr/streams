package kafka_test

import (
	"testing"
	"time"

	"github.com/Shopify/sarama"
	"github.com/rafalmnich/streams/v6"
	"github.com/rafalmnich/streams/v6/kafka"
	"github.com/stretchr/testify/assert"
)

func TestNewSourceConfig(t *testing.T) {
	c := kafka.NewSourceConfig()

	assert.IsType(t, &kafka.SourceConfig{}, c)
}

func TestSourceConfig_Validate(t *testing.T) {
	c := kafka.NewSourceConfig()
	c.Brokers = []string{"test"}

	err := c.Validate()

	assert.NoError(t, err)
}

func TestSourceConfig_ValidateErrors(t *testing.T) {
	tests := []struct {
		name string
		cfg  func(*kafka.SourceConfig)
		err  string
	}{
		{
			name: "Brokers",
			cfg: func(c *kafka.SourceConfig) {
				c.Brokers = []string{}
			},
			err: "Brokers must have at least one broker",
		},
		{
			name: "KeyDecoder",
			cfg: func(c *kafka.SourceConfig) {
				c.Brokers = []string{"test"}
				c.KeyDecoder = nil
			},
			err: "KeyDecoder must be an instance of Decoder",
		},
		{
			name: "ValueDecoder",
			cfg: func(c *kafka.SourceConfig) {
				c.Brokers = []string{"test"}
				c.ValueDecoder = nil
			},
			err: "ValueDecoder must be an instance of Decoder",
		},
		{
			name: "BufferSize",
			cfg: func(c *kafka.SourceConfig) {
				c.Brokers = []string{"test"}
				c.BufferSize = 0
			},
			err: "BufferSize must be at least 1",
		},
		{
			name: "BaseConfig",
			cfg: func(c *kafka.SourceConfig) {
				c.Brokers = []string{"test"}
				c.Metadata.Retry.Max = -1
			},
			err: "Metadata.Retry.Max must be >= 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := kafka.NewSourceConfig()
			tt.cfg(c)

			err := c.Validate()

			assert.Equal(t, tt.err, string(err.(sarama.ConfigurationError)))
		})
	}
}

func TestMetadata_WithOrigin(t *testing.T) {
	meta := kafka.Metadata{{Topic: "foo", Partition: 0, Offset: 3}}

	meta.WithOrigin(streams.CommitterOrigin)

	assert.Equal(t, kafka.Metadata{{Topic: "foo", Partition: 0, Offset: 3, Origin: streams.CommitterOrigin}}, meta)
}

func TestMetadata_Merge(t *testing.T) {
	meta1 := kafka.Metadata{{Topic: "foo", Partition: 0, Offset: 3}}
	meta2 := kafka.Metadata{{Topic: "foo", Partition: 1, Offset: 2}}

	res := meta2.Merge(meta1, streams.Lossless)

	assert.IsType(t, kafka.Metadata{}, res)
	meta1 = res.(kafka.Metadata)
	assert.Equal(t, kafka.Metadata{{Topic: "foo", Partition: 0, Offset: 3}, {Topic: "foo", Partition: 1, Offset: 2}}, meta1)
}

func TestMetadata_MergeTakesCommitterOverProcessorWhenCommitter(t *testing.T) {
	meta1 := kafka.Metadata{{Topic: "foo", Partition: 0, Offset: 2, Origin: streams.ProcessorOrigin}}
	meta2 := kafka.Metadata{{Topic: "foo", Partition: 0, Offset: 3, Origin: streams.CommitterOrigin}}

	res := meta2.Merge(meta1, streams.Lossless)

	assert.IsType(t, kafka.Metadata{}, res)
	resMeta := res.(kafka.Metadata)
	assert.Equal(t, kafka.Metadata{{Topic: "foo", Partition: 0, Offset: 3, Origin: streams.CommitterOrigin}}, resMeta)
}

func TestMetadata_MergeTakesCommitterOverProcessorWhenProcessor(t *testing.T) {
	meta1 := kafka.Metadata{{Topic: "foo", Partition: 0, Offset: 2, Origin: streams.ProcessorOrigin}}
	meta2 := kafka.Metadata{{Topic: "foo", Partition: 0, Offset: 3, Origin: streams.CommitterOrigin}}

	res := meta1.Merge(meta2, streams.Lossless)

	assert.IsType(t, kafka.Metadata{}, res)
	resMeta := res.(kafka.Metadata)
	assert.Equal(t, kafka.Metadata{{Topic: "foo", Partition: 0, Offset: 3, Origin: streams.CommitterOrigin}}, resMeta)
}

func TestMetadata_MergeTakesHighestWhenTheSameOriginAndDupless(t *testing.T) {
	meta1 := kafka.Metadata{{Topic: "foo", Partition: 0, Offset: 3, Origin: streams.ProcessorOrigin}}
	meta2 := kafka.Metadata{{Topic: "foo", Partition: 0, Offset: 2, Origin: streams.ProcessorOrigin}}

	res := meta2.Merge(meta1, streams.Dupless)

	assert.IsType(t, kafka.Metadata{}, res)
	meta1 = res.(kafka.Metadata)
	assert.Equal(t, kafka.Metadata{{Topic: "foo", Partition: 0, Offset: 3, Origin: streams.ProcessorOrigin}}, meta1)
}

func TestMetadata_MergeTakesLowestWhenTheSameOriginAndLossLess(t *testing.T) {
	meta1 := kafka.Metadata{{Topic: "foo", Partition: 0, Offset: 3, Origin: streams.ProcessorOrigin}}
	meta2 := kafka.Metadata{{Topic: "foo", Partition: 0, Offset: 2, Origin: streams.ProcessorOrigin}}

	res := meta2.Merge(meta1, streams.Lossless)

	assert.IsType(t, kafka.Metadata{}, res)
	meta1 = res.(kafka.Metadata)
	assert.Equal(t, kafka.Metadata{{Topic: "foo", Partition: 0, Offset: 2, Origin: streams.ProcessorOrigin}}, meta1)
}

func TestMetadata_MergeNilMerged(t *testing.T) {
	b := kafka.Metadata{{Topic: "foo", Partition: 0, Offset: 3}}

	res := b.Merge(nil, streams.Lossless)

	assert.IsType(t, kafka.Metadata{}, res)
	a := res.(kafka.Metadata)
	assert.Equal(t, kafka.Metadata{{Topic: "foo", Partition: 0, Offset: 3}}, a)
}

func BenchmarkMetadata_Merge(b *testing.B) {
	var meta streams.Metadata = kafka.Metadata{{Topic: "test", Partition: 1, Offset: 2}}
	other := kafka.Metadata{{Topic: "test", Partition: 2, Offset: 2}}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		meta = other.Merge(meta, streams.Lossless)
	}
}

func TestNewSource(t *testing.T) {
	broker0 := sarama.NewMockBroker(t, 0)
	defer broker0.Close()
	broker0.SetHandlerByMap(map[string]sarama.MockResponse{
		"MetadataRequest": sarama.NewMockMetadataResponse(t).
			SetBroker(broker0.Addr(), broker0.BrokerID()).
			SetLeader("test_topic", 0, broker0.BrokerID()),
		"FindCoordinatorRequest": sarama.NewMockFindCoordinatorResponse(t).
			SetCoordinator(sarama.CoordinatorGroup, "test_group", broker0),
		"JoinGroupRequest": sarama.NewMockWrapper(&sarama.JoinGroupResponse{
			Version:       1,
			Err:           sarama.ErrNoError,
			GroupProtocol: "protocol",
		}),
		"SyncGroupRequest": sarama.NewMockWrapper(&sarama.SyncGroupResponse{
			Err: sarama.ErrNoError,
			MemberAssignment: []byte{
				0, 1, // Version
				0, 0, 0, 1, // Topic array length
				0, 10, 't', 'e', 's', 't', '_', 't', 'o', 'p', 'i', 'c', // Topic one
				0, 0, 0, 1, // Topic one, partition array length
				0, 0, 0, 0, // 0
				0, 0, 0, 3, 0x01, 0x02, 0x03, // Userdata
			},
		}),
		"OffsetFetchRequest": sarama.NewMockOffsetFetchResponse(t),
		"LeaveGroupRequest": sarama.NewMockWrapper(&sarama.LeaveGroupResponse{
			Err: sarama.ErrNoError,
		}),
	})
	c := kafka.NewSourceConfig()
	c.Brokers = []string{broker0.Addr()}
	c.Topic = "test_topic"
	c.GroupID = "test_group"
	c.Version = sarama.V2_3_0_0

	s, err := kafka.NewSource(c)

	time.Sleep(100 * time.Millisecond)

	assert.NoError(t, err)
	assert.IsType(t, &kafka.Source{}, s)

	if s != nil {
		s.Close()
	}
}

func TestNewSource_ValidatesConfig(t *testing.T) {
	c := kafka.NewSourceConfig()

	_, err := kafka.NewSource(c)

	assert.Error(t, err)
}

func TestNewSource_Error(t *testing.T) {
	broker0 := sarama.NewMockBroker(t, 0)
	broker0.Close()
	c := kafka.NewSourceConfig()
	c.Brokers = []string{broker0.Addr()}
	c.Topic = "test_topic"
	c.GroupID = "test_group"

	_, err := kafka.NewSource(c)

	assert.Error(t, err)
}

// TODO
//  	Fix this test. It's failing with an error:
//  			"kafka: error decoding packet: correlation ID didn't match, wanted 8, got 9".
// 		It looks like the mock broker doesn't handle the protocol for the ConsumerGroup correctly,
// 		or some expectations are incorrect/missing.
//
// func TestSource_Consume(t *testing.T) {
// 	broker0 := sarama.NewMockBroker(t, 0)
// 	defer broker0.Close()
// 	broker0.SetHandlerByMap(map[string]sarama.MockResponse{
// 		"MetadataRequest": sarama.NewMockMetadataResponse(t).
// 			SetBroker(broker0.Addr(), broker0.BrokerID()).
// 			SetLeader("test_topic", 0, broker0.BrokerID()),
// 		"FindCoordinatorRequest": sarama.NewMockFindCoordinatorResponse(t).
// 			SetCoordinator(sarama.CoordinatorGroup, "test_group", broker0),
// 		"JoinGroupRequest": sarama.NewMockWrapper(&sarama.JoinGroupResponse{
// 			Version:       1,
// 			Err:           sarama.ErrNoError,
// 			GroupProtocol: "protocol",
// 		}),
// 		"SyncGroupRequest": sarama.NewMockWrapper(&sarama.SyncGroupResponse{
// 			Err: sarama.ErrNoError,
// 			MemberAssignment: []byte{
// 				0, 1, // Version
// 				0, 0, 0, 1, // Topic array length
// 				0, 10, 't', 'e', 's', 't', '_', 't', 'o', 'p', 'i', 'c', // Topic one
// 				0, 0, 0, 1, // Topic one, partition array length
// 				0, 0, 0, 0, // 0
// 				0, 0, 0, 3, 0x01, 0x02, 0x03, // Userdata
// 			},
// 		}),
// 		"OffsetFetchRequest": sarama.NewMockOffsetFetchResponse(t).
// 			SetOffset("test_group", "test_topic", 0, 10, "", sarama.ErrNoError),
// 		"OffsetRequest": sarama.NewMockOffsetResponse(t).
// 			SetOffset("test_topic", 0, sarama.OffsetNewest, 10).
// 			SetOffset("test_topic", 0, sarama.OffsetOldest, 7),
// 		"FetchRequest": sarama.NewMockFetchResponse(t, 1).
// 			SetVersion(1).
// 			SetMessage("test_topic", 0, 10, sarama.StringEncoder("foo")).
// 			SetHighWaterMark("test_topic", 0, 14),
// 		"LeaveGroupRequest": sarama.NewMockWrapper(&sarama.LeaveGroupResponse{
// 			Err: sarama.ErrNoError,
// 		}),
// 	})
// 	c := kafka.NewSourceConfig()
// 	c.Brokers = []string{broker0.Addr()}
// 	c.Topic = "test_topic"
// 	c.GroupID = "test_group"
// 	c.Version = sarama.V2_3_0_0
//
// 	s, err := kafka.NewSource(c)
// 	if err != nil {
// 		panic(err)
// 	}
// 	defer s.Close()
//
// 	time.Sleep(500 * time.Millisecond)
//
// 	msg, err := s.Consume()
//
// 	assert.NoError(t, err)
// 	assert.Equal(t, []byte(nil), msg.Key)
// 	assert.Equal(t, []byte("foo"), msg.Value)
// }

func TestSource_ConsumeError(t *testing.T) {
	broker0 := sarama.NewMockBroker(t, 0)
	defer broker0.Close()
	broker0.SetHandlerByMap(map[string]sarama.MockResponse{
		"MetadataRequest": sarama.NewMockMetadataResponse(t).
			SetBroker(broker0.Addr(), broker0.BrokerID()).
			SetLeader("test_topic", 0, broker0.BrokerID()),
		"FindCoordinatorRequest": sarama.NewMockFindCoordinatorResponse(t).
			SetCoordinator(sarama.CoordinatorGroup, "test_group", broker0),
		"JoinGroupRequest": sarama.NewMockWrapper(&sarama.JoinGroupResponse{
			Version:       1,
			Err:           sarama.ErrNoError,
			GroupProtocol: "protocol",
		}),
		"SyncGroupRequest": sarama.NewMockWrapper(&sarama.SyncGroupResponse{
			Err:              sarama.ErrBrokerNotAvailable,
			MemberAssignment: []byte{},
		}),
		"LeaveGroupRequest": sarama.NewMockWrapper(&sarama.LeaveGroupResponse{
			Err: sarama.ErrNoError,
		}),
	})
	c := kafka.NewSourceConfig()
	c.Brokers = []string{broker0.Addr()}
	c.Topic = "test_topic"
	c.GroupID = "test_group"
	c.Version = sarama.V2_3_0_0

	s, err := kafka.NewSource(c)
	if err != nil {
		panic(err)
	}
	defer s.Close()

	time.Sleep(500 * time.Millisecond)

	_, err = s.Consume()

	assert.Error(t, err)
}

// TODO
//  	Fix this test. It's failing with an error:
//  			"kafka: error decoding packet: correlation ID didn't match, wanted 8, got 9".
// 		It looks like the mock broker doesn't handle the protocol for the ConsumerGroup correctly,
// 		or some expectations are incorrect/missing.
//
// func TestSource_Commit(t *testing.T) {
// 	broker0 := sarama.NewMockBroker(t, 0)
// 	defer broker0.Close()
// 	broker0.SetHandlerByMap(map[string]sarama.MockResponse{
// 		"MetadataRequest": sarama.NewMockMetadataResponse(t).
// 			SetBroker(broker0.Addr(), broker0.BrokerID()).
// 			SetLeader("test_topic", 0, broker0.BrokerID()),
// 		"FindCoordinatorRequest": sarama.NewMockFindCoordinatorResponse(t).
// 			SetCoordinator(sarama.CoordinatorGroup, "test_group", broker0),
// 		"JoinGroupRequest": sarama.NewMockWrapper(&sarama.JoinGroupResponse{
// 			Version:       1,
// 			Err:           sarama.ErrNoError,
// 			GroupProtocol: "protocol",
// 		}),
// 		"SyncGroupRequest": sarama.NewMockWrapper(&sarama.SyncGroupResponse{
// 			Err: sarama.ErrNoError,
// 			MemberAssignment: []byte{
// 				0, 1, // Version
// 				0, 0, 0, 1, // Topic array length
// 				0, 10, 't', 'e', 's', 't', '_', 't', 'o', 'p', 'i', 'c', // Topic one
// 				0, 0, 0, 1, // Topic one, partition array length
// 				0, 0, 0, 0, // 0
// 				0, 0, 0, 3, 0x01, 0x02, 0x03, // Userdata
// 			},
// 		}),
// 		"OffsetFetchRequest": sarama.NewMockOffsetFetchResponse(t).
// 			SetOffset("test_group", "test_topic", 0, 10, "", sarama.ErrNoError),
// 		"OffsetRequest": sarama.NewMockOffsetResponse(t).
// 			SetOffset("test_topic", 0, sarama.OffsetNewest, 10).
// 			SetOffset("test_topic", 0, sarama.OffsetOldest, 7),
// 		"FetchRequest": sarama.NewMockFetchResponse(t, 1).
// 			SetVersion(1).
// 			SetMessage("test_topic", 0, 10, sarama.StringEncoder("foo")).
// 			SetHighWaterMark("test_topic", 0, 14),
// 		"OffsetCommitRequest": sarama.NewMockOffsetCommitResponse(t),
// 		"LeaveGroupRequest": sarama.NewMockWrapper(&sarama.LeaveGroupResponse{
// 			Err: sarama.ErrNoError,
// 		}),
// 	})
// 	c := kafka.NewSourceConfig()
// 	c.Brokers = []string{broker0.Addr()}
// 	c.Topic = "test_topic"
// 	c.GroupID = "test_group"
// 	c.Version = sarama.V2_3_0_0
//
// 	s, err := kafka.NewSource(c)
// 	if err != nil {
// 		panic(err)
// 	}
// 	defer s.Close()
// 	meta := kafka.Metadata{{Topic: "test_topic", Partition: 0, Offset: 10}}
//
// 	time.Sleep(100 * time.Millisecond)
//
// 	_, err = s.Consume()
// 	require.NoError(t, err)
//
// 	err = s.Commit(meta)
//
// 	assert.NoError(t, err)
// }

// TODO
//  	Fix this test. It's failing with an error:
//  			"kafka: error decoding packet: correlation ID didn't match, wanted 8, got 9".
// 		It looks like the mock broker doesn't handle the protocol for the ConsumerGroup correctly,
// 		or some expectations are incorrect/missing.
//
// func TestSource_CommitNilMetadata(t *testing.T) {
// 	broker0 := sarama.NewMockBroker(t, 0)
// 	defer broker0.Close()
// 	broker0.SetHandlerByMap(map[string]sarama.MockResponse{
// 		"MetadataRequest": sarama.NewMockMetadataResponse(t).
// 			SetBroker(broker0.Addr(), broker0.BrokerID()).
// 			SetLeader("test_topic", 0, broker0.BrokerID()),
// 		"FindCoordinatorRequest": sarama.NewMockFindCoordinatorResponse(t).
// 			SetCoordinator(sarama.CoordinatorGroup, "test_group", broker0),
// 		"JoinGroupRequest": sarama.NewMockWrapper(&sarama.JoinGroupResponse{
// 			Version:       1,
// 			Err:           sarama.ErrNoError,
// 			GroupProtocol: "protocol",
// 		}),
// 		"SyncGroupRequest": sarama.NewMockWrapper(&sarama.SyncGroupResponse{
// 			Err: sarama.ErrNoError,
// 			MemberAssignment: []byte{
// 				0, 1, // Version
// 				0, 0, 0, 1, // Topic array length
// 				0, 10, 't', 'e', 's', 't', '_', 't', 'o', 'p', 'i', 'c', // Topic one
// 				0, 0, 0, 1, // Topic one, partition array length
// 				0, 0, 0, 0, // 0
// 				0, 0, 0, 3, 0x01, 0x02, 0x03, // Userdata
// 			},
// 		}),
// 		"OffsetFetchRequest": sarama.NewMockOffsetFetchResponse(t).
// 			SetOffset("test_group", "test_topic", 0, 10, "", sarama.ErrNoError),
// 		"OffsetRequest": sarama.NewMockOffsetResponse(t).
// 			SetOffset("test_topic", 0, sarama.OffsetNewest, 10).
// 			SetOffset("test_topic", 0, sarama.OffsetOldest, 7),
// 		"FetchRequest": sarama.NewMockFetchResponse(t, 1).
// 			SetVersion(1).
// 			SetMessage("test_topic", 0, 10, sarama.StringEncoder("foo")).
// 			SetHighWaterMark("test_topic", 0, 14),
// 		"LeaveGroupRequest": sarama.NewMockWrapper(&sarama.LeaveGroupResponse{
// 			Err: sarama.ErrNoError,
// 		}),
// 	})
// 	c := kafka.NewSourceConfig()
// 	c.Brokers = []string{broker0.Addr()}
// 	c.Topic = "test_topic"
// 	c.GroupID = "test_group"
// 	c.Version = sarama.V2_3_0_0
//
// 	s, err := kafka.NewSource(c)
// 	if err != nil {
// 		panic(err)
// 	}
// 	defer s.Close()
//
// 	time.Sleep(100 * time.Millisecond)
//
// 	_, err = s.Consume()
// 	require.NoError(t, err)
//
// 	err = s.Commit(nil)
//
// 	assert.NoError(t, err)
// }

// TODO
//  	Fix this test. It's failing with an error:
//  			"kafka: error decoding packet: correlation ID didn't match, wanted 8, got 9".
// 		It looks like the mock broker doesn't handle the protocol for the ConsumerGroup correctly,
// 		or some expectations are incorrect/missing.
//
// func TestSource_CommitReturnError(t *testing.T) {
// 	broker0 := sarama.NewMockBroker(t, 0)
// 	defer broker0.Close()
// 	broker0.SetHandlerByMap(map[string]sarama.MockResponse{
// 		"MetadataRequest": sarama.NewMockMetadataResponse(t).
// 			SetBroker(broker0.Addr(), broker0.BrokerID()).
// 			SetLeader("test_topic", 0, broker0.BrokerID()),
// 		"FindCoordinatorRequest": sarama.NewMockFindCoordinatorResponse(t).
// 			SetCoordinator(sarama.CoordinatorGroup, "test_group", broker0),
// 		"JoinGroupRequest": sarama.NewMockWrapper(&sarama.JoinGroupResponse{
// 			Version:       1,
// 			Err:           sarama.ErrNoError,
// 			GroupProtocol: "protocol",
// 		}),
// 		"SyncGroupRequest": sarama.NewMockWrapper(&sarama.SyncGroupResponse{
// 			Err: sarama.ErrNoError,
// 			MemberAssignment: []byte{
// 				0, 1, // Version
// 				0, 0, 0, 1, // Topic array length
// 				0, 10, 't', 'e', 's', 't', '_', 't', 'o', 'p', 'i', 'c', // Topic one
// 				0, 0, 0, 1, // Topic one, partition array length
// 				0, 0, 0, 0, // 0
// 				0, 0, 0, 3, 0x01, 0x02, 0x03, // Userdata
// 			},
// 		}),
// 		"OffsetFetchRequest": sarama.NewMockOffsetFetchResponse(t).
// 			SetOffset("test_group", "test_topic", 0, 10, "", sarama.ErrNoError),
// 		"OffsetRequest": sarama.NewMockOffsetResponse(t).
// 			SetOffset("test_topic", 0, sarama.OffsetNewest, 10).
// 			SetOffset("test_topic", 0, sarama.OffsetOldest, 7),
// 		"FetchRequest": sarama.NewMockFetchResponse(t, 1).
// 			SetVersion(1).
// 			SetMessage("test_topic", 0, 10, sarama.StringEncoder("foo")).
// 			SetHighWaterMark("test_topic", 0, 14),
// 		"OffsetCommitRequest": sarama.NewMockOffsetCommitResponse(t).
// 			SetError("test_group", "test_topic", 0, sarama.ErrBrokerNotAvailable),
// 		"LeaveGroupRequest": sarama.NewMockWrapper(&sarama.LeaveGroupResponse{
// 			Err: sarama.ErrNoError,
// 		}),
// 	})
// 	c := kafka.NewSourceConfig()
// 	c.Brokers = []string{broker0.Addr()}
// 	c.Topic = "test_topic"
// 	c.GroupID = "test_group"
// 	c.Version = sarama.V2_3_0_0
//
// 	s, err := kafka.NewSource(c)
// 	if err != nil {
// 		panic(err)
// 	}
// 	defer s.Close()
// 	meta := kafka.Metadata{{Topic: "test_topic", Partition: 0, Offset: 10}}
//
// 	time.Sleep(100 * time.Millisecond)
//
// 	_, err = s.Consume()
// 	require.NoError(t, err)
//
// 	err = s.Commit(meta)
//
// 	assert.Error(t, err)
// }
