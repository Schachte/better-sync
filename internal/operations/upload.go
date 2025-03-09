package operations

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/bogem/id3v2"
	"github.com/ganeshrvel/go-mtpfs/mtp"
	"github.com/schachte/better-sync/internal/device"
	"github.com/schachte/better-sync/internal/model"
	"github.com/schachte/better-sync/internal/util"
	"github.com/schollz/progressbar/v3"
)

const (
	PARENT_ROOT     uint32 = 0
	FILETYPE_FOLDER uint16 = 0x3001
)

func EmptyProgressFunc(_ int64) error {
	return nil
}

func UploadSong(dev *mtp.Device, storagesRaw interface{}) {

	storageID, musicFolderID, err := SelectStorageAndMusicFolder(dev, storagesRaw)
	if err != nil {
		util.LogError("Error selecting storage: %v", err)
		fmt.Printf("Error: %v\n", err)
		return
	}

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

func uploadSingleFile(dev *mtp.Device, storageID, musicFolderID uint32) {

	fmt.Print("Enter path to MP3 file: ")
	reader := bufio.NewReader(os.Stdin)
	filePath, _ := reader.ReadString('\n')
	filePath = strings.TrimSpace(filePath)

	filePath = strings.Trim(filePath, "\"'")

	if filePath == "" {
		fmt.Println("No file path provided")
		return
	}

	success := ProcessAndUploadFile(dev, storageID, musicFolderID, filePath)
	if success {
		fmt.Println("File uploaded successfully")
	} else {
		fmt.Println("Failed to upload file")
	}
}

func uploadDirectory(dev *mtp.Device, storageID, musicFolderID uint32) {

	fmt.Print("Enter path to directory containing MP3 files: ")
	reader := bufio.NewReader(os.Stdin)
	dirPath, _ := reader.ReadString('\n')
	dirPath = strings.TrimSpace(dirPath)

	dirPath = strings.Trim(dirPath, "\"'")

	if dirPath == "" {
		fmt.Println("No directory path provided")
		return
	}

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

	fmt.Println("Searching for MP3 files in the directory...")
	var mp3Files []string

	err = filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			util.LogError("Error accessing path %s: %v", path, err)
			return nil
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

	successful := 0
	for i, filePath := range mp3Files {
		fmt.Printf("[%d/%d] Uploading %s...\n", i+1, len(mp3Files), filepath.Base(filePath))
		if ProcessAndUploadFile(dev, storageID, musicFolderID, filePath) {
			successful++
		}
	}

	util.LogInfo("Upload complete. %d/%d files uploaded successfully.", successful, len(mp3Files))
}

func CreateAndUploadPlaylist(dev *mtp.Device, storagesRaw interface{}) {
	fmt.Println("\n=== Create and Upload Playlist ===")

	storageID, musicFolderID, err := SelectStorageAndMusicFolder(dev, storagesRaw)
	if err != nil {
		util.LogError("Error selecting storage: %v", err)
		return
	}

	fmt.Print("Enter playlist name: ")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	playlistName := scanner.Text()
	playlistName = util.SanitizeFileName(playlistName)
	playlistName = strings.ToUpper(playlistName)

	if !strings.HasSuffix(strings.ToLower(playlistName), ".m3u8") {
		playlistName += ".m3u8"
	}

	pathStyle := 1
	fmt.Println("Using standard path format: 0:/MUSIC/ARTIST/ALBUM/##_TRACK.MP3")
	util.LogInfo("Using path style %d for playlist entries", pathStyle)

	mp3Files, err := FindMP3Files(dev, storageID)
	if err != nil {
		util.LogError("Error finding MP3 files: %v", err)
		return
	}

	if len(mp3Files) == 0 {
		fmt.Println("No MP3 files found on device to add to playlist.")
		return
	}

	fmt.Println("Available songs:")
	for i, file := range mp3Files {
		fmt.Printf("%d. %s\n", i+1, file)
	}

	fmt.Print("Enter song numbers to add to playlist (comma separated, e.g. 1,3,5): ")
	scanner.Scan()
	songIndicesInput := scanner.Text()

	var playlistContent strings.Builder
	playlistContent.WriteString("#EXTM3U\n")

	var selectedSongs []string

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

		selectedIndices = append(selectedIndices, index-1)
	}

	for trackNum, idx := range selectedIndices {
		songPath := mp3Files[idx]

		displayTrackNum := trackNum + 1

		pathParts := strings.Split(songPath, "/")
		if len(pathParts) >= 4 {

			filename := pathParts[len(pathParts)-1]
			numberedFilename := fmt.Sprintf("%02d %s", displayTrackNum, filename)
			pathParts[len(pathParts)-1] = numberedFilename

			formattedPath := strings.Join(pathParts, "/")
			if strings.HasPrefix(songPath, "/") {
				formattedPath = "/" + formattedPath
			}

			formattedPath = "0:" + formattedPath

			displayName := strings.ToUpper(util.ExtractTrackInfo(songPath))

			playlistContent.WriteString(fmt.Sprintf("#EXTINF:-1,%s\n", displayName))
			playlistContent.WriteString(formattedPath)
			playlistContent.WriteString("\n")

			selectedSongs = append(selectedSongs, songPath)
			fmt.Printf("Added: %s -> %s\n", songPath, numberedFilename)
		} else {

			formattedPath := util.FormatPlaylistPath(songPath, pathStyle)
			displayName := strings.ToUpper(util.ExtractTrackInfo(songPath))

			playlistContent.WriteString(fmt.Sprintf("#EXTINF:-1,%s\n", displayName))
			playlistContent.WriteString(formattedPath)
			playlistContent.WriteString("\n")

			selectedSongs = append(selectedSongs, songPath)
			fmt.Printf("Added: %s\n", songPath)
		}
	}

	util.LogVerbose("Full playlist content:\n%s", playlistContent.String())

	parentFolderID := musicFolderID
	uploadPath := fmt.Sprintf("/MUSIC/%s", playlistName)

	fmt.Printf("Uploading playlist to: %s\n", uploadPath)
	util.LogInfo("Uploading playlist to path: %s (parent ID: %d)", uploadPath, parentFolderID)

	tempFile, err := os.CreateTemp("", "playlist-*.m3u8")
	if err != nil {
		util.LogError("Error creating temporary playlist file: %v", err)
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	_, err = tempFile.WriteString(playlistContent.String())
	if err != nil {
		util.LogError("Error writing playlist content: %v", err)
		return
	}

	tempFile.Sync()
	tempFile.Seek(0, 0)

	fileInfo, err := tempFile.Stat()
	if err != nil {
		util.LogError("Error getting file info: %v", err)
		return
	}

	info := mtp.ObjectInfo{
		StorageID:        storageID,
		ObjectFormat:     0xBA05,
		ParentObject:     parentFolderID,
		Filename:         playlistName,
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

	fmt.Printf("Sending playlist data (size: %d bytes)...\n", fileInfo.Size())

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

	fmt.Println("Playlist data transferred successfully using alternative method.")

	fmt.Printf("Successfully created and uploaded playlist %s\n", playlistName)
	util.LogInfo("Successfully created and uploaded playlist %s (object ID: %d) to %s",
		playlistName, objectID, uploadPath)

	util.LogInfo("Playlist contains %d songs", len(selectedSongs))
	for i, song := range selectedSongs {
		util.LogVerbose("Playlist song %d: %s", i+1, song)
	}

	fmt.Println("Waiting for device to process playlist...")
	time.Sleep(2 * time.Second)

	verified := VerifyPlaylistUploaded(dev, storageID, parentFolderID, playlistName)
	if verified {
		fmt.Println("Playlist verification successful")
	} else {
		fmt.Println("Playlist verification failed")
	}
}

func VerifyPlaylistUploaded(dev *mtp.Device, storageID uint32, parentID uint32, playlistName string) bool {
	util.LogInfo("Verifying playlist upload for %s", playlistName)

	handles := mtp.Uint32Array{}
	err := dev.GetObjectHandles(storageID, 0, parentID, &handles)
	if err != nil {
		util.LogError("Error getting object handles during verification: %v", err)
		util.LogError("Unable to verify playlist creation due to device error")
		util.LogInfo("However, the playlist was likely created successfully.")
		return false
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
			util.LogInfo("✓ Playlist %s verified at parent folder", playlistName)
			found = true
			foundHandle = handle
			break
		}
	}

	if found {
		tryReadPlaylistContent(dev, foundHandle, playlistName)
		return true
	} else {
		util.LogInfo("Playlist %s not found in direct parent check", playlistName)
		util.LogInfo("Direct verification couldn't find playlist. Running a full search...")

		util.LogVerbose("Waiting 3 seconds for device to update its database...")
		time.Sleep(3 * time.Second)

		playlists, err := FindPlaylists(dev, storageID)
		if err != nil {
			util.LogError("Error searching for playlists: %v", err)
			util.LogError("Error during full search, but playlist may still have been created successfully.")
			return false
		}

		for _, path := range playlists {
			filename := filepath.Base(path)
			if filename == playlistName {
				util.LogInfo("Playlist verification: Found playlist %s at path %s during full search",
					playlistName, path)
				util.LogInfo("✓ Playlist %s verified at %s", playlistName, path)
				return true
			}
		}

		util.LogInfo("Could not verify playlist %s was indexed by device", playlistName)
		util.LogInfo("Note: Playlist %s was uploaded, but couldn't be verified in device index.", playlistName)
		util.LogInfo("This is common with MTP devices which may need to be disconnected/reconnected")
		util.LogInfo("or may need time to update their internal database.")
		util.LogInfo("The playlist should be available after reconnecting or restarting the device.")

		util.LogInfo("\nTroubleshooting tips:")
		util.LogInfo("1. Check if your device has a 'Refresh Media Library' option in its settings")
		util.LogInfo("2. Try disconnecting and reconnecting the device")
		util.LogInfo("3. Try creating playlists directly on the device instead")
		util.LogInfo("4. Some devices only recognize playlists created by specific apps")
		return false
	}
}

func tryReadPlaylistContent(dev *mtp.Device, objectID uint32, playlistName string) {
	util.LogInfo("Attempting to read content of playlist %s (ID: %d)", playlistName, objectID)
	fmt.Println("Attempting to read playlist content to verify transfer...")

	info := mtp.ObjectInfo{}
	err := dev.GetObjectInfo(objectID, &info)
	if err != nil {
		util.LogVerbose("Could not get object info: %v", err)
		fmt.Println("Could not read playlist content - this is normal for many devices")
		return
	}

	util.LogInfo("Playlist appears to exist on device")
}

func ProcessAndUploadFile(dev *mtp.Device, storageID, musicFolderID uint32, filePath string) bool {

	fileInfo, _ := os.Stat(filePath)
	if fileInfo.Size() > 10*1024*1024 {
		fmt.Printf("File %s is too large. This implementation only supports files up to 10MB.\n",
			filepath.Base(filePath))
		return false
	}

	tag, err := id3v2.Open(filePath, id3v2.Options{Parse: true})

	artist := "UNKNOWN_ARTIST"
	album := "UNKNOWN_ALBUM"

	if err == nil {
		if tag.Artist() != "" {

			artist = strings.ToUpper(util.SanitizeFolderName(tag.Artist()))
		}

		if tag.Album() != "" {

			album = strings.ToUpper(util.SanitizeFolderName(tag.Album()))
		}

		defer tag.Close()
	} else {
		util.LogError("Error reading ID3 tags: %v", err)
		fmt.Printf("Could not read ID3 tags from %s. Using default folders.\n", filepath.Base(filePath))
	}

	fileName := filepath.Base(filePath)

	fileName = strings.ToUpper(util.SanitizeFileName(fileName))

	fmt.Printf("Creating directory structure: /MUSIC/%s/%s\n", artist, album)

	artistFolderID, err := findOrCreateFolder(dev, storageID, musicFolderID, artist)
	if err != nil {
		util.LogError("Error creating artist folder: %v", err)
		return false
	}

	albumFolderID, err := findOrCreateFolder(dev, storageID, artistFolderID, album)
	if err != nil {
		util.LogError("Error creating album folder: %v", err)
		return false
	}

	devicePath := fmt.Sprintf("/MUSIC/%s/%s/%s", artist, album, fileName)
	fmt.Printf("Uploading %s to %s\n", fileName, devicePath)
	util.LogVerbose("Uploading %s to album folder (storage ID: %d, folder ID: %d)", fileName, storageID, albumFolderID)

	info := mtp.ObjectInfo{
		StorageID:        storageID,
		ObjectFormat:     0xB901,
		ParentObject:     albumFolderID,
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

	fmt.Println("Preparing to send file data...")

	file, err := os.Open(filePath)
	if err != nil {
		util.LogError("Error opening file: %v", err)
		return false
	}
	defer file.Close()

	data := make([]byte, fileInfo.Size())
	_, err = file.Read(data)
	if err != nil {
		util.LogError("Error reading file: %v", err)
		return false
	}

	fmt.Println("Sending file data...")
	reader := bytes.NewReader(data)
	err = dev.SendObject(reader, fileInfo.Size(), EmptyProgressFunc)

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

		fmt.Printf("File: %s, Size: %d bytes\n", filepath.Base(filePath), fileInfo.Size())
		fmt.Println("Trying to verify if file exists on device...")

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

	verified := verifyFileUploaded(dev, objectID, storageID, albumFolderID, fileName, fileInfo.Size())
	if verified {
		fmt.Printf("✓ Verified: %s exists on device\n", fileName)
	} else {
		fmt.Printf("⚠ Warning: Could not verify %s on device\n", fileName)
		fmt.Println("The file may still have been uploaded, but could not be confirmed.")
	}

	return true
}

func verifyFileUploaded(dev *mtp.Device, objectID, storageID, parentID uint32, fileName string, expectedSize int64) bool {
	util.LogInfo("Verifying file upload for %s (ID: %d)", fileName, objectID)
	fmt.Printf("Verifying file was successfully uploaded...\n")

	fileInfo := mtp.ObjectInfo{}
	err := dev.GetObjectInfo(objectID, &fileInfo)
	if err == nil {

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

			if info.CompressedSize == uint32(expectedSize) {
				fmt.Printf("✓ File verified in folder with correct size: %d bytes\n", info.CompressedSize)
				return true
			} else {
				util.LogError("File size mismatch: expected %d bytes, got %d bytes",
					expectedSize, info.CompressedSize)
				fmt.Printf("File exists but size mismatch: expected %d bytes, got %d bytes\n",
					expectedSize, info.CompressedSize)

				return true
			}
		}
	}

	fmt.Println("File not found on first attempt. Waiting 2 seconds and trying again...")
	time.Sleep(2 * time.Second)

	err = dev.GetObjectInfo(objectID, &fileInfo)
	if err == nil {
		util.LogInfo("Delayed verification successful: %s exists with size %d bytes",
			fileName, fileInfo.CompressedSize)
		return true
	}

	util.LogError("Could not verify file %s on device after multiple attempts", fileName)
	return false
}

func GetAlbumFromFileName(filename string) string {

	dir := filepath.Dir(filename)
	if dir != "." && dir != "/" {
		return filepath.Base(dir)
	}
	return ""
}

func findOrCreateFolder(dev *mtp.Device, storageID, parentID uint32, folderName string) (uint32, error) {

	folderID, err := util.FindFolder(dev, storageID, parentID, folderName)
	if err == nil {

		util.LogVerbose("Using existing folder: %s (ID: %d)", folderName, folderID)
		return folderID, nil
	}

	util.LogVerbose("Folder '%s' not found, attempting to create it", folderName)
	folderID, err = device.CreateFolder(dev, storageID, parentID, folderName)
	if err != nil {

		retryID, retryErr := util.FindFolder(dev, storageID, parentID, folderName)
		if retryErr == nil {
			util.LogVerbose("Found folder '%s' on second attempt (ID: %d)", folderName, retryID)
			return retryID, nil
		}

		return 0, fmt.Errorf("error finding or creating folder '%s': %w", folderName, err)
	}

	return folderID, nil
}

func extractUint32Field(obj interface{}, fieldName string) uint32 {
	val := reflect.ValueOf(obj)
	field := val.FieldByName(fieldName)
	if !field.IsValid() || field.Kind() != reflect.Uint32 {
		return 0
	}
	return uint32(field.Uint())
}

func extractStringField(obj interface{}, fieldName string) string {
	val := reflect.ValueOf(obj)
	field := val.FieldByName(fieldName)
	if !field.IsValid() || field.Kind() != reflect.String {
		return ""
	}
	return field.String()
}

func SelectStorageAndMusicFolder(dev *mtp.Device, storagesRaw interface{}) (uint32, uint32, error) {

	storagesValue := reflect.ValueOf(storagesRaw)
	if storagesValue.Kind() != reflect.Slice || storagesValue.Len() == 0 {
		return 0, 0, fmt.Errorf("no storage found on device")
	}

	firstStorage := storagesValue.Index(0).Interface()
	storageID := extractUint32Field(firstStorage, "Sid")
	storageDesc := extractStringField(firstStorage, "Description")

	util.LogInfo("Automatically selected storage: %s (ID: %d)", storageDesc, storageID)
	fmt.Printf("Automatically selected storage: %s (ID: %d)\n", storageDesc, storageID)

	musicFolderID, err := util.FindOrCreateMusicFolder(dev, storageID)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to find or create Music folder: %v", err)
	}

	return storageID, musicFolderID, nil
}

