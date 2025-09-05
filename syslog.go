package logger

import (
	syslog "github.com/RackSec/srslog"
)

var logLevelMap = map[logLevel]syslog.Priority{
	DEBUG: syslog.LOG_DEBUG,
	INFO:  syslog.LOG_INFO,
	WARN:  syslog.LOG_WARNING,
	ERROR: syslog.LOG_ERR,
}

func (l *loggerT) EnableSyslog(appTag string) {
	writer, err := syslog.Dial("", "", logLevelMap[l.level], appTag)
	if err != nil {
		ErrorPrintf("Can't connect to syslog, %v", err)
	}

	l.outflow = writer
}
