// Command echo is a test helper for internal/ipc. It reads and discards stdin,
// then behaves according to environment variables:
//
//	ECHO_MODE=passthrough (default) — copy stdin to stdout, exit 0
//	ECHO_MODE=stdout                — write $ECHO_STDOUT to stdout, exit 0
//	ECHO_MODE=fail                  — write "boom" to stderr, exit 3
//	ECHO_MODE=garbage               — write "not json" to stdout, exit 0
//	ECHO_MODE=silent                — write nothing, exit 0
//	ECHO_MODE=hang                  — block until killed
//	ECHO_MODE=noisy                 — write ~64 KiB then "END" to stderr, exit 0
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "hold-pipes" {
		time.Sleep(4 * time.Second)
		return
	}

	data, _ := io.ReadAll(os.Stdin)
	switch os.Getenv("ECHO_MODE") {
	case "fail":
		fmt.Fprintln(os.Stderr, "boom")
		os.Exit(3)
	case "garbage":
		fmt.Fprint(os.Stdout, "not json")
	case "stdout":
		fmt.Fprint(os.Stdout, os.Getenv("ECHO_STDOUT"))
	case "inherited":
		fmt.Fprint(os.Stdout, os.Getenv("ECHO_STDOUT"))
		cmd := exec.Command(os.Args[0], "hold-pipes")
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Start(); err != nil {
			panic(err)
		}
		_ = cmd.Process.Release()
	case "sized", "continuous", "sized-fail":
		fmt.Fprint(os.Stdout, `{"value":"ok"}`)
		n, _ := strconv.Atoi(os.Getenv("ECHO_BYTES"))
		chunk := strings.Repeat(" ", 32<<10)
		for n > 0 || os.Getenv("ECHO_MODE") == "continuous" {
			count := len(chunk)
			if os.Getenv("ECHO_MODE") != "continuous" && n < count {
				count = n
			}
			if _, err := io.WriteString(os.Stdout, chunk[:count]); err != nil {
				return
			}
			n -= count
		}
		if os.Getenv("ECHO_MODE") == "sized-fail" {
			os.Exit(3)
		}
	case "silent":
		// nothing
	case "hang":
		time.Sleep(time.Hour)
	case "noisy":
		fmt.Fprint(os.Stderr, strings.Repeat("x", 64<<10)+"END")
	default:
		os.Stdout.Write(data)
	}
}
