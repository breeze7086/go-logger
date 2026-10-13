package logger

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
)

type stubKafkaProducer struct {
	mutex      sync.Mutex
	messages   []kafka.Message
	writeErr   error
	closeErr   error
	closeCalls int
	onWrite    func(context.Context) error
}

func (p *stubKafkaProducer) WriteMessages(ctx context.Context, messages ...kafka.Message) error {
	p.mutex.Lock()
	p.messages = append(p.messages, messages...)
	p.mutex.Unlock()
	if p.onWrite != nil {
		return p.onWrite(ctx)
	}
	return p.writeErr
}

func (p *stubKafkaProducer) Close() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.closeCalls++
	return p.closeErr
}

func (p *stubKafkaProducer) snapshot() ([]kafka.Message, int) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return append([]kafka.Message(nil), p.messages...), p.closeCalls
}

func newStubKafkaWriter(t *testing.T, producer *stubKafkaProducer, timeout time.Duration, onError func(error)) *KafkaWriter {
	t.Helper()
	writer, err := NewKafkaWriter(KafkaConfig{
		Brokers: []string{"localhost:9092"}, Topic: "test-logs", Timeout: timeout, OnError: onError,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.producer.Close(); err != nil {
		t.Fatal(err)
	}
	writer.producer = producer
	t.Cleanup(func() { _ = writer.Close() })
	return writer
}

func TestKafkaConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		brokers []string
		topic   string
		timeout time.Duration
	}{
		{name: "no brokers", topic: "logs"},
		{name: "empty broker", brokers: []string{""}, topic: "logs"},
		{name: "missing port", brokers: []string{"localhost"}, topic: "logs"},
		{name: "missing host", brokers: []string{":9092"}, topic: "logs"},
		{name: "non-numeric port", brokers: []string{"localhost:kafka"}, topic: "logs"},
		{name: "zero port", brokers: []string{"localhost:0"}, topic: "logs"},
		{name: "port out of range", brokers: []string{"localhost:65536"}, topic: "logs"},
		{name: "invalid second broker", brokers: []string{"localhost:9092", ""}, topic: "logs"},
		{name: "empty topic", brokers: []string{"localhost:9092"}},
		{name: "space in topic", brokers: []string{"localhost:9092"}, topic: "test logs"},
		{name: "reserved topic", brokers: []string{"localhost:9092"}, topic: ".."},
		{name: "topic too long", brokers: []string{"localhost:9092"}, topic: strings.Repeat("a", 250)},
		{name: "negative timeout", brokers: []string{"localhost:9092"}, topic: "logs", timeout: -time.Second},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			writer, err := NewKafkaWriter(KafkaConfig{Brokers: test.brokers, Topic: test.topic, Timeout: test.timeout})
			if err == nil || writer != nil {
				t.Fatalf("expected invalid configuration, got writer=%v err=%v", writer, err)
			}
		})
	}
}

