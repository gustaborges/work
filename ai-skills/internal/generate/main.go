package main

import (
	skills "github.com/gustaborges/work/ai-skills"
	"os"
	"path/filepath"
)

func main() {
	for name, data := range skills.Generated() {
		if err := os.WriteFile(filepath.Join("work-plugin-builder/references/generated", name), data, 0644); err != nil {
			panic(err)
		}
	}
}
