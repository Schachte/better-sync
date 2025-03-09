// main.go
package main

import (
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	// Simply call the CLI version
	cmd := exec.Command(filepath.Join("cmd", "better-sync", "better-sync"), os.Args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
}
