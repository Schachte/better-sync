package files

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ganeshrvel/go-mtpfs/mtp"
	"github.com/schachte/better-sync/internal/util"
)

// CreatePlaylistFile creates an M3U8 playlist file with the given songs
func CreatePlaylistFile(playlistName string, songs []string, pathStyle int) (string, error) {
	// Create temporary directory if it doesn't exist
	tempDir := filepath.Join(os.TempDir(), "mtpmusic")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Sanitize playlist name
	playlistName = util.SanitizeFolderName(playlistName)

	// Create playlist content
	var content strings.Builder
	content.WriteString("#EXTM3U\n")
	for _, song := range songs {
		// Format the path according to the selected style
		formattedPath := util.FormatPlaylistPath(song, pathStyle)
		content.WriteString(formattedPath)
		content.WriteString("\n")
	}

	// Create the temporary file
	tempFilePath := filepath.Join(tempDir, playlistName+".m3u8")
	err := os.WriteFile(tempFilePath, []byte(content.String()), 0644)
	if err != nil {
		return "", fmt.Errorf("error creating playlist file: %w", err)
	}

	return tempFilePath, nil
}

// EmptyProgressFunc is a no-op progress function
func EmptyProgressFunc(_ int64) error {
	return nil
}

// UploadPlaylistToDevice uploads a playlist file to the device
func UploadPlaylistToDevice(dev *mtp.Device, storageID, parentFolderID uint32, playlistFilePath string) (uint32, error) {
	// Open the playlist file
	file, err := os.Open(playlistFilePath)
	if err != nil {
		return 0, fmt.Errorf("error opening playlist file: %w", err)
	}
	defer file.Close()

	// Get file info
	fileInfo, err := file.Stat()
	if err != nil {
		return 0, fmt.Errorf("error getting file info: %w", err)
	}

	// Get base name
	baseFileName := filepath.Base(playlistFilePath)

	// Create object info
	info := mtp.ObjectInfo{
		StorageID:        storageID,
		ObjectFormat:     0xBA05, // Playlist format
		ParentObject:     parentFolderID,
		Filename:         baseFileName,
		CompressedSize:   uint32(fileInfo.Size()),
		ModificationDate: time.Now(),
	}

	// Send object info - this returns 4 values, but we only need the object ID
	_, _, objectID, err := dev.SendObjectInfo(storageID, parentFolderID, &info)
	if err != nil {
		return 0, fmt.Errorf("error sending playlist info: %w", err)
	}

	// Read file into memory for sending
	data, err := io.ReadAll(file)
	if err != nil {
		return objectID, fmt.Errorf("error reading playlist file: %w", err)
	}

	// Send the file data
	err = dev.SendObject(bytes.NewReader(data), fileInfo.Size(), EmptyProgressFunc)
	if err != nil {
		return objectID, fmt.Errorf("error sending playlist data: %w", err)
	}

	return objectID, nil
}

// RetryUploadPlaylist with improved retry logic
func RetryUploadPlaylist(dev *mtp.Device, storageID, parentFolderID uint32, playlistName string, songs []string, pathStyle int) error {
	// Generate playlist content
	var content strings.Builder
	content.WriteString("#EXTM3U\n")

	for _, song := range songs {
		formattedPath := util.FormatPlaylistPath(song, pathStyle)
		content.WriteString(formattedPath)
		content.WriteString("\n")
	}

	// Create playlist data in memory
	playlistData := []byte(content.String())

	// Create object info
	objectInfo := mtp.ObjectInfo{
		StorageID:        storageID,
		ObjectFormat:     0xBA05, // Playlist format
		ParentObject:     parentFolderID,
		Filename:         playlistName + ".m3u8",
		CompressedSize:   uint32(len(playlistData)),
		ModificationDate: time.Now(),
	}

	// Send object info with retry
	var infoErr error
	for infoAttempt := 1; infoAttempt <= 3; infoAttempt++ {
		_, _, _, infoErr = dev.SendObjectInfo(storageID, parentFolderID, &objectInfo)
		if infoErr == nil {
			break
		}

		util.LogError("SendObjectInfo attempt %d failed: %v", infoAttempt, infoErr)
		fmt.Printf("Info transfer attempt %d failed: %v\n", infoAttempt, infoErr)

		if infoAttempt < 3 {
			time.Sleep(1 * time.Second)
		}
	}

	if infoErr != nil {
		return fmt.Errorf("error sending playlist info after multiple attempts: %w", infoErr)
	}

	// Send object data with retry
	dataSize := int64(len(playlistData))
	var dataErr error

	for dataAttempt := 1; dataAttempt <= 3; dataAttempt++ {
		reader := bytes.NewReader(playlistData)
		dataErr = dev.SendObject(reader, dataSize, EmptyProgressFunc)
		if dataErr == nil {
			break
		}

		util.LogError("SendObject attempt %d failed: %v", dataAttempt, dataErr)
		fmt.Printf("Data transfer attempt %d failed: %v\n", dataAttempt, dataErr)

		if dataAttempt < 3 {
			time.Sleep(1 * time.Second)
		}
	}

	if dataErr != nil {
		return fmt.Errorf("error sending playlist data after multiple attempts: %w", dataErr)
	}

	return nil
}

// TryAlternativeTransferMethod attempts to use an alternative method to send file data
func TryAlternativeTransferMethod(dev *mtp.Device, data []byte, fileSize int64) bool {
	// Send object with a bytes reader - SendObject needs a reader, file size, and progress function
	err := dev.SendObject(bytes.NewReader(data), fileSize, EmptyProgressFunc)
	if err != nil {
		util.LogError("Alternative transfer method failed: %v", err)
		return false
	}
	return true
}
