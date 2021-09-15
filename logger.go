package logger

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"time"
)

type logLevel int8

// Log level constants
const (
	DEBUG logLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

var severityName = map[logLevel]string{
	DEBUG: "DEBUG",
	INFO:  "INFO",
	WARN:  "WARN",
	ERROR: "ERROR",
	FATAL: "FATAL",
}

type loggerT struct {
	level      logLevel
	timeFormat string
	outflow    io.Writer
	callerSkip int
}

var std *loggerT

func init() {
	std = NewLogger(INFO, "2006-01-02 15:04:05", os.Stdout)
	std.setCallerSkip(4)
}

// SetSeverity Set the log level
func SetSeverity(level interface{}) {
	switch level.(type) {
	case logLevel:
		std.level = level.(logLevel)
	case int8:
		std.level = logLevel(level.(int8))
	}
}

// SetTimeformat Set the time format within log
func SetTimeformat(timeformat string) {
	std.timeFormat = timeformat
}

// GetSeverityName Convert the log level string to constant number value
func GetSeverityName(level logLevel) string {
	return severityName[level]
}

// GetSeverityLevel Get the log level constant number value from level string
func GetSeverityLevel(levelName string) int8 {
	for num, name := range severityName {
		if name == levelName {
			return int8(num)
		}
	}
	return -1
}

// NewLogger New a instance of logger
func NewLogger(level logLevel, timeformat string, outflow io.Writer) *loggerT {
	// Default logger configuration
	return &loggerT{
		level:      level,
		timeFormat: timeformat,
		outflow:    outflow,
		callerSkip: 3,
	}
}

func (l *loggerT) getCaller() (string, int) {
	if _, file, line, ok := runtime.Caller(l.callerSkip); ok {
		f := strings.Split(file, "/")
		return f[len(f)-1], line
	} else {
		return "???", 0
	}
}

func (l *loggerT) SetSeverity(level interface{}) {
	switch level.(type) {
	case logLevel:
		l.level = level.(logLevel)
	case int8:
		l.level = logLevel(level.(int8))
	}
}

func (l *loggerT) SetOutflow(outflow io.Writer) {
	l.outflow = outflow
}

func (l *loggerT) setTimeFormat(timeFormat string) {
	l.timeFormat = timeFormat
}

func (l *loggerT) setCallerSkip(skip int) {
	l.callerSkip = skip
}

func (l *loggerT) logf(level logLevel, format string, v ...interface{}) {
	if l.level <= level {
		var prefix string

		if level < INFO {
			file, line := l.getCaller()
			prefix = fmt.Sprintf("file:%s line:%d %s "+"["+severityName[level]+"] ",
				file, line, time.Now().Format(l.timeFormat))
		} else {
			prefix = fmt.Sprintf("%s ["+severityName[level]+"] ", time.Now().Format(l.timeFormat))
		}
		fmt.Printf(prefix+format+"\n", v...)
	}
}

func (l *loggerT) logln(level logLevel, v ...interface{}) {
	if l.level <= level {
		var prefix string
		var msg = make([]string, len(v)+1)

		if level < INFO {
			file, line := l.getCaller()
			prefix = fmt.Sprintf("file:%s line:%d %s "+"["+severityName[level]+"] ",
				file, line, time.Now().Format(l.timeFormat))
		} else {
			prefix = fmt.Sprintf("%s ["+severityName[level]+"] ", time.Now().Format(l.timeFormat))
		}
		msg = append(msg, prefix)

		for range v {
			msg = append(msg, `%v`)
		}
		s := fmt.Sprintf(strings.Join(msg, ""), v...)
		fmt.Println(s)
	}
}

func (l *loggerT) DebugPrintf(format string, v ...interface{}) {
	l.logf(DEBUG, format, v...)
}

func (l *loggerT) InfoPrintf(format string, v ...interface{}) {
	l.logf(INFO, format, v...)
}

func (l *loggerT) WarnPrintf(format string, v ...interface{}) {
	l.logf(WARN, format, v...)
}

func (l *loggerT) ErrorPrintf(format string, v ...interface{}) {
	l.logf(ERROR, format, v...)
}

func (l *loggerT) FatalPrintf(format string, v ...interface{}) {
	l.logf(FATAL, format, v...)
	os.Exit(1)
}

func (l *loggerT) DebugPrintln(v ...interface{}) {
	l.logln(DEBUG, v...)
}

func (l *loggerT) InfoPrintln(v ...interface{}) {
	l.logln(INFO, v...)
}

func (l *loggerT) WarnPrintln(v ...interface{}) {
	l.logln(WARN, v...)
}

func (l *loggerT) ErrorPrintln(v ...interface{}) {
	l.logln(ERROR, v...)
}

func (l *loggerT) FatalPrintln(v ...interface{}) {
	l.logln(FATAL, v...)
	os.Exit(1)
}

// DebugPrintf std Debug printf function
func DebugPrintf(format string, v ...interface{}) {
	std.DebugPrintf(format, v...)
}

// InfoPrintf std Info printf function
func InfoPrintf(format string, v ...interface{}) {
	std.InfoPrintf(format, v...)
}

// WarnPrintf std Warn printf function
func WarnPrintf(format string, v ...interface{}) {
	std.WarnPrintf(format, v...)
}

// ErrorPrintf std Error printf function
func ErrorPrintf(format string, v ...interface{}) {
	std.ErrorPrintf(format, v...)
}

// FatalPrintf std Fatal printf function
func FatalPrintf(format string, v ...interface{}) {
	std.FatalPrintf(format, v...)
	os.Exit(1)
}

// DebugPrintln std Debug println function
func DebugPrintln(v ...interface{}) {
	std.DebugPrintln(v...)
}

// InfoPrintln std Info println function
func InfoPrintln(v ...interface{}) {
	std.InfoPrintln(v...)
}

// WarnPrintln std Warn println function
func WarnPrintln(v ...interface{}) {
	std.WarnPrintln(v...)
}

// ErrorPrintln std Error println function
func ErrorPrintln(v ...interface{}) {
	std.ErrorPrintln(v...)
}

// FatalPrintln std Fatal println function
func FatalPrintln(v ...interface{}) {
	std.FatalPrintln(v...)
	os.Exit(1)
}
