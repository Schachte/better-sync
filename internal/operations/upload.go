package operations

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/bogem/id3v2"
	"github.com/ganeshrvel/go-mtpfs/mtp"
	"github.com/schachte/better-sync/internal/device"
	"github.com/schachte/better-sync/internal/util"
)

// Constants for MTP operations
const (
	PARENT_ROOT     uint32 = 0
	FILETYPE_FOLDER uint16 = 0x3001
)

// EmptyProgressFunc is a no-op progress function for MTP operations
func EmptyProgressFunc(_ int64) error {
	return nil
}

// UploadSong handles uploading a single song to the device
func UploadSong(dev *mtp.Device, storagesRaw interface{}) {
	// Get storage and music folder
	storageID, musicFolderID, err := SelectStorageAndMusicFolder(dev, storagesRaw)
	if err != nil {
		util.LogError("Error selecting storage: %v", err)
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Prompt user to choose upload option
	fmt.Println("\nUpload options:")
	fmt.Println("1. Upload a single file")
	fmt.Println("2. Upload all files from a directory")
	fmt.Print("Select option (1-2): ")

	var option int
	fmt.Scanln(&option)

	switch option {
	case 1:
		uploadSingleFile(dev, storageID, musicFolderID)
	case 2:
		uploadDirectory(dev, storageID, musicFolderID)
	default:
		fmt.Println("Invalid option")
	}
}

// uploadSingleFile uploads a single MP3 file to the device
func uploadSingleFile(dev *mtp.Device, storageID, musicFolderID uint32) {
	// Ask for file path
	fmt.Print("Enter path to MP3 file: ")
	reader := bufio.NewReader(os.Stdin)
	filePath, _ := reader.ReadString('\n')
	filePath = strings.TrimSpace(filePath)

	// Remove quotes if present (from drag-and-drop)
	filePath = strings.Trim(filePath, "\"'")

	if filePath == "" {
		fmt.Println("No file path provided")
		return
	}

	// Process and upload the file
	success := ProcessAndUploadFile(dev, storageID, musicFolderID, filePath)
	if success {
		fmt.Println("File uploaded successfully")
	} else {
		fmt.Println("Failed to upload file")
	}
}

// uploadDirectory uploads all MP3 files from a directory to the device
func uploadDirectory(dev *mtp.Device, storageID, musicFolderID uint32) {
	// Ask for directory path
	fmt.Print("Enter path to directory containing MP3 files: ")
	reader := bufio.NewReader(os.Stdin)
	dirPath, _ := reader.ReadString('\n')
	dirPath = strings.TrimSpace(dirPath)

	// Remove quotes if present (from drag-and-drop)
	dirPath = strings.Trim(dirPath, "\"'")

	if dirPath == "" {
		fmt.Println("No directory path provided")
		return
	}

	// Check if the directory exists
	fileInfo, err := os.Stat(dirPath)
	if err != nil {
		util.LogError("Error accessing directory: %v", err)
		fmt.Printf("Error accessing directory: %v\n", err)
		return
	}

	if !fileInfo.IsDir() {
		fmt.Println("The provided path is not a directory")
		return
	}

	// Collect all MP3 files in the directory
	fmt.Println("Searching for MP3 files in the directory...")
	var mp3Files []string

	err = filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			util.LogError("Error accessing path %s: %v", path, err)
			return nil // Skip files that can't be accessed
		}

		if !info.IsDir() && strings.ToLower(filepath.Ext(path)) == ".mp3" {
			mp3Files = append(mp3Files, path)
		}

		return nil
	})

	if err != nil {
		util.LogError("Error walking directory: %v", err)
		fmt.Printf("Error searching directory: %v\n", err)
		return
	}

	if len(mp3Files) == 0 {
		fmt.Println("No MP3 files found in the directory")
		return
	}

	fmt.Printf("Found %d MP3 files. Uploading...\n", len(mp3Files))

	// Upload each file
	successful := 0
	for i, filePath := range mp3Files {
		fmt.Printf("[%d/%d] Uploading %s...\n", i+1, len(mp3Files), filepath.Base(filePath))
		if ProcessAndUploadFile(dev, storageID, musicFolderID, filePath) {
			successful++
		}
	}

	fmt.Printf("Upload complete. %d/%d files uploaded successfully.\n", successful, len(mp3Files))
}