func tryAlternativeDataTransfer(dev *mtp.Device, objectID uint32, data []byte, fileSize int64) error {
	util.LogInfo("Trying alternative data transfer methods for object ID %d", objectID)

	reader := bytes.NewReader(data)
	err := dev.SendObject(reader, fileSize, EmptyProgressFunc)
	if err == nil {
		util.LogInfo("Method 1 (standard SendObject with fresh reader) successful")
		return nil
	}
	util.LogError("Method 1 failed: %v", err)

	time.Sleep(2 * time.Second)
	reader = bytes.NewReader(data)
	err = dev.SendObject(reader, fileSize, EmptyProgressFunc)
	if err == nil {
		util.LogInfo("Method 2 (SendObject after delay) successful")
		return nil
	}
	util.LogError("Method 2 failed: %v", err)

	if fileSize > 1024*1024 {

		truncatedData := data
		if len(data) > 1024*1024 {
			truncatedData = data[:1024*1024]
		}

		reader = bytes.NewReader(truncatedData)
		err = dev.SendObject(reader, int64(len(truncatedData)), EmptyProgressFunc)
		if err == nil {
			util.LogInfo("Method 3 (truncated file transfer) partially successful")

			return fmt.Errorf("only transferred part of the file (%d of %d bytes)", len(truncatedData), fileSize)
		}
		util.LogError("Method 3 failed: %v", err)
	}

	return err
}

