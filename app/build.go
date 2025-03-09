package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func afterBuild() {
	fmt.Println("Running post-build tasks...")

	// Find libusb
	cmd := exec.Command("brew", "--prefix", "libusb")
	brewPrefix, err := cmd.Output()
	if err != nil {
		fmt.Println("Error finding libusb:", err)
		return
	}

	// Trim newline
	brewPrefixStr := strings.TrimSpace(string(brewPrefix))

	// Find the dylib
	libusbPath := filepath.Join(brewPrefixStr, "lib", "libusb-1.0.dylib")

	// Copy to app bundle
	targetPath := filepath.Join("build", "bin", "app.app", "Contents", "MacOS", "libusb.dylib")
	copyFile(libusbPath, targetPath)

	// Set the correct rpath
	cmd = exec.Command("install_name_tool", "-change", libusbPath, "@executable_path/libusb.dylib",
		filepath.Join("build", "bin", "app.app", "Contents", "MacOS", "app"))
	err = cmd.Run()
	if err != nil {
		fmt.Println("Error updating rpath:", err)
	}

	fmt.Println("Post-build tasks completed!")
}

func copyFile(src, dst string) {
	fmt.Printf("Copying %s to %s\n", src, dst)

	// Make sure target directory exists
	err := os.MkdirAll(filepath.Dir(dst), 0755)
	if err != nil {
		fmt.Println("Error creating destination directory:", err)
		return
	}

	data, err := os.ReadFile(src)
	if err != nil {
		fmt.Println("Error reading source file:", err)
		return
	}

	err = os.WriteFile(dst, data, 0755)
	if err != nil {
		fmt.Println("Error writing target file:", err)
	}
}
