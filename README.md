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

# What's next
| Feature | Status |
| ------- |---|
| Support to mask sensitive information when do the print out | Done |
| Support to send logs to a Kafka topic | Planned |
| Support to send logs to local logging system(syslog, rsyslog) | Done |
| Function "Dump" to gracefully print out a struct or a map | Done |
