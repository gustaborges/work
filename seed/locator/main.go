// Command locator is the official reference "filesystem-repository-locator" seed
// component: it reads a repository query from stdin as JSON and returns matching
// local checkout paths. See
// specs/001-first-local-work/contracts/ipc-repository-locator.md for the wire
// contract.
package main

import (
	"fmt"
	"os"
)

func main() {
	// TODO: implement the stdin/stdout JSON contract.
	fmt.Fprintln(os.Stderr, "filesystem-repository-locator: not implemented")
	os.Exit(1)
}
