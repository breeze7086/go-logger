# go-logger
This module supposes to be a simple and easy Golang module.

# Quick Start
## Install
~~~
go get github.com/breeze7086/go-logger
~~~
## Sample
~~~
import github.com/breeze7086/go-logger

func main() {
  logger.SetSeverity(logger.DEBUG)
  logger.SetTimeformat("2006/01/02 15:04:05")
  logger.DebugPrintln("This a DEBUG log")
  logger.InfoPrintln("This is a INFO log")
}
~~~
## Output
~~~
Set the log level to DEBUG
file:main.go line:10 2022/03/24 11:56:11 [DEBUG] This a DEBUG log
2022/03/24 11:56:11 [INFO] This is a INFO log
~~~

# Change output from stdout to a file
The default logger instance will use *stdout* as default output location.  
However, go-logger supports to change the output stream to other location.  
In this case, you need to construct the logger instance by yourself to specify the output location.  
## File
~~~
import github.com/breeze7086/go-logger

func main() {
  // Create a file
  f, err := os.Create("test.log")
	if err != nil {
		t.Fatal(err)
	}

  defer func() {
    if err := f.Close(); err != nil {
	  t.Fatal(err)
	}
  }()

  // Create an logger instance with some initial parameters
  // The third parameter "f" is the output flow you will use via this logger instance
  // You can also set the output flow by calling the instance funtion l.SetOutflow(outflow io.Writer)
  l := NewLogger(DEBUG, "2006-01-02 15:04:05", f)
  l.DebugPrintln("This is the println DEBUG testing string")
  l.InfoPrintln("This is the println INFO testing string")
}
~~~
## Output  
Logs will be generated in a file *test.log*
~~~
$ cat test.log
file:main.go line:17 2022-03-24 13:27:07 [DEBUG] This is a DEBUG log
2022-03-24 13:27:07 [INFO] This is a INFO log
~~~

# What's next
1. Support customize log output structure
2. Support to send logs to a Kafka topic