func TestKafkaConfigSecurityAndDefaults(t *testing.T) {
	brokers := []string{"[::1]:9092", "localhost:9093"}
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	mechanism := plain.Mechanism{Username: "test-user", Password: "test-password"}
	writer, err := NewKafkaWriter(KafkaConfig{
		Brokers: brokers, Topic: "service.logs-1", ClientID: "test-client", TLS: tlsConfig, SASLMechanism: mechanism,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	producer := writer.producer.(*kafka.Writer)
	if writer.timeout != 10*time.Second || producer.ReadTimeout != writer.timeout || producer.WriteTimeout != writer.timeout {
		t.Fatalf("unexpected default timeout: writer=%v read=%v write=%v", writer.timeout, producer.ReadTimeout, producer.WriteTimeout)
	}
	if producer.Async || producer.RequiredAcks != kafka.RequireAll || producer.BatchSize != 1 {
		t.Fatal("producer must wait for acknowledgements for each write")
	}
	if producer.AllowAutoTopicCreation {
		t.Fatal("producer must use an existing topic")
	}
	if writer.transport.TLS != tlsConfig || writer.transport.SASL != mechanism || writer.transport.ClientID != "test-client" {
		t.Fatal("TLS, SASL, or client ID was not forwarded to the transport")
	}
	brokers[0] = "changed:1234"
	if strings.Contains(producer.Addr.String(), "changed") {
		t.Fatal("changing the caller's brokers modified the producer")
	}
}

func TestKafkaLoggerOutput(t *testing.T) {
	producer := &stubKafkaProducer{}
	writer := newStubKafkaWriter(t, producer, 0, nil)
	l := NewLogger(INFO, "2006-01-02 15:04:05", writer)
	l.DebugPrintln("filtered message")
	l.InfoPrintf("service=%s count=%d", "go-logger", 3)
	l.WarnPrintln("warning message")
	l.Dump(map[string]string{"service": "go-logger"})
	messages, _ := producer.snapshot()
	if len(messages) != 3 {
		t.Fatalf("expected one record per unfiltered log, got %d", len(messages))
	}
	for i, want := range []string{"[INFO] service=go-logger count=3\n", "[WARN] warning message\n", `"service": "go-logger"`} {
		if !strings.Contains(string(messages[i].Value), want) || !bytes.HasSuffix(messages[i].Value, []byte("\n")) {
			t.Fatalf("record %d did not contain %q: %q", i, want, messages[i].Value)
		}
	}
}

func TestKafkaWriterPreservesPayload(t *testing.T) {
	producer := &stubKafkaProducer{}
	writer := newStubKafkaWriter(t, producer, 0, nil)
	if n, err := writer.Write(nil); n != 0 || err != nil {
		t.Fatalf("empty write: n=%d err=%v", n, err)
	}
	payload := []byte("log line\n")
	if n, err := writer.Write(payload); n != len(payload) || err != nil {
		t.Fatalf("write: n=%d err=%v", n, err)
	}
	payload[0] = 'X'
	messages, _ := producer.snapshot()
	if len(messages) != 1 || string(messages[0].Value) != "log line\n" {
		t.Fatalf("payload was changed or an empty record was sent: %v", messages)
	}
}

func TestKafkaWriterPublishError(t *testing.T) {
	want := errors.New("broker rejected record")
	producer := &stubKafkaProducer{writeErr: want}
	var reported error
	writer := newStubKafkaWriter(t, producer, 0, func(err error) { reported = err })
	n, err := writer.Write([]byte("log"))
	if n != 0 || !errors.Is(err, want) || !errors.Is(reported, want) {
		t.Fatalf("publish error was not returned and reported: n=%d err=%v reported=%v", n, err, reported)
	}
	if !strings.Contains(err.Error(), "test-logs") {
		t.Fatalf("error did not identify the topic: %v", err)
	}
}

func TestKafkaWriterTimeout(t *testing.T) {
	producer := &stubKafkaProducer{onWrite: func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}}
	writer := newStubKafkaWriter(t, producer, 20*time.Millisecond, func(error) {})
	if n, err := writer.Write([]byte("log")); n != 0 || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("write did not use a deadline: n=%d err=%v", n, err)
	}
}

func TestKafkaWriterDefaultErrorReporting(t *testing.T) {
	readEnd, writeEnd, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	previous := os.Stderr
	os.Stderr = writeEnd
	defer func() {
		os.Stderr = previous
		_ = readEnd.Close()
		_ = writeEnd.Close()
	}()
	producer := &stubKafkaProducer{writeErr: errors.New("broker rejected record")}
	writer := newStubKafkaWriter(t, producer, 0, nil)
	if _, err := writer.Write([]byte("log")); err == nil {
		t.Fatal("expected a publish error")
	}
	os.Stderr = previous
	_ = writeEnd.Close()
	output, err := io.ReadAll(readEnd)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(output, []byte("test-logs")) || !bytes.Contains(output, []byte("broker rejected record")) {
		t.Fatalf("default error reporting did not identify the failure: %q", output)
	}
}

func TestKafkaWriterClose(t *testing.T) {
	want := errors.New("close failed")
	producer := &stubKafkaProducer{closeErr: want}
	var reported error
	writer := newStubKafkaWriter(t, producer, 0, func(err error) { reported = err })
	for i := 0; i < 2; i++ {
		if err := writer.Close(); !errors.Is(err, want) {
			t.Fatalf("Close lost its error: %v", err)
		}
	}
	if n, err := writer.Write([]byte("log")); n != 0 || !errors.Is(err, io.ErrClosedPipe) || !errors.Is(reported, io.ErrClosedPipe) {
		t.Fatalf("write after close: n=%d err=%v reported=%v", n, err, reported)
	}
	messages, closeCalls := producer.snapshot()
	if closeCalls != 1 || len(messages) != 0 {
		t.Fatalf("close calls=%d, records after close=%d", closeCalls, len(messages))
	}
}

