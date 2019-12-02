package utils

import "fmt"

const DEBUG = false

func DebugPrintf(format string, a ...interface{}) {
	if DEBUG {
		fmt.Printf(format, a)
	}
}

func DebugPrintln(a ...interface{}) {
	if DEBUG {
		fmt.Println(a)
	}
}