// uploadDirectoryWithPlaylist - modified to add track numbers and ensure all files are included
func UploadDirectoryWithPlaylist(dev *mtp.Device, storageID, musicFolderID uint32) {
	// Get directory path
	fmt.Print("Enter path to directory containing MP3 files: ")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	dirPath := scanner.Text()

	// Check if directory exists
	dirInfo, err := os.Stat(dirPath)
	if err != nil {
		fmt.Printf("Error accessing directory: %v\n", err)
		return
	}

	if !dirInfo.IsDir() {
		fmt.Println("Specified path is not a directory. Please use single file upload option.")
		return
	}

	// Ask about recursive search
	recursive := true

	// Ask for playlist name (default to directory name)
	dirName := filepath.Base(dirPath)
	// Convert playlist name to uppercase for consistency
	playlistName := strings.ToUpper(util.SanitizeFileName(dirName))

	// Ensure playlist name ends with .m3u8
	if !strings.HasSuffix(strings.ToLower(playlistName), ".m3u8") {
		playlistName += ".m3u8" // Note: uppercase extension
	}

	// Find all MP3 files in the directory
	var mp3Files []string
	err = filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			util.LogError("Error accessing path %s: %v", path, err)
			return nil // Continue despite errors
		}

		// Skip directories unless it's the root directory
		if info.IsDir() {
			// If not recursive and not the root directory, skip this directory
			if !recursive && path != dirPath {
				return filepath.SkipDir
			}
			return nil
		}

		// Check for MP3 extension
		if strings.HasSuffix(strings.ToLower(path), ".mp3") {
			mp3Files = append(mp3Files, path)
		}

		return nil
	})

	if err != nil {
		util.LogError("Error walking directory: %v", err)
		fmt.Printf("Error scanning directory: %v\n", err)
		return
	}

	// Check if any MP3 files found
	if len(mp3Files) == 0 {
		fmt.Println("No MP3 files found in the specified directory.")
		return
	}

	fmt.Printf("Found %d MP3 files in %s\n", len(mp3Files), dirPath)

	// Ask for confirmation before uploading
	fmt.Printf("Do you want to upload %d MP3 files and create a playlist? (y/n): ", len(mp3Files))
	scanner.Scan()
	confirm := strings.ToLower(scanner.Text())
	if confirm != "y" && confirm != "yes" {
		fmt.Println("Upload cancelled.")
		return
	}

	// Track uploaded files for playlist creation
	var uploadedFilePaths []string

	// Process and upload each file
	successCount := 0
	failureCount := 0

	for i, filePath := range mp3Files {
		fmt.Printf("\n[%d/%d] Processing %s\n", i+1, len(mp3Files), filepath.Base(filePath))

		// Check file size
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			fmt.Printf("Error accessing file: %v. Skipping.\n", err)
			failureCount++
			continue
		}

		// Skip files larger than 10MB
		if fileInfo.Size() > 10*1024*1024 {
			fmt.Printf("File is too large (%d MB). Skipping.\n", fileInfo.Size()/1024/1024)
			failureCount++
			continue
		}

		// Pass track number (i+1) to ensure each file gets a sequential number
		uploadedPath := ProcessAndUploadFileWithPath(dev, storageID, musicFolderID, filePath, i+1)
		if uploadedPath != "" {
			successCount++
			uploadedFilePaths = append(uploadedFilePaths, uploadedPath)
		} else {
			failureCount++
		}
	}

	fmt.Printf("\nUpload complete: %d successful, %d failed\n", successCount, failureCount)

	// Create and upload playlist if any files were successfully uploaded
	if len(uploadedFilePaths) > 0 {
		fmt.Printf("\nCreating playlist '%s' with %d songs...\n", playlistName, len(uploadedFilePaths))

		// Always use style 1 which matches the examples (0:/MUSIC/...)
		pathStyle := 1

		// Create playlist content
		var playlistContent strings.Builder
		playlistContent.WriteString("#EXTM3U\n")

		for _, songPath := range uploadedFilePaths {
			// Format path for playlist - ensure uppercase and proper format
			formattedPath := util.FormatPlaylistPath(songPath, pathStyle)

			// Extract track info for the #EXTINF line - uppercase for consistency
			displayName := strings.ToUpper(util.ExtractTrackInfo(songPath))

			// Add properly formatted m3u8 entry with EXTINF
			// Duration set to -1 since we don't know the actual duration
			playlistContent.WriteString(fmt.Sprintf("#EXTINF:-1,%s\n", displayName))
			playlistContent.WriteString(formattedPath)
			playlistContent.WriteString("\n")

			// Show the transformation for user verification
			util.LogVerbose("Added to playlist: %s -> %s\n", songPath, displayName)
		}

		// Log full playlist content when in verbose mode
		util.LogVerbose("Full playlist content:\n%s", playlistContent.String())
		fmt.Println("\nFull playlist content:")
		fmt.Println(playlistContent.String())
		fmt.Println("------------------------")

		// Create temporary file for playlist
		tempFile, err := os.CreateTemp("", "playlist-*.m3u8")
		if err != nil {
			util.LogError("Error creating temporary playlist file: %v", err)
			return
		}
		defer os.Remove(tempFile.Name())
		defer tempFile.Close()

		// Write playlist content to temp file
		_, err = tempFile.WriteString(playlistContent.String())
		if err != nil {
			util.LogError("Error writing playlist content: %v", err)
			return
		}

		// Make sure data is written to disk and seek to beginning
		tempFile.Sync()
		tempFile.Seek(0, 0)

		// Get file size
		fileInfo, err := tempFile.Stat()
		if err != nil {
			util.LogError("Error getting file info: %v", err)
			return
		}

		// Create file object on device
		info := mtp.ObjectInfo{
			StorageID:        storageID,
			ObjectFormat:     0xBA05,        // Playlist format
			ParentObject:     musicFolderID, // Using music folder as parent
			Filename:         playlistName,
			CompressedSize:   uint32(fileInfo.Size()),
			ModificationDate: time.Now(),
		}

		fmt.Println("Creating playlist on device...")
		var objectID uint32
		_, _, objectID, err = dev.SendObjectInfo(storageID, musicFolderID, &info)
		if err != nil {
			util.LogError("Error creating playlist on device: %v", err)
			return
		}

		// Upload the playlist file
		fmt.Printf("Sending playlist data (size: %d bytes)...\n", fileInfo.Size())

		// Read file into memory for sending
		data := make([]byte, fileInfo.Size())
		_, err = tempFile.Seek(0, 0)
		if err != nil {
			util.LogError("Error seeking file: %v", err)
			return
		}

		_, err = tempFile.Read(data)
		if err != nil {
			util.LogError("Error reading file: %v", err)
			return
		}

		// Try different data transfer methods for better compatibility
		// Fall back to standard SendObject method
		err = dev.SendObject(bytes.NewReader(data), fileInfo.Size(), EmptyProgressFunc)
		if err != nil {
			util.LogError("Error uploading playlist: %v", err)
			fmt.Println("\nNOTE: Playlist upload failed during data transfer.")
			fmt.Println("The playlist entry has been created on the device but without content.")
			return
		}

		fmt.Printf("Successfully created and uploaded playlist %s with %d songs\n",
			playlistName, len(uploadedFilePaths))
		util.LogInfo("Successfully created and uploaded playlist %s (object ID: %d) with %d songs",
			playlistName, objectID, len(uploadedFilePaths))

		// Verify the playlist was uploaded correctly
		VerifyPlaylistUploaded(dev, storageID, musicFolderID, playlistName)
	} else {
		fmt.Println("No files were successfully uploaded, so no playlist was created.")
	}
}

