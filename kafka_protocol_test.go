package logger

import (
	"bufio"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/protocol"
	"github.com/segmentio/kafka-go/protocol/apiversions"
	"github.com/segmentio/kafka-go/protocol/metadata"
	"github.com/segmentio/kafka-go/protocol/produce"
)

type receivedKafkaRecord struct {
	topic    string
	value    string
	acks     int16
	clientID string
}

// Exercise the real Kafka client over TCP, including metadata discovery,
// record encoding, and broker acknowledgements, without an external cluster.
func startKafkaProtocolBroker(t *testing.T, topic string, errorCode int16) (string, <-chan receivedKafkaRecord) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := int32(listener.Addr().(*net.TCPAddr).Port)
	records := make(chan receivedKafkaRecord, 16)
	var group sync.WaitGroup
	var mutex sync.Mutex
	connections := make(map[net.Conn]struct{})
	group.Add(1)
	go func() {
		defer group.Done()
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			mutex.Lock()
			connections[conn] = struct{}{}
			mutex.Unlock()
			group.Add(1)
			go func() {
				defer group.Done()
				defer conn.Close()
				defer func() {
					mutex.Lock()
					delete(connections, conn)
					mutex.Unlock()
				}()
				reader := bufio.NewReader(conn)
				for {
					// The protocol decoder reports an unexpected EOF even when a
					// connection closes between requests. Check the boundary first.
					if _, err := reader.Peek(1); err != nil {
						if !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) {
							t.Errorf("read Kafka request boundary: %v", err)
						}
						return
					}
					version, correlation, clientID, request, err := protocol.ReadRequest(reader)
					if err != nil {
						if !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) {
							t.Errorf("read Kafka request: %v", err)
						}
						return
					}
					var response protocol.Message
					switch request := request.(type) {
					case *apiversions.Request:
						response = &apiversions.Response{ApiKeys: []apiversions.ApiKeyResponse{
							{ApiKey: int16(protocol.ApiVersions), MinVersion: 0, MaxVersion: 2},
							{ApiKey: int16(protocol.Metadata), MinVersion: 0, MaxVersion: 8},
							{ApiKey: int16(protocol.Produce), MinVersion: 3, MaxVersion: 3},
						}}
					case *metadata.Request:
						response = &metadata.Response{
							Brokers:      []metadata.ResponseBroker{{NodeID: 1, Host: "127.0.0.1", Port: port}},
							ControllerID: 1,
							Topics: []metadata.ResponseTopic{{Name: topic, Partitions: []metadata.ResponsePartition{{
								PartitionIndex: 0, LeaderID: 1, ReplicaNodes: []int32{1}, IsrNodes: []int32{1},
							}}}},
						}
					case *produce.Request:
						result := &produce.Response{}
						for _, requestTopic := range request.Topics {
							responseTopic := produce.ResponseTopic{Topic: requestTopic.Topic}
							for _, partition := range requestTopic.Partitions {
								for {
									record, err := partition.RecordSet.Records.ReadRecord()
									if errors.Is(err, io.EOF) {
										break
									}
									if err != nil {
										t.Errorf("decode Kafka record: %v", err)
										return
									}
									value, err := io.ReadAll(record.Value)
									if err != nil {
										t.Errorf("read Kafka record value: %v", err)
										return
									}
									records <- receivedKafkaRecord{topic: requestTopic.Topic, value: string(value), acks: request.Acks, clientID: clientID}
								}
								responseTopic.Partitions = append(responseTopic.Partitions, produce.ResponsePartition{
									Partition: partition.Partition, ErrorCode: errorCode,
								})
							}
							result.Topics = append(result.Topics, responseTopic)
						}
						response = result
					default:
						t.Errorf("unexpected Kafka request %T", request)
						return
					}
					if err := protocol.WriteResponse(conn, version, correlation, response); err != nil {
						if !errors.Is(err, net.ErrClosed) {
							t.Errorf("write Kafka response: %v", err)
						}
						return
					}
				}
			}()
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		mutex.Lock()
		for conn := range connections {
			_ = conn.Close()
		}
		mutex.Unlock()
		group.Wait()
	})
	return listener.Addr().String(), records
}

func receiveKafkaRecord(t *testing.T, records <-chan receivedKafkaRecord) receivedKafkaRecord {
	t.Helper()
	select {
	case record := <-records:
		return record
	case <-time.After(3 * time.Second):
		t.Fatal("broker did not receive the record")
		return receivedKafkaRecord{}
	}
}

func TestKafkaProtocolPublishing(t *testing.T) {
	address, records := startKafkaProtocolBroker(t, "application-logs", 0)
	l := NewLogger(INFO, "2006-01-02 15:04:05", io.Discard)
	writer, err := l.EnableKafka(KafkaConfig{
		Brokers: []string{address}, Topic: "application-logs", ClientID: "protocol-test", Timeout: 3 * time.Second,
		OnError: func(err error) { t.Errorf("Kafka output: %v", err) },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	l.DebugPrintln("filtered message")
	l.InfoPrintf("service=%s count=%d", "go-logger", 3)
	l.WarnPrintln("warning message")
	l.Dump(map[string]string{"event": "ready"})
	for _, want := range []string{"[INFO] service=go-logger count=3\n", "[WARN] warning message\n", `"event": "ready"`} {
		record := receiveKafkaRecord(t, records)
		if record.topic != "application-logs" || record.clientID != "protocol-test" || record.acks != -1 || !strings.Contains(record.value, want) {
			t.Fatalf("unexpected Kafka record: %+v, wanted %q", record, want)
		}
	}
	select {
	case record := <-records:
		t.Fatalf("unexpected record for filtered log: %+v", record)
	default:
	}
}

func TestKafkaProtocolBrokerRejection(t *testing.T) {
	address, records := startKafkaProtocolBroker(t, "application-logs", int16(kafka.TopicAuthorizationFailed))
	var reported error
	writer, err := NewKafkaWriter(KafkaConfig{
		Brokers: []string{address}, Topic: "application-logs", Timeout: 3 * time.Second,
		OnError: func(err error) { reported = err },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	n, err := writer.Write([]byte("test record"))
	var writeErrors kafka.WriteErrors
	if n != 0 || !errors.As(err, &writeErrors) || len(writeErrors) != 1 || writeErrors[0] != kafka.TopicAuthorizationFailed || reported != err {
		t.Fatalf("broker rejection was not returned and reported: n=%d err=%v reported=%v", n, err, reported)
	}
	if record := receiveKafkaRecord(t, records); record.value != "test record" {
		t.Fatalf("unexpected rejected record: %+v", record)
	}
}
