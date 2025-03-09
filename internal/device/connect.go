package device

import (
	"fmt"
	"strings"
	"time"

	"github.com/ganeshrvel/go-mtpfs/mtp"
	"github.com/ganeshrvel/go-mtpx"
	"github.com/schachte/better-sync/internal/util"
)

func Initialize(timeout time.Duration) (*mtp.Device, error) {
	util.LogVerbose("Initializing device with timeout of %v...", timeout)
	fmt.Println("Looking for MTP devices...")

	type initResult struct {
		dev *mtp.Device
		err error
	}

	initCh := make(chan initResult, 1)

	go func() {
		fmt.Println("Starting MTP device detection...")
		util.LogVerbose("Starting MTP device detection in background goroutine")

		initOptions := mtpx.Init{}

		fmt.Println("Calling mtpx.Initialize...")
		dev, err := mtpx.Initialize(initOptions)

		if err != nil {
			util.LogError("MTP initialization failed: %v", err)
			fmt.Printf("MTP initialization failed: %v\n", err)
		} else {
			util.LogVerbose("MTP device found successfully")
			fmt.Println("MTP device found successfully")
		}

		initCh <- initResult{dev, err}
	}()

	fmt.Printf("Waiting for device initialization (timeout: %v)...\n", timeout)
	select {
	case result := <-initCh:
		if result.err != nil {
			return nil, result.err
		}
		util.LogVerbose("Device initialized successfully")
		return result.dev, nil
	case <-time.After(timeout):
		fmt.Println("\nDevice initialization timed out! Possible causes:")
		fmt.Println(" - No MTP device is connected")
		fmt.Println(" - Device is not in MTP mode (check USB connection settings)")
		fmt.Println(" - Device is locked or requires authorization")
		fmt.Println(" - USB connection issues or driver problems")
		fmt.Println(" - Conflicting software might be using the device")
		return nil, fmt.Errorf("initialization timed out after %v", timeout)
	}
}

func CheckForCommonMTPConflicts(err error) {
	if err == nil {
		return
	}

	errMsg := err.Error()

	if strings.Contains(errMsg, "access denied") ||
		strings.Contains(errMsg, "busy") ||
		strings.Contains(errMsg, "in use") ||
		strings.Contains(errMsg, "cannot open device") ||
		strings.Contains(errMsg, "resource unavailable") {

		fmt.Println("\n======================================================")
		fmt.Println("CONNECTION ERROR: Unable to access the MTP device.")
		fmt.Println("\nThis might be caused by other applications that are currently")
		fmt.Println("accessing your device. Please try closing the following programs:")
		fmt.Println("  - Garmin Express")
		fmt.Println("  - Google Drive / Google Backup and Sync")
		fmt.Println("  - Dropbox")
		fmt.Println("  - OneDrive")
		fmt.Println("  - iTunes / Apple Music")
		fmt.Println("  - Windows Explorer / Mac Finder windows showing the device")
		fmt.Println("  - Phone companion apps")
		fmt.Println("  - Any other file synchronization software")
		fmt.Println("\nAfter closing these applications, please try again.")
		fmt.Println("======================================================")
	}
}

func InitializeDeviceWithTimeout(timeout time.Duration) (*mtp.Device, error) {
	util.LogVerbose("Initializing device with timeout of %v...", timeout)
	fmt.Println("Looking for MTP devices...")

	type initResult struct {
		dev *mtp.Device
		err error
	}

	initCh := make(chan initResult, 1)

	go func() {
		fmt.Println("Starting MTP device detection...")
		util.LogVerbose("Starting MTP device detection in background goroutine")

		initOptions := mtpx.Init{}

		fmt.Println("Calling mtpx.Initialize...")
		dev, err := mtpx.Initialize(initOptions)

		if err != nil {
			util.LogError("MTP initialization failed: %v", err)
			fmt.Printf("MTP initialization failed: %v\n", err)
			checkForCommonMTPConflicts(err)
		} else {
			util.LogVerbose("MTP device found successfully")
			fmt.Println("MTP device found successfully")
		}

		initCh <- initResult{dev, err}
	}()

	fmt.Printf("Waiting for device initialization (timeout: %v)...\n", timeout)
	select {
	case result := <-initCh:
		if result.err != nil {
			checkForCommonMTPConflicts(result.err)
			return nil, result.err
		}
		util.LogVerbose("Device initialized successfully")
		return result.dev, nil
	case <-time.After(timeout):
		fmt.Println("\nDevice initialization timed out! Possible causes:")
		fmt.Println(" - No MTP device is connected")
		fmt.Println(" - Device is not in MTP mode (check USB connection settings)")
		fmt.Println(" - Device is locked or requires authorization")
		fmt.Println(" - USB connection issues or driver problems")
		fmt.Println(" - Conflicting software might be using the device")
		return nil, fmt.Errorf("initialization timed out after %v", timeout)
	}
}

func checkForCommonMTPConflicts(err error) {
	errMsg := err.Error()

	if strings.Contains(errMsg, "access denied") ||
		strings.Contains(errMsg, "busy") ||
		strings.Contains(errMsg, "in use") ||
		strings.Contains(errMsg, "cannot open device") ||
		strings.Contains(errMsg, "resource unavailable") {

		fmt.Println("\n======================================================")
		fmt.Println("CONNECTION ERROR: Unable to access the MTP device.")
		fmt.Println("\nThis might be caused by other applications that are currently")
		fmt.Println("accessing your device. Please try closing the following programs:")
		fmt.Println("  - Garmin Express")
		fmt.Println("  - Google Drive / Google Backup and Sync")
		fmt.Println("  - Dropbox")
		fmt.Println("  - OneDrive")
		fmt.Println("  - iTunes / Apple Music")
		fmt.Println("  - Windows Explorer / Mac Finder windows showing the device")
		fmt.Println("  - Phone companion apps")
		fmt.Println("  - Any other file synchronization software")
		fmt.Println("\nAfter closing these applications, please try again.")
		fmt.Println("======================================================")
	}
}
