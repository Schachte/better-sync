package device

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/ganeshrvel/go-mtpfs/mtp"
	"github.com/ganeshrvel/go-mtpx"
	"github.com/schachte/better-sync/pkg/util"
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

		var dev *mtp.Device
		var err error

		// Save original stdout, stderr, and logger
		oldStdout := os.Stdout
		oldStderr := os.Stderr
		oldLogger := log.Default()

		// Create pipes to capture output
		rOut, wOut, _ := os.Pipe()
		rErr, wErr, _ := os.Pipe()

		// Redirect stdout and stderr
		os.Stdout = wOut
		os.Stderr = wErr

		// Set a null logger to suppress standard Go logs
		log.SetOutput(io.Discard)
		log.SetFlags(0)
		log.SetPrefix("")

		for {
			initOptions := mtpx.Init{DebugMode: false}
			dev, err = mtpx.Initialize(initOptions)
			if err == nil {
				break
			}
			time.Sleep(1 * time.Second)
		}

		// Restore stdout, stderr, and logger
		wOut.Close()
		wErr.Close()
		os.Stdout = oldStdout
		os.Stderr = oldStderr
		log.SetOutput(oldLogger.Writer())
		log.SetFlags(oldLogger.Flags())
		log.SetPrefix(oldLogger.Prefix())

		// Discard captured output
		go io.Copy(io.Discard, rOut)
		go io.Copy(io.Discard, rErr)
		rOut.Close()
		rErr.Close()

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

		// Save original stdout, stderr, and logger
		oldStdout := os.Stdout
		oldStderr := os.Stderr
		oldLogger := log.Default()

		// Create pipes to capture output
		rOut, wOut, _ := os.Pipe()
		rErr, wErr, _ := os.Pipe()

		// Redirect stdout and stderr
		os.Stdout = wOut
		os.Stderr = wErr

		// Set a null logger to suppress standard Go logs
		log.SetOutput(io.Discard)
		log.SetFlags(0)
		log.SetPrefix("")

		initOptions := mtpx.Init{DebugMode: false}
		dev, err := mtpx.Initialize(initOptions)

		// Restore stdout, stderr, and logger
		wOut.Close()
		wErr.Close()
		os.Stdout = oldStdout
		os.Stderr = oldStderr
		log.SetOutput(oldLogger.Writer())
		log.SetFlags(oldLogger.Flags())
		log.SetPrefix(oldLogger.Prefix())

		// Discard captured output
		go io.Copy(io.Discard, rOut)
		go io.Copy(io.Discard, rErr)
		rOut.Close()
		rErr.Close()

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