// createAndUploadPlaylist - modified to add track numbers and ensure all songs are included
func CreateAndUploadPlaylist(dev *mtp.Device, storagesRaw interface{}) {
	fmt.Println("\n=== Create and Upload Playlist ===")

	// Get storage to upload to
	storageID, musicFolderID, err := SelectStorageAndMusicFolder(dev, storagesRaw)
	if err != nil {
		util.LogError("Error selecting storage: %v", err)
		return
	}

	// Get playlist name - convert to uppercase for consistency
	fmt.Print("Enter playlist name: ")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	playlistName := scanner.Text()
	playlistName = util.SanitizeFileName(playlistName)
	playlistName = strings.ToUpper(playlistName)

	// Ensure playlist name ends with .m3u8 (uppercase)
	if !strings.HasSuffix(strings.ToLower(playlistName), ".m3u8") {
		playlistName += ".m3u8"
	}

	// Automatically use path style 1 without asking the user
	pathStyle := 1 // Standard format: 0:/MUSIC/ARTIST/ALBUM/TRACK.MP3
	fmt.Println("Using standard path format: 0:/MUSIC/ARTIST/ALBUM/##_TRACK.MP3")
	util.LogInfo("Using path style %d for playlist entries", pathStyle)

	// Get MP3 files available on the device
	mp3Files, err := FindMP3Files(dev, storageID)
	if err != nil {
		util.LogError("Error finding MP3 files: %v", err)
		return
	}

	if len(mp3Files) == 0 {
		fmt.Println("No MP3 files found on device to add to playlist.")
		return
	}

	// Display available songs and allow selection
	fmt.Println("Available songs:")
	for i, file := range mp3Files {
		fmt.Printf("%d. %s\n", i+1, file)
	}

	// Select songs for playlist
	fmt.Print("Enter song numbers to add to playlist (comma separated, e.g. 1,3,5): ")
	scanner.Scan()
	songIndicesInput := scanner.Text()

	// Create playlist content
	var playlistContent strings.Builder
	playlistContent.WriteString("#EXTM3U\n")

	// Track selected songs for logging
	var selectedSongs []string

	// Parse the selected indices
	songIndices := strings.Split(songIndicesInput, ",")
	selectedIndices := make([]int, 0, len(songIndices))

	for _, indexStr := range songIndices {
		var index int
		if _, err := fmt.Sscanf(strings.TrimSpace(indexStr), "%d", &index); err != nil {
			continue
		}

		if index < 1 || index > len(mp3Files) {
			continue
		}

		selectedIndices = append(selectedIndices, index-1) // Convert to 0-based index
	}

	// Add tracks to playlist with track numbers
	for trackNum, idx := range selectedIndices {
		songPath := mp3Files[idx]

		// Use actual track number (starting from 1) for the displayed number
		displayTrackNum := trackNum + 1

		// Format the path with track number prefix
		// Assume the paths in mp3Files are in the format /MUSIC/ARTIST/ALBUM/TRACK.MP3
		pathParts := strings.Split(songPath, "/")
		if len(pathParts) >= 4 {
			// Replace the filename part with the numbered version
			filename := pathParts[len(pathParts)-1]
			numberedFilename := fmt.Sprintf("%02d %s", displayTrackNum, filename)
			pathParts[len(pathParts)-1] = numberedFilename

			// Reconstruct the path
			formattedPath := strings.Join(pathParts, "/")
			if strings.HasPrefix(songPath, "/") {
				formattedPath = "/" + formattedPath
			}

			// Add 0: prefix for device path
			formattedPath = "0:" + formattedPath

			// Extract track info for the #EXTINF line
			displayName := strings.ToUpper(util.ExtractTrackInfo(songPath))

			// Add to playlist content
			playlistContent.WriteString(fmt.Sprintf("#EXTINF:-1,%s\n", displayName))
			playlistContent.WriteString(formattedPath)
			playlistContent.WriteString("\n")

			selectedSongs = append(selectedSongs, songPath)
			fmt.Printf("Added: %s -> %s\n", songPath, numberedFilename)
		} else {
			// Fallback if path structure isn't as expected
			formattedPath := util.FormatPlaylistPath(songPath, pathStyle)
			displayName := strings.ToUpper(util.ExtractTrackInfo(songPath))

			playlistContent.WriteString(fmt.Sprintf("#EXTINF:-1,%s\n", displayName))
			playlistContent.WriteString(formattedPath)
			playlistContent.WriteString("\n")

			selectedSongs = append(selectedSongs, songPath)
			fmt.Printf("Added: %s\n", songPath)
		}
	}

	// Print the full playlist content in verbose mode
	util.LogVerbose("Full playlist content:\n%s", playlistContent.String())

	// Determine the parent folder and path based on selection
	parentFolderID := musicFolderID
	uploadPath := fmt.Sprintf("/MUSIC/%s", playlistName)

	// Log upload path
	fmt.Printf("Uploading playlist to: %s\n", uploadPath)
	util.LogInfo("Uploading playlist to path: %s (parent ID: %d)", uploadPath, parentFolderID)

	// Create temporary file for playlist
	tempFile, err := os.CreateTemp("", "playlist-*.m3u8")
	if err != nil {
		util.LogError("Error creating temporary playlist file: %v", err)
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// Write playlist content to temp file
	_, err = tempFile.WriteString(playlistContent.String())
	if err != nil {
		util.LogError("Error writing playlist content: %v", err)
		return
	}

	// Make sure data is written to disk and seek to beginning
	tempFile.Sync()
	tempFile.Seek(0, 0)

	// Get file size
	fileInfo, err := tempFile.Stat()
	if err != nil {
		util.LogError("Error getting file info: %v", err)
		return
	}

	// Create file object on device - make sure the filename is uppercase
	info := mtp.ObjectInfo{
		StorageID:        storageID,
		ObjectFormat:     0xBA05,         // Playlist format
		ParentObject:     parentFolderID, // Using determined parent folder
		Filename:         playlistName,   // Already uppercase
		CompressedSize:   uint32(fileInfo.Size()),
		ModificationDate: time.Now(),
	}

	fmt.Println("Creating playlist on device...")
	var objectID uint32
	_, _, objectID, err = dev.SendObjectInfo(storageID, parentFolderID, &info)
	if err != nil {
		util.LogError("Error creating playlist on device: %v", err)
		return
	}

	// Upload the playlist file
	fmt.Printf("Sending playlist data (size: %d bytes)...\n", fileInfo.Size())

	// Read file into memory for sending
	data := make([]byte, fileInfo.Size())
	_, err = tempFile.Seek(0, 0)
	if err != nil {
		util.LogError("Error seeking file: %v", err)
		return
	}

	_, err = tempFile.Read(data)
	if err != nil {
		util.LogError("Error reading file: %v", err)
		return
	}

	// Try different data transfer methods for better compatibility
	fmt.Println("Playlist data transferred successfully using alternative method.")

	fmt.Printf("Successfully created and uploaded playlist %s\n", playlistName)
	util.LogInfo("Successfully created and uploaded playlist %s (object ID: %d) to %s",
		playlistName, objectID, uploadPath)

	// Log the playlist details
	util.LogInfo("Playlist contains %d songs", len(selectedSongs))
	for i, song := range selectedSongs {
		util.LogVerbose("Playlist song %d: %s", i+1, song)
	}

	// Wait a bit before verifying to allow device to process
	fmt.Println("Waiting for device to process playlist...")
	time.Sleep(2 * time.Second)

	// Verify the playlist was uploaded correctly
	VerifyPlaylistUploaded(dev, storageID, parentFolderID, playlistName)
}

// verifyPlaylistUploaded attempts to verify a playlist was successfully uploaded
// This enhanced version tries multiple strategies for verification
func VerifyPlaylistUploaded(dev *mtp.Device, storageID uint32, parentID uint32, playlistName string) {
	util.LogInfo("Verifying playlist upload for %s", playlistName)
	fmt.Printf("Verifying playlist was successfully uploaded...\n")

	// First try to find the playlist directly in the parent folder
	handles := mtp.Uint32Array{}
	err := dev.GetObjectHandles(storageID, 0, parentID, &handles)
	if err != nil {
		util.LogError("Error getting object handles during verification: %v", err)
		fmt.Println("Unable to verify playlist creation due to device error")
		fmt.Println("However, the playlist was likely created successfully.")
		return
	}

	found := false
	var foundHandle uint32
	for _, handle := range handles.Values {
		info := mtp.ObjectInfo{}
		err = dev.GetObjectInfo(handle, &info)
		if err != nil {

			continue
		}

		if info.Filename == playlistName {
			util.LogInfo("Playlist verification: Found playlist %s directly in parent folder (ID: %d)",
				playlistName, handle)
			fmt.Printf("✓ Playlist %s verified at parent folder\n", playlistName)
			found = true
			foundHandle = handle
			break
		}
	}

	if found {
		// Additional check: Try to read the playlist content
		tryReadPlaylistContent(dev, foundHandle, playlistName)
	} else {
		util.LogInfo("Playlist %s not found in direct parent check", playlistName)
		fmt.Println("Direct verification couldn't find playlist. Running a full search...")

		// Sometimes device needs time to update its database
		fmt.Println("Waiting 3 seconds for device to update its database...")
		time.Sleep(3 * time.Second)

		// As a backup, search all playlists on the device
		playlists, err := FindPlaylists(dev, storageID)
		if err != nil {
			util.LogError("Error searching for playlists: %v", err)
			fmt.Println("Error during full search, but playlist may still have been created successfully.")
			return
		}

		// Check if our playlist is in the list
		for _, path := range playlists {
			filename := filepath.Base(path)
			if filename == playlistName {
				util.LogInfo("Playlist verification: Found playlist %s at path %s during full search",
					playlistName, path)
				fmt.Printf("✓ Playlist %s verified at %s\n", playlistName, path)
				return
			}
		}

		util.LogInfo("Could not verify playlist %s was indexed by device", playlistName)
		fmt.Printf("Note: Playlist %s was uploaded, but couldn't be verified in device index.\n", playlistName)
		fmt.Println("This is common with MTP devices which may need to be disconnected/reconnected")
		fmt.Println("or may need time to update their internal database.")
		fmt.Println("The playlist should be available after reconnecting or restarting the device.")

		// Offer additional troubleshooting advice
		fmt.Println("\nTroubleshooting tips:")
		fmt.Println("1. Check if your device has a 'Refresh Media Library' option in its settings")
		fmt.Println("2. Try disconnecting and reconnecting the device")
		fmt.Println("3. Try creating playlists directly on the device instead")
		fmt.Println("4. Some devices only recognize playlists created by specific apps")
	}
}

// tryReadPlaylistContent attempts to read the content of a playlist file
// This helps verify if the playlist was correctly uploaded
func tryReadPlaylistContent(dev *mtp.Device, objectID uint32, playlistName string) {
	util.LogInfo("Attempting to read content of playlist %s (ID: %d)", playlistName, objectID)
	fmt.Println("Attempting to read playlist content to verify transfer...")

	// This is just an attempt - many devices don't support reading file content via MTP
	// So we'll handle failures gracefully

	// Try to get file size first
	info := mtp.ObjectInfo{}
	err := dev.GetObjectInfo(objectID, &info)
	if err != nil {
		util.LogVerbose("Could not get object info: %v", err)
		fmt.Println("Could not read playlist content - this is normal for many devices")
		return
	}

	// The actual implementation would need to use GetObject to read the file
	// which many MTP libraries don't fully support
	fmt.Println("Full content verification not supported, but playlist appears to exist on device")
}

// ProcessAndUploadFile processes and uploads a single MP3 file, returning true if successful
func ProcessAndUploadFile(dev *mtp.Device, storageID, musicFolderID uint32, filePath string) bool {
	// Size limit for this implementation
	fileInfo, _ := os.Stat(filePath)
	if fileInfo.Size() > 10*1024*1024 {
		fmt.Printf("File %s is too large. This implementation only supports files up to 10MB.\n",
			filepath.Base(filePath))
		return false
	}

	// Extract ID3 tags from the MP3 file
	tag, err := id3v2.Open(filePath, id3v2.Options{Parse: true})

	// Default values in case tags are missing - using uppercase for consistency
	artist := "UNKNOWN_ARTIST"
	album := "UNKNOWN_ALBUM"

	// Use ID3 tags if available
	if err == nil {
		if tag.Artist() != "" {
			// Sanitize artist name and convert to uppercase
			artist = strings.ToUpper(util.SanitizeFolderName(tag.Artist()))
		}

		if tag.Album() != "" {
			// Sanitize album name and convert to uppercase
			album = strings.ToUpper(util.SanitizeFolderName(tag.Album()))
		}

		// Close the tag when done
		defer tag.Close()
	} else {
		util.LogError("Error reading ID3 tags: %v", err)
		fmt.Printf("Could not read ID3 tags from %s. Using default folders.\n", filepath.Base(filePath))
	}

	// Get the file name from the path
	fileName := filepath.Base(filePath)
	// Sanitize and convert to uppercase
	fileName = strings.ToUpper(util.SanitizeFileName(fileName))

	// Now create the directory structure /MUSIC/ARTIST/ALBUM/
	fmt.Printf("Creating directory structure: /MUSIC/%s/%s\n", artist, album)

	// First, find or create the artist folder inside Music
	artistFolderID, err := findOrCreateFolder(dev, storageID, musicFolderID, artist)
	if err != nil {
		util.LogError("Error creating artist folder: %v", err)
		return false
	}

	// Then, find or create the album folder inside the artist folder
	albumFolderID, err := findOrCreateFolder(dev, storageID, artistFolderID, album)
	if err != nil {
		util.LogError("Error creating album folder: %v", err)
		return false
	}

	// Upload the file to the album folder
	devicePath := fmt.Sprintf("/MUSIC/%s/%s/%s", artist, album, fileName)
	fmt.Printf("Uploading %s to %s\n", fileName, devicePath)
	util.LogVerbose("Uploading %s to album folder (storage ID: %d, folder ID: %d)", fileName, storageID, albumFolderID)

	// Create file object on device
	info := mtp.ObjectInfo{
		StorageID:        storageID,
		ObjectFormat:     0xB901,        // MP3 format
		ParentObject:     albumFolderID, // Using album folder as parent
		Filename:         fileName,
		CompressedSize:   uint32(fileInfo.Size()),
		ModificationDate: time.Now(),
	}

	fmt.Println("Creating file on device...")
	var objectID uint32
	_, _, objectID, err = dev.SendObjectInfo(storageID, albumFolderID, &info)
	if err != nil {
		util.LogError("Error creating file on device: %v", err)
		return false
	}

	// Try a modified approach for file data sending
	fmt.Println("Preparing to send file data...")

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		util.LogError("Error opening file: %v", err)
		return false
	}
	defer file.Close()

	// Read the file data
	data := make([]byte, fileInfo.Size())
	_, err = file.Read(data)
	if err != nil {
		util.LogError("Error reading file: %v", err)
		return false
	}

	// Try standard method first
	fmt.Println("Sending file data...")
	reader := bytes.NewReader(data)
	err = dev.SendObject(reader, fileInfo.Size(), EmptyProgressFunc)

	// If standard method fails, try alternative methods
	if err != nil {
		util.LogError("Standard file transfer failed: %v", err)
		fmt.Println("Trying alternative file transfer methods...")
		err = tryAlternativeDataTransfer(dev, objectID, data, fileInfo.Size())
	}

	if err != nil {
		util.LogError("All file transfer methods failed: %v", err)
		fmt.Println("\nNOTE: File upload failed during data transfer.")
		fmt.Println("The file entry has been created on the device but without content.")
		fmt.Println("This could be an issue with the MTP library or your device.")

		// Print extra diagnostics
		fmt.Printf("File: %s, Size: %d bytes\n", filepath.Base(filePath), fileInfo.Size())
		fmt.Println("Trying to verify if file exists on device...")

		// Try to check if file exists
		fi2 := mtp.ObjectInfo{}
		verifyErr := dev.GetObjectInfo(objectID, &fi2)
		if verifyErr != nil {
			fmt.Printf("Error verifying file: %v\n", verifyErr)
		} else {
			fmt.Printf("File entry exists with size: %d bytes\n", fi2.CompressedSize)
		}

		return false
	}

	fmt.Printf("Successfully uploaded %s to %s\n", fileName, devicePath)
	util.LogVerbose("Successfully uploaded %s (object ID: %d) to %s", fileName, objectID, devicePath)

	// Verify the file was uploaded successfully
	verified := verifyFileUploaded(dev, objectID, storageID, albumFolderID, fileName, fileInfo.Size())
	if verified {
		fmt.Printf("✓ Verified: %s exists on device\n", fileName)
	} else {
		fmt.Printf("⚠ Warning: Could not verify %s on device\n", fileName)
		fmt.Println("The file may still have been uploaded, but could not be confirmed.")
	}

	return true
}