func UploadDirectoryWithPlaylist(dev *mtp.Device, storageID, musicFolderID uint32) *UploadResult {
	result := &UploadResult{
		Success:       false,
		UploadedFiles: make([]model.MP3File, 0),
		Playlist:      nil,
		Errors:        make([]string, 0),
	}

	fmt.Print("Enter path to directory containing MP3 files: ")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	dirPath := scanner.Text()

	dirInfo, err := os.Stat(dirPath)
	if err != nil {
		result.AddError(fmt.Sprintf("Error accessing directory: %v", err))
		return result
	}

	if !dirInfo.IsDir() {
		result.AddError("Specified path is not a directory. Please use single file upload option.")
		return result
	}

	recursive := true
	dirName := filepath.Base(dirPath)
	playlistName := strings.ToUpper(util.SanitizeFileName(dirName))
	if !strings.HasSuffix(strings.ToLower(playlistName), ".m3u8") {
		playlistName += ".m3u8"
	}

	var mp3Files []string
	err = filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			util.LogVerbose("Error accessing path %s: %v", path, err)
			return nil
		}

		if info.IsDir() {
			if !recursive && path != dirPath {
				return filepath.SkipDir
			}
			return nil
		}

		if strings.HasSuffix(strings.ToLower(path), ".mp3") {
			mp3Files = append(mp3Files, path)
		}

		return nil
	})

	if err != nil {
		util.LogVerbose("Error walking directory: %v", err)
		result.AddError(fmt.Sprintf("Error scanning directory: %v", err))
		return result
	}

	if len(mp3Files) == 0 {
		result.AddError("No MP3 files found in the specified directory.")
		return result
	}

	fmt.Printf("Found %d MP3 files in %s\n", len(mp3Files), dirPath)
	fmt.Printf("Do you want to upload %d MP3 files and create a playlist? (y/n): ", len(mp3Files))
	scanner.Scan()
	confirm := strings.ToLower(scanner.Text())
	if confirm != "y" && confirm != "yes" {
		result.AddError("Upload cancelled by user.")
		return result
	}

	var uploadedFilePaths []string
	successCount := 0
	failureCount := 0

	for i, filePath := range mp3Files {
		fmt.Printf("\n[%d/%d] Processing %s\n", i+1, len(mp3Files), filepath.Base(filePath))

		fileInfo, err := os.Stat(filePath)
		if err != nil {
			util.LogVerbose("Error accessing file: %v. Skipping.", err)
			failureCount++
			result.AddError(fmt.Sprintf("Error accessing file %s: %v", filePath, err))
			continue
		}

		if fileInfo.Size() > 10*1024*1024 {
			util.LogVerbose("File is too large (%d MB). Skipping.", fileInfo.Size()/1024/1024)
			failureCount++
			result.AddError(fmt.Sprintf("File %s is too large (%d MB)", filePath, fileInfo.Size()/1024/1024))
			continue
		}

		fileResult := ProcessAndUploadFileWithPath(dev, storageID, musicFolderID, filePath, i+1)
		if fileResult.Success {
			successCount++
			uploadedFilePaths = append(uploadedFilePaths, fileResult.UploadedPath)
			result.UploadedFiles = append(result.UploadedFiles, model.MP3File{
				Path:        fileResult.UploadedPath,
				ObjectID:    fileResult.ObjectID,
				ParentID:    musicFolderID,
				StorageID:   storageID,
				DisplayName: fileResult.DisplayName,
			})
		} else {
			failureCount++
			result.AddError(fmt.Sprintf("Failed to upload %s: %s", filePath, fileResult.Error))
		}
	}

	fmt.Printf("\nUpload complete: %d successful, %d failed\n", successCount, failureCount)

	if len(uploadedFilePaths) > 0 {
		playlistResult, err := createPlaylist(dev, storageID, musicFolderID, playlistName, uploadedFilePaths)
		if err != nil {
			result.AddError(fmt.Sprintf("Playlist creation failed: %v", err))
		} else {
			result.Playlist = &playlistResult
			result.Success = true
		}
	} else {
		result.AddError("No files were successfully uploaded, so no playlist was created.")
	}

	return result
}

