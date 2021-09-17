package logger

import (
	"testing"
)

func init() {
	SetSeverity(DEBUG)
}

func Test_DebugPrintln(t *testing.T) {
	DebugPrintln("This is the DEBUG testing string")
}

func Test_InfoPrintln(t *testing.T) {
	InfoPrintln("This is the INFO testing string")
}

func Test_WarnPrintln(t *testing.T) {
	WarnPrintln("This is the WARN testing string")
}

func Test_ErrorPrintln(t *testing.T) {
	ErrorPrintln("This is the ERROR testing string")
}

func Test_FatalPrintln(t *testing.T) {
	FatalPrintln("This is the FATALtesting string")
}
