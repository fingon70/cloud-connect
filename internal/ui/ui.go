package ui

import (
	"fmt"
	"os"
)

func Infof(format string, args ...any) {
	fmt.Printf(format+"\n", args...)
}

func Errorf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}