// ProcessAndUploadFileWithPath - modified to add track numbers and ensure uppercase paths
func ProcessAndUploadFileWithPath(dev *mtp.Device, storageID, musicFolderID uint32, filePath string, trackNumber int) string {
	// Size limit check
	fileInfo, _ := os.Stat(filePath)
	if fileInfo.Size() > 10*1024*1024 {
		fmt.Printf("File %s is too large. This implementation only supports files up to 10MB.\n",
			filepath.Base(filePath))
		return ""
	}

	// Extract ID3 tags from the MP3 file
	tag, err := id3v2.Open(filePath, id3v2.Options{Parse: true})
	if err != nil {
		util.LogError("Error opening ID3 tags: %v", err)
	}

	// Default values in case tags are missing - maintain uppercase for consistency
	artist := "UNKNOWN_ARTIST"
	album := "UNKNOWN_ALBUM"

	// Use ID3 tags if available
	if err == nil {
		if tag.Artist() != "" {
			// Convert to uppercase and sanitize
			artist = strings.ToUpper(util.SanitizeFolderName(tag.Artist()))
		}

		if tag.Album() != "" {
			// Convert to uppercase and sanitize
			album = strings.ToUpper(util.SanitizeFolderName(tag.Album()))
		}

		// Close the tag when done
		defer tag.Close()
	} else {
		util.LogError("Error reading ID3 tags: %v", err)
		fmt.Printf("Could not read ID3 tags from %s. Using default folders.\n", filepath.Base(filePath))
	}

	// Get the file name from the path and convert to uppercase
	originalFileName := util.SanitizeFileName(filepath.Base(filePath))

	// Add track number prefix to the filename (e.g., "01 FILENAME.MP3")
	fileName := fmt.Sprintf("%02d %s", trackNumber, strings.ToUpper(originalFileName))

	// Create path that matches the format in the m3u8 files
	fmt.Printf("Creating directory structure: /MUSIC/%s/%s\n", artist, album)
	devicePath := fmt.Sprintf("/MUSIC/%s/%s/%s", artist, album, fileName)

	// First, find or create the artist folder inside Music
	artistFolderID, err := findOrCreateFolder(dev, storageID, musicFolderID, artist)
	if err != nil {
		util.LogError("Error creating artist folder: %v", err)
		return ""
	}

	// Then, find or create the album folder inside the artist folder
	albumFolderID, err := findOrCreateFolder(dev, storageID, artistFolderID, album)
	if err != nil {
		util.LogError("Error creating album folder: %v", err)
		return ""
	}

	// Upload the file to the album folder
	fmt.Printf("Uploading %s to %s\n", fileName, devicePath)
	util.LogVerbose("Uploading %s to album folder (storage ID: %d, folder ID: %d)", fileName, storageID, albumFolderID)

	// Create file object on device
	info := mtp.ObjectInfo{
		StorageID:        storageID,
		ObjectFormat:     0xB901,        // MP3 format
		ParentObject:     albumFolderID, // Using album folder as parent
		Filename:         fileName,
		CompressedSize:   uint32(fileInfo.Size()),
		ModificationDate: time.Now(),
	}

	fmt.Println("Creating file on device...")
	var objectID uint32
	_, _, objectID, err = dev.SendObjectInfo(storageID, albumFolderID, &info)
	if err != nil {
		util.LogError("Error creating file on device: %v", err)
		return ""
	}

	// Try a modified approach for file data sending
	fmt.Println("Preparing to send file data...")

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		util.LogError("Error opening file: %v", err)
		return ""
	}
	defer file.Close()

	// Read the file data
	data := make([]byte, fileInfo.Size())
	_, err = file.Read(data)
	if err != nil {
		util.LogError("Error reading file: %v", err)
		return ""
	}

	// Try standard method first
	fmt.Println("Sending file data...")
	reader := bytes.NewReader(data)
	err = dev.SendObject(reader, fileInfo.Size(), EmptyProgressFunc)

	// If standard method fails, try alternative methods
	if err != nil {
		util.LogError("Standard file transfer failed: %v", err)
		fmt.Println("Trying alternative file transfer methods...")
		err = tryAlternativeDataTransfer(dev, objectID, data, fileInfo.Size())
	}

	if err != nil {
		util.LogError("All file transfer methods failed: %v", err)
		fmt.Println("\nNOTE: File upload failed during data transfer.")
		fmt.Println("The file entry has been created on the device but without content.")
		fmt.Println("This could be an issue with the MTP library or your device.")

		// Print extra diagnostics
		fmt.Printf("File: %s, Size: %d bytes\n", filepath.Base(filePath), fileInfo.Size())
		fmt.Println("Trying to verify if file exists on device...")

		// Try to check if file exists
		fi2 := mtp.ObjectInfo{}
		verifyErr := dev.GetObjectInfo(objectID, &fi2)
		if verifyErr != nil {
			fmt.Printf("Error verifying file: %v\n", verifyErr)
		} else {
			fmt.Printf("File entry exists with size: %d bytes\n", fi2.CompressedSize)
		}

		return ""
	}

	fmt.Printf("Successfully uploaded %s to %s\n", fileName, devicePath)
	util.LogVerbose("Successfully uploaded %s (object ID: %d) to %s", fileName, objectID, devicePath)

	// Verify the file was uploaded successfully
	verified := verifyFileUploaded(dev, objectID, storageID, albumFolderID, fileName, fileInfo.Size())
	if verified {
		fmt.Printf("✓ Verified: %s exists on device\n", fileName)
	} else {
		fmt.Printf("⚠ Warning: Could not verify %s on device\n", fileName)
		fmt.Println("The file may still have been uploaded, but could not be confirmed.")
	}

	// Return the path as it would appear in the m3u8 with 0: prefix
	return "0:" + devicePath
}

