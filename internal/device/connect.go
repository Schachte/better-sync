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

	type initResult struct {
		dev *mtp.Device
		err error
	}

	initCh := make(chan initResult, 1)

	go func() {
		util.LogVerbose("Starting MTP device detection in background goroutine")

		initOptions := mtpx.Init{}
		dev, err := mtpx.Initialize(initOptions)

		if err != nil {
			util.LogError("MTP initialization failed: %v", err)
		} else {
			util.LogVerbose("MTP device found successfully")
		}

		initCh <- initResult{dev, err}
	}()

	select {
	case result := <-initCh:
		if result.err != nil {
			return nil, result.err
		}
		util.LogVerbose("Device initialized successfully")
		return result.dev, nil
	case <-time.After(timeout):
		CheckForCommonMTPConflicts(fmt.Errorf("initialization timed out after %v", timeout))
		return nil, fmt.Errorf("initialization timed out after %v", timeout)
	}
}

func InitializeDeviceWithTimeout(timeout time.Duration) (*mtp.Device, error) {
	util.LogVerbose("Initializing device with timeout of %v...", timeout)

	type initResult struct {
		dev *mtp.Device
		err error
	}

	initCh := make(chan initResult, 1)

	go func() {
		util.LogVerbose("Starting MTP device detection in background goroutine")

		initOptions := mtpx.Init{DebugMode: false}

		dev, err := mtpx.Initialize(initOptions)

		if err != nil {
			util.LogError("MTP initialization failed: %v", err)
			fmt.Printf("MTP initialization failed: %v\n", err)
			CheckForCommonMTPConflicts(err)
		} else {
			util.LogVerbose("MTP device found successfully")
		}

		initCh <- initResult{dev, err}
	}()

	select {
	case result := <-initCh:
		if result.err != nil {
			CheckForCommonMTPConflicts(result.err)
			return nil, result.err
		}
		util.LogVerbose("Device initialized successfully")
		return result.dev, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("initialization timed out after %v", timeout)
	}
}

func CheckForCommonMTPConflicts(err error) {
	errMsg := err.Error()

	if strings.Contains(errMsg, "access denied") ||
		strings.Contains(errMsg, "busy") ||
		strings.Contains(errMsg, "in use") ||
		strings.Contains(errMsg, "cannot open device") ||
		strings.Contains(errMsg, "LIBUSB_ERROR_NOT_FOUND") ||
		strings.Contains(errMsg, "resource unavailable") {

		fmt.Println("\n\033[36m======================================================\033[0m")
		fmt.Println("\033[31m❌ CONNECTION ERROR: Unable to access the MTP device ❌\033[0m")
		fmt.Println("\n\033[33m⚠️  This might be caused by other applications that are currently")
		fmt.Println("accessing your device. Please try closing the following programs:\033[0m")
		fmt.Println("\033[36m  📱 Garmin Express")
		fmt.Println("  ☁️  Google Drive / Google Backup and Sync")
		fmt.Println("  📦 Dropbox")
		fmt.Println("  ☁️  OneDrive")
		fmt.Println("  🎵 iTunes / Apple Music")
		fmt.Println("  📂 Windows Explorer / Mac Finder windows showing the device")
		fmt.Println("  📱 Phone companion apps")
		fmt.Println("  🔄 Any other file synchronization software\033[0m")
		fmt.Println("\n\033[32m✨ After closing these applications, please try again.\033[0m")
		fmt.Println("\033[36m======================================================\033[0m")
	}
}
