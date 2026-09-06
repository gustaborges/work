// Command echo is a test helper for internal/ipc. It reads and discards stdin,
// then behaves according to environment variables:
//
//	ECHO_MODE=passthrough (default) — copy stdin to stdout, exit 0
//	ECHO_MODE=stdout                — write $ECHO_STDOUT to stdout, exit 0
//	ECHO_MODE=fail                  — write "boom" to stderr, exit 3
//	ECHO_MODE=garbage               — write "not json" to stdout, exit 0
//	ECHO_MODE=silent                — write nothing, exit 0
package main

import (
	"fmt"
	"io"
	"os"
)

func main() {
	data, _ := io.ReadAll(os.Stdin)
	switch os.Getenv("ECHO_MODE") {
	case "fail":
		fmt.Fprintln(os.Stderr, "boom")
		os.Exit(3)
	case "garbage":
		fmt.Fprint(os.Stdout, "not json")
	case "stdout":
		fmt.Fprint(os.Stdout, os.Getenv("ECHO_STDOUT"))
	case "silent":
		// nothing
	default:
		os.Stdout.Write(data)
	}
}
