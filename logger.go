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

func SetSeverity(level interface{}) {
	switch level.(type) {
	case logLevel:
		std.level = level.(logLevel)
	case int8:
		std.level = logLevel(level.(int8))
	}
}

func SetTimeformat(timeformat string) {
	std.timeFormat = timeformat
}

func GetSeverityName(level logLevel) string {
	return severityName[level]
}

func GetSeverityLevel(levelName string) int8 {
	for num, name := range severityName {
		if name == levelName {
			return int8(num)
		}
	}
	return -1
}

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
		file, line := l.getCaller()
		prefix := fmt.Sprintf("file:%s line:%d %s "+"["+severityName[level]+"] ",
			file, line, time.Now().Format(l.timeFormat))
		fmt.Printf(prefix+format+"\n", v...)
	}
}

func (l *loggerT) logln(level logLevel, v ...interface{}) {
	if l.level <= level {
		var msg = make([]string, len(v)+1)
		file, line := l.getCaller()
		prefix := fmt.Sprintf("file:%s line:%d %s "+"["+severityName[level]+"] ",
			file, line, time.Now().Format(l.timeFormat))
		msg = append(msg, prefix)

		for range v {
			msg = append(msg, `%v`)
		}
		s := fmt.Sprintf(strings.Join(msg, ""), v...)
		fmt.Println(s)
	}
}

func (l *loggerT) DebugPrinf(format string, v ...interface{}) {
	l.logf(DEBUG, format, v...)
}

func (l *loggerT) InfoPrinf(format string, v ...interface{}) {
	l.logf(INFO, format, v...)
}

func (l *loggerT) WarnPrinf(format string, v ...interface{}) {
	l.logf(WARN, format, v...)
}

func (l *loggerT) ErrorPrinf(format string, v ...interface{}) {
	l.logf(ERROR, format, v...)
}

func (l *loggerT) FatalPrinf(format string, v ...interface{}) {
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

func DebugPrintf(format string, v ...interface{}) {
	std.DebugPrinf(format, v...)
}

func InfoPrintf(format string, v ...interface{}) {
	std.InfoPrinf(format, v...)
}

func WarnPrintf(format string, v ...interface{}) {
	std.WarnPrinf(format, v...)
}

func ErrorPrintf(format string, v ...interface{}) {
	std.ErrorPrinf(format, v...)
}

func FatalPrintf(format string, v ...interface{}) {
	std.FatalPrinf(format, v...)
	os.Exit(1)
}

func DebugPrintln(v ...interface{}) {
	std.DebugPrintln(v...)
}

func InfoPrintln(v ...interface{}) {
	std.InfoPrintln(v...)
}

func WarnPrintln(v ...interface{}) {
	std.WarnPrintln(v...)
}

func ErrorPrintln(v ...interface{}) {
	std.ErrorPrintln(v...)
}

func FatalPrintln(v ...interface{}) {
	std.FatalPrintln(v...)
	os.Exit(1)
}
