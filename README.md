# go-logger
This module is a simple and easy Golang logging module.

# Quick Start
## Install
~~~
go get github.com/breeze7086/go-logger
~~~

## Print logs to stdout
~~~
package main

import "github.com/breeze7086/go-logger"

func main() {
  logger.SetSeverity(logger.DEBUG)
  logger.SetTimeformat("2006/01/02 15:04:05")

  logger.DebugPrintln("This is a DEBUG log")
  logger.InfoPrintln("This is an INFO log")
  logger.WarnPrintf("This is a %s log", "WARN")
  logger.ErrorPrintf("This is an %s log", "ERROR")
}
~~~

## Output
~~~
Set the log level to DEBUG
file:main.go line:10 2022/03/24 11:56:11 [DEBUG] This is a DEBUG log
2022/03/24 11:56:11 [INFO] This is an INFO log
2022/03/24 11:56:11 [WARN] This is a WARN log
2022/03/24 11:56:11 [ERROR] This is an ERROR log
~~~

# Change output from stdout to a file
The default logger instance uses *stdout* as the output location.
You can create a logger instance with a custom `io.Writer` to write logs elsewhere.

## File output
~~~
package main

import (
  "log"
  "os"

  "github.com/breeze7086/go-logger"
)

func main() {
  f, err := os.Create("test.log")
  if err != nil {
    log.Fatal(err)
  }
  defer f.Close()

  l := logger.NewLogger(logger.DEBUG, "2006-01-02 15:04:05", f)
  l.DebugPrintln("This is a DEBUG log")
  l.InfoPrintln("This is an INFO log")
}
~~~

## Output
Logs will be generated in a file *test.log*.
~~~
$ cat test.log
file:main.go line:17 2022-03-24 13:27:07 [DEBUG] This is a DEBUG log
2022-03-24 13:27:07 [INFO] This is an INFO log
~~~

# Mask sensitive values
Use `Mask` when you need to hide a sensitive value before printing it.
~~~
package main

import (
  "os"

  "github.com/breeze7086/go-logger"
)

func main() {
  l := logger.NewLogger(logger.INFO, "2006-01-02 15:04:05", os.Stdout)
  l.InfoPrintf("password=%s", l.Mask("secret-password"))
}
~~~

## Output
~~~
2022-03-24 13:27:07 [INFO] password=******
~~~

# Dump structs and maps
Use `Dump` to print structs, maps, slices, and arrays in a readable format.
`Dump` uses the logger output flow, so it works with stdout, files, and other custom writers.

~~~
package main

import "github.com/breeze7086/go-logger"

type ServiceConfig struct {
  Service string
  Enabled bool
  Ports   []int
}

func main() {
  logger.Dump(ServiceConfig{
    Service: "go-logger",
    Enabled: true,
    Ports:   []int{8080, 8443},
  })

  logger.Dump(map[string]interface{}{
    "service": "go-logger",
    "enabled": true,
  })
}
~~~

# Send logs to syslog
Create a logger instance and call `EnableSyslog` to send logs to the local logging system.
If syslog is not available, the logger keeps its previous output flow.

~~~
package main

import (
  "os"

  "github.com/breeze7086/go-logger"
)

func main() {
  l := logger.NewLogger(logger.INFO, "2006-01-02 15:04:05", os.Stdout)
  l.EnableSyslog("go-logger")
  l.InfoPrintln("This log is sent to syslog when syslog is available")
}
~~~

# Send logs to a Kafka topic
Create a logger instance and call `EnableKafka` with the bootstrap broker addresses
and an existing topic. Each log entry is sent as one Kafka record, including its
timestamp, severity, and trailing newline. `Dump` uses the same output.

~~~go
package main

import (
  "log"
  "os"
  "time"

  "github.com/breeze7086/go-logger"
)

func main() {
  l := logger.NewLogger(logger.INFO, "2006-01-02 15:04:05", os.Stdout)
  writer, err := l.EnableKafka(logger.KafkaConfig{
    Brokers:  []string{"localhost:9092"},
    Topic:    "application-logs",
    ClientID: "go-logger-example",
    Timeout:  5 * time.Second,
    OnError: func(err error) {
      log.Printf("Kafka output failed: %v", err)
    },
  })
  if err != nil {
    log.Fatal(err)
  }
  defer func() {
    if err := writer.Close(); err != nil {
      log.Printf("Close Kafka output: %v", err)
    }
  }()

  l.InfoPrintf("service=%s started", "go-logger")
  l.Dump(map[string]string{"event": "ready"})
}
~~~

`EnableKafka` validates configuration and replaces the logger's output. Broker
connections are established on the first write. Invalid configuration leaves the
previous output in place. The package-level `logger.EnableKafka` configures the
default logger; `logger.NewKafkaWriter` also provides an `io.WriteCloser` for
direct use or for `SetOutflow`.

Writes are synchronous and wait for acknowledgements from all in-sync replicas.
The default timeout is ten seconds. Publish failures are returned by
`KafkaWriter.Write` and passed to `OnError`; without a callback, errors are printed
to stderr. A failed or timed-out publish may still have reached the broker. There
is no automatic fallback to the previous output. Call `Close` at shutdown to
release the producer's connections, and configure output before starting
concurrent logging.

For authenticated clusters, set `KafkaConfig.TLS` to a `*tls.Config` and
`KafkaConfig.SASLMechanism` to a mechanism from `github.com/segmentio/kafka-go/sasl`,
such as `plain.Mechanism` or `scram.Mechanism`. TLS uses Go's normal certificate
validation. Kafka support uses the pure Go `kafka-go v0.4.48` client to retain this
module's Go 1.17 compatibility.

Set `KafkaConfig.Dial` when broker connections need a custom route, such as an
HTTP CONNECT proxy. It opens the raw connection; the Kafka transport then performs
TLS and SASL handshakes. Kafka TCP connections do not automatically use
`HTTP_PROXY` or `HTTPS_PROXY`.

# What's next
| Feature | Status |
| ------- |---|
| Support to mask sensitive information when do the print out | ✅ |
| Support to send logs to a Kafka topic | ✅ |
| Support to send logs to local logging system(syslog, rsyslog) | ✅ |
| Function "Dump" to gracefully print out a struct or a map | ✅ |