func TestKafkaErrorCallbackCanCloseWriter(t *testing.T) {
	want := errors.New("publish failed")
	producer := &stubKafkaProducer{writeErr: want}
	var writer *KafkaWriter
	writer = newStubKafkaWriter(t, producer, 0, func(error) { _ = writer.Close() })
	if _, err := writer.Write([]byte("log")); !errors.Is(err, want) {
		t.Fatal(err)
	}
	_, closeCalls := producer.snapshot()
	if closeCalls != 1 {
		t.Fatal("error callback did not close the writer")
	}
}

func TestKafkaWriterConcurrentWritesAndClose(t *testing.T) {
	producer := &stubKafkaProducer{}
	writer := newStubKafkaWriter(t, producer, 0, func(error) {})
	var group sync.WaitGroup
	for i := 0; i < 25; i++ {
		group.Add(2)
		go func() {
			defer group.Done()
			if _, err := writer.Write([]byte("log")); err != nil && !errors.Is(err, io.ErrClosedPipe) {
				t.Errorf("concurrent write: %v", err)
			}
		}()
		go func() {
			defer group.Done()
			if err := writer.Close(); err != nil {
				t.Errorf("concurrent close: %v", err)
			}
		}()
	}
	group.Wait()
	_, closeCalls := producer.snapshot()
	if closeCalls != 1 {
		t.Fatalf("producer closed %d times", closeCalls)
	}
}

func TestKafkaWriterCloseWaitsForWrite(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	producer := &stubKafkaProducer{onWrite: func(ctx context.Context) error {
		close(entered)
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}}
	writer := newStubKafkaWriter(t, producer, 2*time.Second, func(error) {})
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) })
	writeDone, closeDone := make(chan error, 1), make(chan error, 1)
	go func() { _, err := writer.Write([]byte("log")); writeDone <- err }()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("write did not reach the producer")
	}
	go func() { closeDone <- writer.Close() }()
	select {
	case err := <-closeDone:
		t.Fatalf("Close returned before the in-flight write finished: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	releaseOnce.Do(func() { close(release) })
	for _, result := range []<-chan error{writeDone, closeDone} {
		select {
		case err := <-result:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("write or close did not finish")
		}
	}
}

func TestEnableKafka(t *testing.T) {
	var previousOutput bytes.Buffer
	l := NewLogger(INFO, "2006-01-02 15:04:05", &previousOutput)
	if writer, err := l.EnableKafka(KafkaConfig{}); err == nil || writer != nil {
		t.Fatal("invalid configuration was accepted")
	}
	l.InfoPrintln("previous output retained")
	if !strings.Contains(previousOutput.String(), "previous output retained") {
		t.Fatal("invalid configuration replaced the previous output")
	}
	writer, err := l.EnableKafka(KafkaConfig{Brokers: []string{"localhost:9092"}, Topic: "logs"})
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	if l.outflow != writer {
		t.Fatal("EnableKafka did not replace the output")
	}
}

func TestEnableKafkaDefaultLogger(t *testing.T) {
	previous := std.outflow
	defer std.SetOutflow(previous)
	writer, err := EnableKafka(KafkaConfig{Brokers: []string{"localhost:9092"}, Topic: "logs"})
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	if std.outflow != writer {
		t.Fatal("package EnableKafka did not configure the default logger")
	}
}

func TestPrintfFormatsCustomWriterOutput(t *testing.T) {
	var output bytes.Buffer
	l := NewLogger(INFO, "2006-01-02 15:04:05", &output)
	l.InfoPrintf("name=%s count=%d progress=%d%%", "service", 3, 50)
	if !strings.HasSuffix(output.String(), "[INFO] name=service count=3 progress=50%\n") {
		t.Fatalf("Printf did not format custom writer output: %q", output.String())
	}
}
