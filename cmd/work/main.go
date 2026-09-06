// Command work is the entry point for the Work CLI. All behavior lives in
// internal/cli; main only delegates and lets it own the process exit code.
package main

import "github.com/gustaborges/work/internal/cli"

func main() {
	cli.Execute()
}