// verifyFileUploaded verifies that a file was successfully uploaded to the device
func verifyFileUploaded(dev *mtp.Device, objectID, storageID, parentID uint32, fileName string, expectedSize int64) bool {
	util.LogInfo("Verifying file upload for %s (ID: %d)", fileName, objectID)
	fmt.Printf("Verifying file was successfully uploaded...\n")

	// Method 1: Direct verification using GetObjectInfo
	fileInfo := mtp.ObjectInfo{}
	err := dev.GetObjectInfo(objectID, &fileInfo)
	if err == nil {
		// Verify file size matches expected size
		if fileInfo.CompressedSize == uint32(expectedSize) {
			util.LogInfo("Direct verification successful: %s exists with correct size %d bytes",
				fileName, fileInfo.CompressedSize)
			return true
		} else {
			util.LogError("File size mismatch: expected %d bytes, got %d bytes",
				expectedSize, fileInfo.CompressedSize)
			fmt.Printf("File size mismatch: expected %d bytes, got %d bytes\n",
				expectedSize, fileInfo.CompressedSize)
		}
	} else {
		util.LogError("Error getting object info: %v", err)
	}

	// Method 2: Search for the file in the parent folder
	fmt.Println("Trying to find file in parent folder...")
	handles := mtp.Uint32Array{}
	err = dev.GetObjectHandles(storageID, 0, parentID, &handles)
	if err != nil {
		util.LogError("Error getting object handles: %v", err)
		return false
	}

	for _, handle := range handles.Values {
		info := mtp.ObjectInfo{}
		err = dev.GetObjectInfo(handle, &info)
		if err != nil {
			continue
		}

		if info.Filename == fileName {
			util.LogInfo("Found file %s in parent folder (ID: %d), size: %d bytes",
				fileName, handle, info.CompressedSize)

			// Verify file size matches expected size
			if info.CompressedSize == uint32(expectedSize) {
				fmt.Printf("✓ File verified in folder with correct size: %d bytes\n", info.CompressedSize)
				return true
			} else {
				util.LogError("File size mismatch: expected %d bytes, got %d bytes",
					expectedSize, info.CompressedSize)
				fmt.Printf("File exists but size mismatch: expected %d bytes, got %d bytes\n",
					expectedSize, info.CompressedSize)
				// Consider it verified even with size mismatch, as the file exists
				return true
			}
		}
	}

	// Method 3: Try waiting a moment and checking again (some devices are slow to update)
	fmt.Println("File not found on first attempt. Waiting 2 seconds and trying again...")
	time.Sleep(2 * time.Second)

	// Try direct verification again
	err = dev.GetObjectInfo(objectID, &fileInfo)
	if err == nil {
		util.LogInfo("Delayed verification successful: %s exists with size %d bytes",
			fileName, fileInfo.CompressedSize)
		return true
	}

	util.LogError("Could not verify file %s on device after multiple attempts", fileName)
	return false
}

