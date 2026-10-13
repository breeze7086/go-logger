package logger

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl"
)

// KafkaConfig configures a logger's Kafka output.
type KafkaConfig struct {
	// Brokers contains bootstrap addresses in host:port form.
	Brokers []string
	// Topic must already exist in the Kafka cluster.
	Topic string
	// ClientID identifies the producer to the brokers.
	ClientID string
	// Timeout limits each publish operation. Zero uses ten seconds.
	Timeout time.Duration
	// TLS and SASLMechanism enable encrypted and authenticated connections.
	TLS           *tls.Config
	SASLMechanism sasl.Mechanism
	// OnError receives publish errors, including writes after Close. When nil,
	// errors are printed to stderr. It may be called concurrently.
	OnError func(error)
}

type kafkaProducer interface {
	WriteMessages(context.Context, ...kafka.Message) error
	Close() error
}

// KafkaWriter publishes each nonempty Write as one Kafka record. Successful
// writes wait for all in-sync replicas to acknowledge the record. Use
// NewKafkaWriter to construct it and Close to release its connections.
type KafkaWriter struct {
	producer  kafkaProducer
	transport *kafka.Transport
	topic     string
	timeout   time.Duration
	onError   func(error)
	mutex     sync.RWMutex
	closed    bool
	closeErr  error
}

var _ io.WriteCloser = (*KafkaWriter)(nil)

// NewKafkaWriter validates config and creates a synchronous Kafka writer.
// Connections are established on the first write, not during construction.
func NewKafkaWriter(config KafkaConfig) (*KafkaWriter, error) {
	if len(config.Brokers) == 0 {
		return nil, fmt.Errorf("logger: Kafka requires at least one broker")
	}
	for _, broker := range config.Brokers {
		host, port, err := net.SplitHostPort(broker)
		if err != nil || strings.TrimSpace(host) == "" || strings.TrimSpace(host) != host {
			return nil, fmt.Errorf("logger: invalid Kafka broker %q: expected host:port", broker)
		}
		portNumber, err := strconv.Atoi(port)
		if err != nil || portNumber < 1 || portNumber > 65535 {
			return nil, fmt.Errorf("logger: invalid Kafka broker port in %q", broker)
		}
	}
	if config.Topic == "" || len(config.Topic) > 249 || config.Topic == "." || config.Topic == ".." {
		return nil, fmt.Errorf("logger: invalid Kafka topic %q", config.Topic)
	}
	for _, character := range config.Topic {
		if !((character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') || strings.ContainsRune("._-", character)) {
			return nil, fmt.Errorf("logger: invalid Kafka topic %q", config.Topic)
		}
	}
	if config.Timeout < 0 {
		return nil, fmt.Errorf("logger: Kafka timeout must not be negative")
	}
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}
	if config.OnError == nil {
		config.OnError = func(err error) {
			_, _ = fmt.Fprintln(os.Stderr, err)
		}
	}

	transport := &kafka.Transport{
		ClientID:       config.ClientID,
		DialTimeout:    config.Timeout,
		MetadataTopics: []string{config.Topic},
		TLS:            config.TLS,
		SASL:           config.SASLMechanism,
	}
	producer := &kafka.Writer{
		Addr:         kafka.TCP(append([]string(nil), config.Brokers...)...),
		Topic:        config.Topic,
		Balancer:     &kafka.LeastBytes{},
		BatchSize:    1,
		MaxAttempts:  1,
		ReadTimeout:  config.Timeout,
		WriteTimeout: config.Timeout,
		RequiredAcks: kafka.RequireAll,
		Transport:    transport,
	}
	return &KafkaWriter{
		producer:  producer,
		transport: transport,
		topic:     config.Topic,
		timeout:   config.Timeout,
		onError:   config.OnError,
	}, nil
}

// Write publishes p unchanged as a single record, including any trailing newline.
// A publish error returns zero bytes written and is also passed to OnError.
func (w *KafkaWriter) Write(p []byte) (int, error) {
	w.mutex.RLock()
	if w.closed {
		w.mutex.RUnlock()
		return 0, w.reportError(io.ErrClosedPipe)
	}
	if len(p) == 0 {
		w.mutex.RUnlock()
		return 0, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), w.timeout)
	// Kafka may finish a timed-out request in the background. Copy the caller's
	// buffer so it can safely be reused as soon as Write returns.
	err := w.producer.WriteMessages(ctx, kafka.Message{Value: append([]byte(nil), p...)})
	cancel()
	w.mutex.RUnlock()
	if err != nil {
		return 0, w.reportError(err)
	}
	return len(p), nil
}

func (w *KafkaWriter) reportError(err error) error {
	err = fmt.Errorf("logger: publish to Kafka topic %q: %w", w.topic, err)
	// Run the callback without the mutex so it may safely close the writer.
	w.onError(err)
	return err
}

// Close waits for in-flight writes and releases the producer's connections.
// It is safe to call repeatedly or concurrently; later writes fail.
func (w *KafkaWriter) Close() error {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	if !w.closed {
		w.closed = true
		w.closeErr = w.producer.Close()
		w.transport.CloseIdleConnections()
	}
	return w.closeErr
}

// EnableKafka replaces this logger's output with a new KafkaWriter. Configure
// it before logging concurrently, and close the returned writer on shutdown.
// Invalid configuration leaves the logger's existing output in place.
func (l *loggerT) EnableKafka(config KafkaConfig) (*KafkaWriter, error) {
	writer, err := NewKafkaWriter(config)
	if err != nil {
		return nil, err
	}
	l.SetOutflow(writer)
	return writer, nil
}

// EnableKafka configures Kafka output for the package's default logger.
func EnableKafka(config KafkaConfig) (*KafkaWriter, error) {
	return std.EnableKafka(config)
}
