// Command starter is the official reference "local-path-starter" seed component:
// it reads a repository reference from stdin as JSON and resolves it to a local
// path. See specs/001-first-local-work/contracts/ipc-starter.md for the wire
// contract.
package main

import (
	"fmt"
	"os"
)

func main() {
	// TODO: implement the stdin/stdout JSON contract.
	fmt.Fprintln(os.Stderr, "local-path-starter: not implemented")
	os.Exit(1)
}