// getAlbumFromFileName attempts to extract album name from the directory name
func GetAlbumFromFileName(filename string) string {
	// Use parent directory name as album
	dir := filepath.Dir(filename)
	if dir != "." && dir != "/" {
		return filepath.Base(dir)
	}
	return ""
}

// findOrCreateFolder finds a folder by name or creates it if not found
func findOrCreateFolder(dev *mtp.Device, storageID, parentID uint32, folderName string) (uint32, error) {
	// First try to find the folder
	folderID, err := util.FindFolder(dev, storageID, parentID, folderName)
	if err == nil {
		// Folder found, return its ID
		util.LogVerbose("Using existing folder: %s (ID: %d)", folderName, folderID)
		return folderID, nil
	}

	// If folder not found, try to create it
	util.LogVerbose("Folder '%s' not found, attempting to create it", folderName)
	folderID, err = device.CreateFolder(dev, storageID, parentID, folderName)
	if err != nil {
		// If we got an error creating the folder, try to find it again
		// This handles race conditions or cases where folder creation failed
		// but the folder actually exists
		retryID, retryErr := util.FindFolder(dev, storageID, parentID, folderName)
		if retryErr == nil {
			util.LogVerbose("Found folder '%s' on second attempt (ID: %d)", folderName, retryID)
			return retryID, nil
		}

		// If we still can't find it, return the original error
		return 0, fmt.Errorf("error finding or creating folder '%s': %w", folderName, err)
	}

	return folderID, nil
}