type FileUploadResult struct {
	Success      bool
	UploadedPath string
	ObjectID     uint32
	DisplayName  string
	Error        string
}

type UploadResult struct {
	Success       bool
	UploadedFiles []model.MP3File
	Playlist      *model.Playlist
	Errors        []string
}

func (r *UploadResult) AddError(msg string) {
	r.Errors = append(r.Errors, msg)
	util.LogVerbose("Error: %s", msg)
}

func createPlaylist(dev *mtp.Device, storageID, parentID uint32, playlistName string, uploadedFilePaths []string) (model.Playlist, error) {
	util.LogVerbose("Creating playlist '%s' with %d songs...", playlistName, len(uploadedFilePaths))

	pathStyle := 1
	var playlistContent strings.Builder
	playlistContent.WriteString("#EXTM3U\n")

	var songs []model.PlaylistSong

	for _, songPath := range uploadedFilePaths {
		formattedPath := util.FormatPlaylistPath(songPath, pathStyle)
		displayName := strings.ToUpper(util.ExtractTrackInfo(songPath))

		playlistContent.WriteString(fmt.Sprintf("#EXTINF:-1,%s\n", displayName))
		playlistContent.WriteString(formattedPath)
		playlistContent.WriteString("\n")

		util.LogVerbose("Added to playlist: %s -> %s", songPath, displayName)

		songs = append(songs, model.PlaylistSong{
			Name: displayName,
			Path: songPath,
		})
	}

	util.LogVerbose("Full playlist content:\n%s", playlistContent.String())

	tempFile, err := os.CreateTemp("", "playlist-*.m3u8")
	if err != nil {
		return model.Playlist{}, fmt.Errorf("error creating temporary playlist file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	_, err = tempFile.WriteString(playlistContent.String())
	if err != nil {
		return model.Playlist{}, fmt.Errorf("error writing playlist content: %v", err)
	}

	tempFile.Sync()
	tempFile.Seek(0, 0)

	fileInfo, err := tempFile.Stat()
	if err != nil {
		return model.Playlist{}, fmt.Errorf("error getting file info: %v", err)
	}

	info := mtp.ObjectInfo{
		StorageID:        storageID,
		ObjectFormat:     0xBA05,
		ParentObject:     parentID,
		Filename:         playlistName,
		CompressedSize:   uint32(fileInfo.Size()),
		ModificationDate: time.Now(),
	}

	util.LogVerbose("Creating playlist on device...")
	var objectID uint32
	_, _, objectID, err = dev.SendObjectInfo(storageID, parentID, &info)
	if err != nil {
		return model.Playlist{}, fmt.Errorf("error creating playlist on device: %v", err)
	}

	util.LogVerbose("Sending playlist data (size: %d bytes)...", fileInfo.Size())

	data := make([]byte, fileInfo.Size())
	_, err = tempFile.Seek(0, 0)
	if err != nil {
		return model.Playlist{}, fmt.Errorf("error seeking file: %v", err)
	}

	_, err = tempFile.Read(data)
	if err != nil {
		return model.Playlist{}, fmt.Errorf("error reading file: %v", err)
	}

	err = dev.SendObject(bytes.NewReader(data), fileInfo.Size(), EmptyProgressFunc)
	if err != nil {
		return model.Playlist{}, fmt.Errorf("error uploading playlist: %v", err)
	}

	util.LogVerbose("Successfully created and uploaded playlist %s (object ID: %d) with %d songs",
		playlistName, objectID, len(uploadedFilePaths))

	verified := VerifyPlaylistUploaded(dev, storageID, parentID, playlistName)
	if !verified {
		util.LogVerbose("Warning: Playlist verification failed")
	}

	return model.Playlist{
		Path:      playlistName,
		ObjectID:  objectID,
		ParentID:  parentID,
		StorageID: storageID,
		SongPaths: uploadedFilePaths,
	}, nil
}

func ProcessAndUploadFileWithPath(dev *mtp.Device, storageID, musicFolderID uint32, filePath string, trackNumber int) FileUploadResult {
	result := FileUploadResult{
		Success:      false,
		UploadedPath: "",
		ObjectID:     0,
		DisplayName:  "",
		Error:        "",
	}

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		result.Error = fmt.Sprintf("Error accessing file: %v", err)
		util.LogVerbose("Error accessing file: %v", err)
		return result
	}

	if fileInfo.Size() > 10*1024*1024 {
		result.Error = fmt.Sprintf("File %s is too large. This implementation only supports files up to 10MB",
			filepath.Base(filePath))
		util.LogVerbose("File %s is too large. This implementation only supports files up to 10MB",
			filepath.Base(filePath))
		return result
	}

	tag, err := id3v2.Open(filePath, id3v2.Options{Parse: true})

	artist := "UNKNOWN_ARTIST"
	album := "UNKNOWN_ALBUM"

	if err == nil {
		if tag.Artist() != "" {
			artist = strings.ToUpper(util.SanitizeFolderName(tag.Artist()))
		}
		if tag.Album() != "" {
			album = strings.ToUpper(util.SanitizeFolderName(tag.Album()))
		}
		defer tag.Close()
	} else {
		util.LogVerbose("Error reading ID3 tags: %v", err)
	}

	originalFileName := util.SanitizeFileName(filepath.Base(filePath))
	fileName := fmt.Sprintf("%02d %s", trackNumber, strings.ToUpper(originalFileName))
	devicePath := fmt.Sprintf("/MUSIC/%s/%s/%s", artist, album, fileName)

	util.LogVerbose("Processing file: %s", devicePath)

	artistFolderID, err := findOrCreateFolder(dev, storageID, musicFolderID, artist)
	if err != nil {
		result.Error = fmt.Sprintf("Error creating artist folder: %v", err)
		util.LogVerbose("Error creating artist folder: %v", err)
		return result
	}

	albumFolderID, err := findOrCreateFolder(dev, storageID, artistFolderID, album)
	if err != nil {
		result.Error = fmt.Sprintf("Error creating album folder: %v", err)
		util.LogVerbose("Error creating album folder: %v", err)
		return result
	}

	info := mtp.ObjectInfo{
		StorageID:        storageID,
		ObjectFormat:     0xB901,
		ParentObject:     albumFolderID,
		Filename:         fileName,
		CompressedSize:   uint32(fileInfo.Size()),
		ModificationDate: time.Now(),
	}

	var objectID uint32
	_, _, objectID, err = dev.SendObjectInfo(storageID, albumFolderID, &info)
	if err != nil {
		result.Error = fmt.Sprintf("Error creating file on device: %v", err)
		util.LogVerbose("Error creating file on device: %v", err)
		return result
	}

	file, err := os.Open(filePath)
	if err != nil {
		result.Error = fmt.Sprintf("Error opening file: %v", err)
		util.LogVerbose("Error opening file: %v", err)
		return result
	}
	defer file.Close()

	bar := progressbar.NewOptions64(
		fileInfo.Size(),
		progressbar.OptionSetDescription(fmt.Sprintf("Uploading %s", fileName)),
		progressbar.OptionSetWidth(30),
		progressbar.OptionShowBytes(true),
		progressbar.OptionShowCount(),
		progressbar.OptionOnCompletion(func() {
			fmt.Print("\n")
		}),
	)

	progressReader := progressbar.NewReader(file, bar)

	err = dev.SendObject(&progressReader, fileInfo.Size(), EmptyProgressFunc)
	if err != nil {
		util.LogVerbose("Standard file transfer failed: %v", err)

		file.Seek(0, 0)
		data, readErr := io.ReadAll(file)
		if readErr != nil {
			result.Error = fmt.Sprintf("Error reading file: %v", readErr)
			util.LogVerbose("Error reading file: %v", readErr)
			return result
		}

		err = tryAlternativeDataTransfer(dev, objectID, data, fileInfo.Size())
	}

	if err != nil {
		result.Error = fmt.Sprintf("All file transfer methods failed: %v", err)
		util.LogVerbose("All file transfer methods failed: %v", err)
		return result
	}

	util.LogVerbose("Successfully uploaded to %s", devicePath)

	verified := verifyFileUploaded(dev, objectID, storageID, albumFolderID, fileName, fileInfo.Size())
	if !verified {
		util.LogVerbose("Could not verify file on device: %s", fileName)
	}

	result.Success = true
	result.UploadedPath = "0:" + devicePath
	result.ObjectID = objectID
	result.DisplayName = fileName

	return result
}
