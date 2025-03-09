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

func SetupLogging(verbose bool) {
	verboseOutput = verbose

	err := os.MkdirAll("logs", 0755)
	if err != nil {
		fmt.Println("Warning: Failed to create logs directory:", err)
	}

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	logPath := filepath.Join("logs", fmt.Sprintf("mtpapp_%s.log", timestamp))

	logFile, err := os.Create(logPath)
	if err != nil {
		fmt.Println("Warning: Failed to create log file:", err)

		logger = log.New(os.Stdout, "", log.LstdFlags)
		return
	}

	if verbose {
		multiWriter := io.MultiWriter(logFile, os.Stdout)
		logger = log.New(multiWriter, "", log.LstdFlags)
		LogVerbose("Logging to file: %s and to console", logPath)
	} else {
		logger = log.New(logFile, "", log.LstdFlags)
		LogVerbose("Logging to file: %s", logPath)
	}
}

func LogInfo(format string, v ...interface{}) {
	if logger != nil {
		_, file, line, _ := runtime.Caller(1)
		logger.Printf("[INFO] (%s:%d) "+format, append([]interface{}{filepath.Base(file), line}, v...)...)
	}
}

func LogError(format string, v ...interface{}) {
	if logger != nil {
		_, file, line, _ := runtime.Caller(1)
		logger.Printf("[ERROR] (%s:%d) "+format, append([]interface{}{filepath.Base(file), line}, v...)...)
	}
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", v...)
}

func LogVerbose(format string, v ...interface{}) {
	if verboseOutput && logger != nil {
		_, file, line, _ := runtime.Caller(1)
		logger.Printf("[VERBOSE] (%s:%d) "+format, append([]interface{}{filepath.Base(file), line}, v...)...)
	}
}
