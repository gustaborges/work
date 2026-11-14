package main

import (
	skills "github.com/gustaborges/work/ai-skills"
	"os"
)

func main() {
	if err := os.WriteFile("ai-skills/work-plugin-builder/CONTRACT", []byte(skills.ContractPin()), 0644); err != nil {
		panic(err)
	}
}
