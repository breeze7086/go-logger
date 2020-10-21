package logger

import (
	"fmt"
	"testing"
)

func init() {
	SetSeverity(DEBUG)
}

func Test_DebugPrintln(t *testing.T) {
	fmt.Println("===== Testing DebugPrintln Start =====")
	DebugPrintln("This is the testing string")
}

func Test_InfoPrintln(t *testing.T) {
	fmt.Println("===== Testing InfoPrintln Start =====")
	InfoPrintln("This is the testing string")
}

func Test_WarnPrintln(t *testing.T) {
	fmt.Println("===== Testing WarnPrintln Start =====")
	WarnPrintln("This is the testing string")
}

func Test_ErrorPrintln(t *testing.T) {
	fmt.Println("===== Testing ErrorPrintln Start =====")
	ErrorPrintln("This is the testing string")
}

func Test_FatalPrintln(t *testing.T) {
	fmt.Println("===== Testing FatalPrintln Start =====")
	FatalPrintln("This is the testing string")
}