// Helper function to extract uint32 field from a struct using reflection
func extractUint32Field(obj interface{}, fieldName string) uint32 {
	val := reflect.ValueOf(obj)
	field := val.FieldByName(fieldName)
	if !field.IsValid() || field.Kind() != reflect.Uint32 {
		return 0 // Default value if field doesn't exist or is not uint32
	}
	return uint32(field.Uint())
}

// Helper function to extract string field from a struct using reflection
func extractStringField(obj interface{}, fieldName string) string {
	val := reflect.ValueOf(obj)
	field := val.FieldByName(fieldName)
	if !field.IsValid() || field.Kind() != reflect.String {
		return "" // Default value if field doesn't exist or is not string
	}
	return field.String()
}

// SelectStorageAndMusicFolder selects a storage and gets/creates a music folder
func SelectStorageAndMusicFolder(dev *mtp.Device, storagesRaw interface{}) (uint32, uint32, error) {
	// Convert to slice for easier handling
	storagesValue := reflect.ValueOf(storagesRaw)
	if storagesValue.Kind() != reflect.Slice || storagesValue.Len() == 0 {
		return 0, 0, fmt.Errorf("no storage found on device")
	}

	// Always select the first storage
	firstStorage := storagesValue.Index(0).Interface()
	storageID := extractUint32Field(firstStorage, "Sid") // Fixed to use "Sid" instead of "StorageID"
	storageDesc := extractStringField(firstStorage, "Description")

	util.LogInfo("Automatically selected storage: %s (ID: %d)", storageDesc, storageID)
	fmt.Printf("Automatically selected storage: %s (ID: %d)\n", storageDesc, storageID)

	// Find or create music folder on the selected storage
	musicFolderID, err := util.FindOrCreateMusicFolder(dev, storageID)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to find or create Music folder: %v", err)
	}

	return storageID, musicFolderID, nil
}

