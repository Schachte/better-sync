package util

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

var (
	logger        *log.Logger
	verboseOutput bool
)

// SetupLogging configures the logger with optional verbose output
func SetupLogging(verbose bool) {
	verboseOutput = verbose

	// Create logs directory if it doesn't exist
	err := os.MkdirAll("logs", 0755)
	if err != nil {
		fmt.Println("Warning: Failed to create logs directory:", err)
	}

	// Create a timestamped log file
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	logPath := filepath.Join("logs", fmt.Sprintf("mtpapp_%s.log", timestamp))

	logFile, err := os.Create(logPath)
	if err != nil {
		fmt.Println("Warning: Failed to create log file:", err)
		// If we can't create a log file, just log to stdout
		logger = log.New(os.Stdout, "", log.LstdFlags)
		return
	}

	// Use a multiwriter to send logs to both file and console if verbose
	if verbose {
		multiWriter := io.MultiWriter(logFile, os.Stdout)
		logger = log.New(multiWriter, "", log.LstdFlags)
		fmt.Printf("Logging to file: %s and to console\n", logPath)
	} else {
		logger = log.New(logFile, "", log.LstdFlags)
		fmt.Printf("Logging to file: %s\n", logPath)
	}

	// Log basic system information
	logger.Printf("--- MTP Music Manager Started ---")
	logger.Printf("OS: %s, Architecture: %s", runtime.GOOS, runtime.GOARCH)
	logger.Printf("Verbose logging: %v", verbose)
}

// LogInfo logs informational messages
func LogInfo(format string, v ...interface{}) {
	if logger != nil {
		_, file, line, _ := runtime.Caller(1)
		logger.Printf("[INFO] (%s:%d) "+format, append([]interface{}{filepath.Base(file), line}, v...)...)
	}
}

// LogError logs error messages and also outputs to standard error
func LogError(format string, v ...interface{}) {
	if logger != nil {
		_, file, line, _ := runtime.Caller(1)
		logger.Printf("[ERROR] (%s:%d) "+format, append([]interface{}{filepath.Base(file), line}, v...)...)
	}
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", v...)
}

// LogVerbose logs verbose messages only when verbose logging is enabled
func LogVerbose(format string, v ...interface{}) {
	if verboseOutput && logger != nil {
		_, file, line, _ := runtime.Caller(1)
		logger.Printf("[VERBOSE] (%s:%d) "+format, append([]interface{}{filepath.Base(file), line}, v...)...)
	}
}
