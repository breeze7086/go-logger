package logger

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
)

// TestKafkaLiveCluster is opt-in: it publishes two records to an existing topic.
// It reads only from the offsets captured before publishing, without a group ID
// or committed offsets. Credentials remain in files outside the repository.
func TestKafkaLiveCluster(t *testing.T) {
	if os.Getenv("GO_LOGGER_KAFKA_INTEGRATION") != "1" {
		t.Skip("set GO_LOGGER_KAFKA_INTEGRATION=1 and the documented Kafka test variables")
	}
	requiredEnv := func(name string) string {
		t.Helper()
		value := os.Getenv(name)
		if value == "" {
			t.Fatalf("%s is required for the live cluster test", name)
		}
		return value
	}
	brokers := strings.Split(requiredEnv("GO_LOGGER_KAFKA_BROKERS"), ",")
	for i := range brokers {
		brokers[i] = strings.TrimSpace(brokers[i])
	}
	topic := requiredEnv("GO_LOGGER_KAFKA_TOPIC")
	certificate, err := kafkaIntegrationCertificate(requiredEnv("GO_LOGGER_KAFKA_CERT"), requiredEnv("GO_LOGGER_KAFKA_KEY"))
	if err != nil {
		t.Fatalf("load client certificate/key: %v", err)
	}
	caPEM, err := os.ReadFile(requiredEnv("GO_LOGGER_KAFKA_CA"))
	if err != nil {
		t.Fatalf("read CA bundle: %v", err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		t.Fatal("CA bundle contains no valid certificates")
	}
	tlsConfig := &tls.Config{
		MinVersion:   tls.VersionTLS12,
		RootCAs:      roots,
		Certificates: []tls.Certificate{certificate},
	}
	dial, err := kafkaIntegrationDial(os.Getenv("GO_LOGGER_KAFKA_PROXY"))
	if err != nil {
		t.Fatal(err)
	}
	const timeout = 15 * time.Second
	transport := &kafka.Transport{
		Dial: dial, TLS: tlsConfig, ClientID: "go-logger-integration",
		DialTimeout: timeout, MetadataTopics: []string{topic},
	}
	defer transport.CloseIdleConnections()
	client := &kafka.Client{Transport: transport, Timeout: timeout}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	var partitions []kafka.Partition
	var clusterID string
	for _, broker := range brokers {
		t.Run(broker, func(t *testing.T) {
			metadata, err := client.Metadata(ctx, &kafka.MetadataRequest{Addr: kafka.TCP(broker), Topics: []string{topic}})
			if err != nil {
				t.Fatalf("TLS/Kafka metadata from %s: %v", broker, err)
			}
			t.Logf("verified TLS/Kafka connection: broker=%s", broker)
			if clusterID != "" && metadata.ClusterID != clusterID {
				t.Fatal("bootstrap brokers returned different cluster IDs")
			}
			clusterID = metadata.ClusterID
			if len(metadata.Topics) != 1 || metadata.Topics[0].Name != topic {
				t.Fatalf("broker %s did not return metadata for %s", broker, topic)
			}
			selected := metadata.Topics[0]
			if selected.Error != nil {
				t.Fatalf("topic metadata from %s: %v", broker, selected.Error)
			}
			if len(selected.Partitions) == 0 {
				t.Fatalf("topic %s has no partitions", topic)
			}
			for _, partition := range selected.Partitions {
				if partition.Error != nil || partition.Leader.Host == "" {
					t.Fatalf("partition %d has no usable leader: %v", partition.ID, partition.Error)
				}
			}
			partitions = selected.Partitions
			t.Logf("TLS/Kafka metadata OK: broker=%s topic=%s partitions=%d", broker, topic, len(partitions))
		})
	}
	// Diagnose every bootstrap broker, then stop before writing if any failed.
	if t.Failed() {
		t.FailNow()
	}

	starts := make(map[int]int64, len(partitions))
	for _, partition := range partitions {
		address := net.JoinHostPort(partition.Leader.Host, strconv.Itoa(partition.Leader.Port))
		offsets, err := client.ListOffsets(ctx, &kafka.ListOffsetsRequest{
			Addr: kafka.TCP(address), Topics: map[string][]kafka.OffsetRequest{topic: {kafka.LastOffsetOf(partition.ID)}},
		})
		if err != nil {
			t.Fatalf("capture end offset of partition %d: %v", partition.ID, err)
		}
		values := offsets.Topics[topic]
		if len(values) != 1 || values[0].Partition != partition.ID || values[0].Error != nil || values[0].LastOffset < 0 {
			t.Fatalf("invalid end offset for partition %d: %+v", partition.ID, values)
		}
		starts[partition.ID] = values[0].LastOffset
		t.Logf("before publish: partition=%d end_offset=%d", partition.ID, values[0].LastOffset)
	}

	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	token := "go-logger-test-" + hex.EncodeToString(random[:])
	var errorMutex sync.Mutex
	var publishErrors []error
	l := NewLogger(INFO, "2006-01-02 15:04:05", io.Discard)
	writer, err := l.EnableKafka(KafkaConfig{
		Brokers: brokers, Topic: topic, ClientID: "go-logger-integration", Timeout: timeout,
		TLS: tlsConfig, Dial: dial,
		OnError: func(err error) {
			errorMutex.Lock()
			publishErrors = append(publishErrors, err)
			errorMutex.Unlock()
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	l.InfoPrintf("test_id=%s kind=printf count=%d", token, 2)
	l.Dump(map[string]interface{}{"test_id": token, "kind": "dump", "count": 2})
	if err := writer.Close(); err != nil {
		t.Fatalf("close producer: %v", err)
	}
	errorMutex.Lock()
	errors := append([]error(nil), publishErrors...)
	errorMutex.Unlock()
	if len(errors) != 0 {
		t.Fatalf("publish failed: %v", errors)
	}
	t.Logf("two records acknowledged by all in-sync replicas: test_id=%s", token)

	readCtx, stopReading := context.WithTimeout(ctx, 45*time.Second)
	defer stopReading()
	messages := make(chan kafka.Message, len(partitions))
	readErrors := make(chan error, len(partitions))
	var readers sync.WaitGroup
	defer func() { stopReading(); readers.Wait() }()
	for _, partition := range partitions {
		reader := kafka.NewReader(kafka.ReaderConfig{
			Brokers: brokers, Topic: topic, Partition: partition.ID,
			Dialer:   &kafka.Dialer{DialFunc: dial, TLS: tlsConfig, Timeout: timeout, ClientID: "go-logger-integration"},
			MinBytes: 1, MaxBytes: 1024 * 1024, MaxWait: time.Second,
		})
		if err := reader.SetOffset(starts[partition.ID]); err != nil {
			_ = reader.Close()
			t.Fatal(err)
		}
		readers.Add(1)
		go func(reader *kafka.Reader) {
			defer readers.Done()
			defer reader.Close()
			for {
				message, err := reader.ReadMessage(readCtx)
				if err != nil {
					if readCtx.Err() == nil {
						readErrors <- err
					}
					return
				}
				if !strings.Contains(string(message.Value), token) {
					continue
				}
				select {
				case messages <- message:
				case <-readCtx.Done():
					return
				}
			}
		}(reader)
	}
	found := make(map[string]bool)
	for len(found) < 2 {
		select {
		case err := <-readErrors:
			t.Fatalf("read test records: %v", err)
		case <-readCtx.Done():
			t.Fatalf("only found %d/2 test records: %v", len(found), readCtx.Err())
		case message := <-messages:
			value := string(message.Value)
			if len(value) < 27 || value[19:27] != " [INFO] " || !strings.HasSuffix(value, "\n") {
				t.Fatalf("test record lost its timestamp/severity/newline: %q", value)
			}
			if _, err := time.Parse("2006-01-02 15:04:05", value[:19]); err != nil {
				t.Fatalf("invalid test record timestamp: %v", err)
			}
			body := value[27:]
			kind := "printf"
			if body != fmt.Sprintf("test_id=%s kind=printf count=2\n", token) {
				var dumped struct {
					TestID string `json:"test_id"`
					Kind   string `json:"kind"`
					Count  int    `json:"count"`
				}
				if err := json.Unmarshal([]byte(body), &dumped); err != nil || dumped.TestID != token || dumped.Kind != "dump" || dumped.Count != 2 {
					t.Fatalf("unexpected test payload: %q", body)
				}
				kind = "dump"
			}
			if found[kind] {
				t.Fatalf("duplicate %s record for test_id=%s", kind, token)
			}
			found[kind] = true
			t.Logf("verified: kind=%s partition=%d offset=%d test_id=%s", kind, message.Partition, message.Offset, token)
		}
	}
}

func TestKafkaIntegrationProxyTunnel(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	serverResult := make(chan error, 1)
	go func() {
		serverResult <- func() error {
			conn, err := listener.Accept()
			if err != nil {
				return err
			}
			defer conn.Close()
			if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
				return err
			}
			reader := bufio.NewReader(conn)
			request, err := http.ReadRequest(reader)
			if err != nil {
				return err
			}
			if request.Method != http.MethodConnect || request.Host != "broker.example.com:9093" {
				return fmt.Errorf("unexpected CONNECT target: method=%s host=%s", request.Method, request.Host)
			}
			if _, err := io.WriteString(conn, "HTTP/1.1 200 Connection established\r\n\r\nhello"); err != nil {
				return err
			}
			var ping [4]byte
			if _, err := io.ReadFull(reader, ping[:]); err != nil {
				return err
			}
			if string(ping[:]) != "ping" {
				return fmt.Errorf("unexpected tunnel payload")
			}
			_, err = io.WriteString(conn, "pong")
			return err
		}()
	}()
	dial, err := kafkaIntegrationDial("http://" + listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, err := dial(ctx, "tcp", "broker.example.com:9093")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	// Canceling a finished dial must not close the established tunnel.
	cancel()
	if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(conn, "ping"); err != nil {
		t.Fatal(err)
	}
	var payload [9]byte
	if _, err := io.ReadFull(conn, payload[:]); err != nil {
		t.Fatal(err)
	}
	if string(payload[:]) != "hellopong" {
		t.Fatalf("proxy lost buffered tunnel bytes: %q", payload)
	}
	if err := <-serverResult; err != nil {
		t.Fatal(err)
	}
}

// OpenSSL is needed only for an encrypted PKCS#8 key. Its output goes directly
// into memory, never to a decrypted file or test output. The password stays in
// an external file rather than command arguments or environment variables.
func kafkaIntegrationCertificate(certFile, keyFile string) (tls.Certificate, error) {
	certificatePEM, err := os.ReadFile(certFile)
	if err != nil {
		return tls.Certificate{}, err
	}
	keyPEM, err := os.ReadFile(keyFile)
	if err != nil {
		return tls.Certificate{}, err
	}
	block, _ := pem.Decode(keyPEM)
	if block != nil && block.Type == "ENCRYPTED PRIVATE KEY" {
		passwordFile := os.Getenv("GO_LOGGER_KAFKA_KEY_PASSWORD_FILE")
		if passwordFile == "" {
			return tls.Certificate{}, fmt.Errorf("encrypted PKCS#8 key requires GO_LOGGER_KAFKA_KEY_PASSWORD_FILE")
		}
		openssl := os.Getenv("GO_LOGGER_KAFKA_OPENSSL")
		if openssl == "" {
			openssl = "openssl"
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		keyPEM, err = exec.CommandContext(ctx, openssl, "pkey", "-in", keyFile, "-passin", "file:"+passwordFile).Output()
		if err != nil {
			return tls.Certificate{}, fmt.Errorf("decrypt private key with OpenSSL: %w", err)
		}
	}
	defer func() {
		for i := range keyPEM {
			keyPEM[i] = 0
		}
	}()
	return tls.X509KeyPair(certificatePEM, keyPEM)
}

type kafkaIntegrationBufferedConn struct {
	net.Conn
	reader *bufio.Reader
}

func (c *kafkaIntegrationBufferedConn) Read(p []byte) (int, error) {
	return c.reader.Read(p)
}

// The proxy opens a raw TCP tunnel; kafka-go performs the verified TLS handshake.
// Return buffered bytes too, since the proxy response and tunneled data may share
// a TCP packet. Both bootstrap and advertised broker connections use this dial.
func kafkaIntegrationDial(proxyURL string) (func(context.Context, string, string) (net.Conn, error), error) {
	if proxyURL == "" {
		return (&net.Dialer{Timeout: 15 * time.Second}).DialContext, nil
	}
	proxy, err := url.Parse(proxyURL)
	if err != nil || proxy.Scheme != "http" || proxy.Host == "" || proxy.User != nil {
		return nil, fmt.Errorf("GO_LOGGER_KAFKA_PROXY must be an HTTP proxy URL without credentials")
	}
	proxyAddress := proxy.Host
	if proxy.Port() == "" {
		proxyAddress = net.JoinHostPort(proxy.Hostname(), "80")
	}
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", proxyAddress)
		if err != nil {
			return nil, err
		}
		ok := false
		defer func() {
			if !ok {
				_ = conn.Close()
			}
		}()
		deadline, _ := ctx.Deadline()
		if err := conn.SetDeadline(deadline); err != nil {
			return nil, err
		}
		done, stopped := make(chan struct{}), make(chan struct{})
		go func() {
			defer close(stopped)
			select {
			case <-ctx.Done():
				_ = conn.Close()
			case <-done:
			}
		}()
		defer func() { close(done); <-stopped }()
		request := &http.Request{Method: http.MethodConnect, URL: &url.URL{Opaque: address}, Host: address}
		if err := request.Write(conn); err != nil {
			return nil, err
		}
		reader := bufio.NewReader(conn)
		response, err := http.ReadResponse(reader, request)
		if err != nil {
			return nil, err
		}
		if response.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("proxy CONNECT %s: %s", address, response.Status)
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := conn.SetDeadline(time.Time{}); err != nil {
			return nil, err
		}
		ok = true
		return &kafkaIntegrationBufferedConn{Conn: conn, reader: reader}, nil
	}, nil
}
