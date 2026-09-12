// Command boom is a fake Repository Locator test fixture: it always exits
// non-zero, exercising the "operational failure halts the chain" outcome
// (locator-failed, 29; ADR-0015 — no continue_on_locator_error).
package main

import (
	"fmt"
	"io"
	"os"
)

func main() {
	io.ReadAll(os.Stdin)
	fmt.Fprintln(os.Stderr, "boom: simulated locator failure")
	os.Exit(1)
}
