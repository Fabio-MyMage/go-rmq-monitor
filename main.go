package main

import (
	"fmt"
	"os"

	"go-rmq-monitor/cmd"
)

var (
	version = "v0.0.1"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// Handle version command
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Printf("go-rmq-monitor version %s\n", version)
		fmt.Printf("commit: %s\n", commit)
		fmt.Printf("built: %s\n", date)
		os.Exit(0)
	}

	cmd.Execute()
}
