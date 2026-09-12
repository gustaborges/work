// Command empty is a fake Repository Locator test fixture: it always returns
// {"matches": []}, exercising the chain-of-responsibility "continue to the
// next eligible Locator" signal (research R6).
package main

import (
	"encoding/json"
	"io"
	"os"
)

func main() {
	io.ReadAll(os.Stdin)
	json.NewEncoder(os.Stdout).Encode(map[string]any{"matches": []map[string]string{}})
}