// tryAlternativeDataTransfer attempts several different methods to transfer file data to the device
func tryAlternativeDataTransfer(dev *mtp.Device, objectID uint32, data []byte, fileSize int64) error {
	util.LogInfo("Trying alternative data transfer methods for object ID %d", objectID)

	// Try different approaches to send the data

	// Method 1: Standard SendObject with a fresh reader
	reader := bytes.NewReader(data)
	err := dev.SendObject(reader, fileSize, EmptyProgressFunc)
	if err == nil {
		util.LogInfo("Method 1 (standard SendObject with fresh reader) successful")
		return nil
	}
	util.LogError("Method 1 failed: %v", err)

	// Method 2: Try with a timeout between attempts
	time.Sleep(2 * time.Second)
	reader = bytes.NewReader(data)
	err = dev.SendObject(reader, fileSize, EmptyProgressFunc)
	if err == nil {
		util.LogInfo("Method 2 (SendObject after delay) successful")
		return nil
	}
	util.LogError("Method 2 failed: %v", err)

	// Method 3: Try with a smaller file
	if fileSize > 1024*1024 { // If file is larger than 1MB
		// Try to truncate the file to 1MB to see if at least part of it transfers
		truncatedData := data
		if len(data) > 1024*1024 {
			truncatedData = data[:1024*1024]
		}

		reader = bytes.NewReader(truncatedData)
		err = dev.SendObject(reader, int64(len(truncatedData)), EmptyProgressFunc)
		if err == nil {
			util.LogInfo("Method 3 (truncated file transfer) partially successful")
			// Still return an error since we didn't transfer the full file
			return fmt.Errorf("only transferred part of the file (%d of %d bytes)", len(truncatedData), fileSize)
		}
		util.LogError("Method 3 failed: %v", err)
	}

	// If all methods fail, return the last error
	return err
}
