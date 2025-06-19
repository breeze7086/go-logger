package logger

import (
	"os"
	"testing"
)

func Test_STD_Output(t *testing.T) {
	std.SetSeverity(DEBUG)
	DebugPrintln("This is the println DEBUG testing string")
	InfoPrintln("This is the println INFO testing string")
	WarnPrintln("This is the println WARN testing string")
	ErrorPrintln("This is the println ERROR testing string")

	DebugPrintf("This is the printf DEBUG testing string")
	InfoPrintf("This is the printf INFO testing string")
	WarnPrintf("This is the printf WARN testing string")
	ErrorPrintf("This is the printf ERROR testing string")
}

func Test_STD_Output_with_mask(t *testing.T) {
	std.SetSeverity(DEBUG)
	DebugPrintln("This is the println DEBUG testing string with password: password")
	DebugPrintf("This is the println DEBUG testing string with password: %s", "password")

	DebugPrintln("This is the println DEBUG testing string with password: " + std.Mask("password"))
	DebugPrintf("This is the println DEBUG testing string with password: %s", std.Mask("password"))
}

func Test_File_Output(t *testing.T) {
	f, err := os.Create("test.log")
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	l := NewLogger(DEBUG, "2006-01-02 15:04:05", f)
	l.DebugPrintln("This is the println DEBUG testing string")
	l.InfoPrintln("This is the println INFO testing string")
	l.WarnPrintln("This is the println WARN testing string")
	l.ErrorPrintln("This is the println ERROR testing string")

	l.DebugPrintf("This is the printf DEBUG testing string")
	l.InfoPrintf("This is the printf INFO testing string")
	l.WarnPrintf("This is the printf WARN testing string")
	l.ErrorPrintf("This is the printf ERROR testing string")
}
