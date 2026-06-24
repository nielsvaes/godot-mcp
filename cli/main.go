package main

import (
	"os"

	"github.com/tomyud1/godot-mcp/cli/cmd"
)

func main() {
	os.Exit(cmd.Execute())
}
