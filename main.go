package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/schachte/better-sync/internal/device"
	"github.com/schachte/better-sync/internal/operations"
	"github.com/schachte/better-sync/internal/util"
)

func main() {
	// Parse command line flags
	verboseFlag := flag.Bool("verbose", false, "Enable verbose logging")
	operationFlag := flag.Int("op", 0, "Operation to perform (0 for menu, 1-10 for specific operation)")
	scanOnlyFlag := flag.Bool("scan", false, "Only scan for MTP devices and exit")
	timeoutSecFlag := flag.Int("timeout", 30, "Timeout in seconds for device initialization")
	flag.Parse()

	// Set up logging
	util.SetupLogging(*verboseFlag)

	util.LogVerbose("Starting MTP Music Manager")

	// Initialize device
	timeout := time.Duration(*timeoutSecFlag) * time.Second
	dev, err := device.Initialize(timeout)
	if err != nil {
		util.LogError("Failed to initialize device: %v", err)
		device.CheckForCommonMTPConflicts(err)
		os.Exit(1)
	}
	defer dev.Close()

	// If scan-only mode, exit after successful initialization
	if *scanOnlyFlag {
		fmt.Println("MTP device successfully detected. Exiting.")
		os.Exit(0)
	}

	// Fetch storages
	storages, err := device.FetchStorages(dev, timeout)
	if err != nil {
		util.LogError("Failed to fetch storages: %v", err)
		os.Exit(1)
	}

	// Handle operations
	operations.Execute(dev, storages, *operationFlag)

	util.LogVerbose("Program completed")
}
