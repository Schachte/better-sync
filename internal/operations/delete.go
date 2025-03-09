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

	"github.com/ganeshrvel/go-mtpfs/mtp"
	"github.com/ganeshrvel/go-mtpx"
	"github.com/schachte/better-sync/internal/model"
	"github.com/schachte/better-sync/internal/util"
)

func DeletePlaylist(dev *mtp.Device, storagesRaw interface{}) {
	fmt.Println("\n=== Delete Playlist ===")
	util.LogVerbose("Starting playlist deletion operation")

	// Get all playlists from all storages
	storagesValue := reflect.ValueOf(storagesRaw)
	if storagesValue.Kind() != reflect.Slice {
		util.LogError("Error: Storages is not a slice")
		return
	}

	// Collect all playlists from all storages
	allPlaylists := make([]model.PlaylistEntry, 0)

	fmt.Println("Scanning for playlists...")

	for i := 0; i < storagesValue.Len(); i++ {
		storageObj := storagesValue.Index(i).Interface()
		storageID := util.ExtractUint32Field(storageObj, "Sid")
		storageDesc := util.ExtractStringField(storageObj, "StorageDescription")
		if storageDesc == "" {
			storageDesc = util.ExtractStringField(storageObj, "Description")
		}

		playlists, err := FindPlaylists(dev, storageID)
		if err != nil {
			util.LogError("Error finding playlists in storage %s: %v", storageDesc, err)
			fmt.Printf("Error scanning storage: %v\n", err)
			continue
		}

		for _, playlist := range playlists {
			objectID, err := FindObjectByPath(dev, storageID, playlist)
			if err != nil {
				util.LogError("Error finding object ID for playlist %s: %v", playlist, err)
				continue
			}

			// GetObjectInfo needs an object to populate
			info := mtp.ObjectInfo{}
			err = dev.GetObjectInfo(objectID, &info)
			if err != nil {
				util.LogError("Error getting object info for playlist %s: %v", playlist, err)
				continue
			}

			allPlaylists = append(allPlaylists, model.PlaylistEntry{
				StorageID:   storageID,
				Path:        playlist,
				ObjectID:    objectID,
				ParentID:    info.ParentObject,
				StorageDesc: storageDesc,
			})
		}
	}

	if len(allPlaylists) == 0 {
		fmt.Println("No playlists found on the device")
		return
	}

	// Show playlists
	fmt.Println("\n==== Available Playlists ====")
	for i, playlist := range allPlaylists {
		fmt.Printf("%d. [%s] %s\n", i+1, playlist.StorageDesc, filepath.Base(playlist.Path))
	}

	// Get selection
	fmt.Print("\nSelect playlist to delete (1-" + fmt.Sprint(len(allPlaylists)) + "): ")
	var selection int
	fmt.Scanln(&selection)

	if selection < 1 || selection > len(allPlaylists) {
		fmt.Println("Invalid selection")
		return
	}

	selected := allPlaylists[selection-1]

	// Confirm deletion
	fmt.Printf("Are you sure you want to delete the playlist '%s'? (y/n): ", filepath.Base(selected.Path))
	var confirm string
	fmt.Scanln(&confirm)

	if strings.ToLower(confirm) != "y" {
		fmt.Println("Operation cancelled")
		return
	}

	// Delete the playlist
	err := dev.DeleteObject(selected.ObjectID)
	if err != nil {
		util.LogError("Error deleting playlist: %v", err)
		fmt.Printf("Error deleting playlist: %v\n", err)

		// Try alternative method
		fmt.Println("Trying alternative deletion method...")
		err = tryAlternativeDeleteMethod(dev, selected.StorageID, selected.ObjectID)
		if err != nil {
			fmt.Printf("Alternative deletion failed: %v\n", err)
			return
		}
	}

	fmt.Println("Playlist deleted successfully")
}

func DeleteSong(dev *mtp.Device, storagesRaw interface{}) {
	util.LogInfo("Starting song deletion operation")

	// Get all songs from all storages
	storagesValue := reflect.ValueOf(storagesRaw)
	if storagesValue.Kind() != reflect.Slice {
		util.LogError("Error: Storages is not a slice")
		return
	}

	// Collect all songs from all storages
	type SongEntry struct {
		StorageID   uint32
		Path        string
		ObjectID    uint32
		ParentID    uint32
		StorageDesc string
		DisplayName string
	}
	allSongs := make([]SongEntry, 0)

	for i := 0; i < storagesValue.Len(); i++ {
		storageObj := storagesValue.Index(i).Interface()
		storageID := extractUint32Field(storageObj, "Sid")
		description := extractStringField(storageObj, "StorageDescription")
		if description == "" {
			description = extractStringField(storageObj, "Description")
		}

		fmt.Printf("Searching storage #%d: %s (ID: %d) for songs...\n", i+1, description, storageID)
		util.LogInfo("Searching storage #%d: %s (ID: %d) for songs", i+1, description, storageID)

		// Find all MP3 files in the storage
		songPaths, err := FindMP3Files(dev, storageID)
		if err != nil {
			util.LogError("Error finding songs in storage #%d: %v", i+1, err)
			continue
		}

		// For each song, find its object ID
		for _, path := range songPaths {
			// Get the filename from the path
			filename := filepath.Base(path)

			// Get the parent directory path
			parentPath := filepath.Dir(path)
			if parentPath == "." || parentPath == "/" {
				parentPath = ""
			}

			// Find the parent ID by path or default to root
			parentID := PARENT_ROOT
			if parentPath != "" {
				parentID, err = GetFolderIDByPath(dev, storageID, parentPath)
				if err != nil {
					util.LogVerbose("Could not find parent folder for %s: %v", path, err)
					// Continue with root parent
				}
			}

			// Get all objects in the parent folder
			handles := mtp.Uint32Array{}
			err = dev.GetObjectHandles(storageID, 0, parentID, &handles)
			if err != nil {
				util.LogVerbose("Error getting objects in folder %s: %v", parentPath, err)
				continue
			}

			// Find the object with matching filename
			found := false
			var objectID uint32
			for _, handle := range handles.Values {
				info := mtp.ObjectInfo{}
				err = dev.GetObjectInfo(handle, &info)
				if err != nil {
					util.LogVerbose("Error getting object info for handle %d: %v", handle, err)
					continue
				}

				if info.Filename == filename {
					objectID = handle
					found = true
					break
				}
			}

			if found {
				// Extract display name for better readability
				displayName := util.ExtractTrackInfo(path)

				allSongs = append(allSongs, SongEntry{
					StorageID:   storageID,
					Path:        path,
					ObjectID:    objectID,
					ParentID:    parentID,
					StorageDesc: description,
					DisplayName: displayName,
				})
			} else {
				util.LogVerbose("Found song path %s but could not locate its object ID", path)
			}
		}
	}

	if len(allSongs) == 0 {
		fmt.Println("No songs found on the device.")
		return
	}

	// Display all songs to the user
	fmt.Println("\nAvailable songs to delete:")
	for i, song := range allSongs {
		fmt.Printf("%d. %s (%s)\n", i+1, song.DisplayName, song.Path)
	}

	// Let user select a song to delete
	fmt.Print("\nEnter the number of the song to delete (0 to cancel): ")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	input := scanner.Text()
	var index int
	if _, err := fmt.Sscanf(input, "%d", &index); err != nil || index <= 0 || index > len(allSongs) {
		if index == 0 {
			fmt.Println("Operation cancelled.")
		} else {
			fmt.Println("Invalid selection.")
		}
		return
	}

	// Get the selected song
	selectedSong := allSongs[index-1]

	// Confirm deletion
	fmt.Printf("\nAre you sure you want to delete the song '%s'? (y/n): ", selectedSong.DisplayName)
	scanner.Scan()
	confirm := strings.ToLower(scanner.Text())
	if confirm != "y" && confirm != "yes" {
		fmt.Println("Deletion cancelled.")
		return
	}

	// Add an extra warning for songs that might be in playlists
	fmt.Println("\nWARNING: If this song is included in any playlists, those playlists may be affected.")
	fmt.Print("Continue with deletion? (y/n): ")
	scanner.Scan()
	confirmAgain := strings.ToLower(scanner.Text())
	if confirmAgain != "y" && confirmAgain != "yes" {
		fmt.Println("Deletion cancelled.")
		return
	}

	// Attempt to delete the song
	fmt.Printf("Deleting song '%s'...\n", selectedSong.DisplayName)
	util.LogInfo("Attempting to delete song %s (Path: %s, ObjectID: %d, StorageID: %d)",
		selectedSong.DisplayName, selectedSong.Path, selectedSong.ObjectID, selectedSong.StorageID)

	err := dev.DeleteObject(selectedSong.ObjectID)
	if err != nil {
		util.LogError("Error deleting song: %v", err)
		fmt.Printf("Error deleting song: %v\n", err)

		// Offer retry with different approach if deletion failed
		fmt.Print("\nWould you like to try an alternative deletion method? (y/n): ")
		scanner.Scan()
		retry := strings.ToLower(scanner.Text())
		if retry == "y" || retry == "yes" {
			// Try alternative deletion method
			err = tryAlternativeDeleteMethod(dev, selectedSong.StorageID, selectedSong.ObjectID)
			if err != nil {
				util.LogError("Alternative deletion method failed: %v", err)
				fmt.Printf("Alternative deletion method failed: %v\n", err)

				fmt.Println("\nTroubleshooting tips:")
				fmt.Println("1. Some devices don't support deleting files via MTP")
				fmt.Println("2. Try deleting the song using the device's interface")
				fmt.Println("3. You might need to disconnect and reconnect the device")
				fmt.Println("4. The song might be currently playing or locked by the device")
				return
			} else {
				fmt.Printf("Successfully deleted song '%s' using alternative method\n", selectedSong.DisplayName)
				util.LogInfo("Successfully deleted song %s using alternative method", selectedSong.DisplayName)
			}
		} else {
			return
		}
	} else {
		fmt.Printf("Successfully deleted song '%s'\n", selectedSong.DisplayName)
		util.LogInfo("Successfully deleted song %s", selectedSong.DisplayName)
	}

	fmt.Println("\nNote: If the song was part of any playlists, you may need to update those playlists manually.")
}

func tryAlternativeDeleteMethod(dev *mtp.Device, storageID, objectID uint32) error {
	util.LogInfo("Trying alternative deletion method for object ID %d", objectID)

	// Some devices require sending a specific MTP command rather than using DeleteObject
	// This is just one example of an alternative approach

	// 1. Try setting the object to zero size first (this works on some devices)
	info := mtp.ObjectInfo{}
	err := dev.GetObjectInfo(objectID, &info)
	if err != nil {
		return fmt.Errorf("failed to get object info: %v", err)
	}

	// Modify the object info to have zero size
	info.CompressedSize = 0

	// Send the modified object info
	_, _, _, err = dev.SendObjectInfo(storageID, info.ParentObject, &info)
	if err != nil {
		return fmt.Errorf("failed to send modified object info: %v", err)
	}

	// 2. Now try deleting the emptied object
	err = dev.DeleteObject(objectID)
	if err != nil {
		return fmt.Errorf("failed to delete modified object: %v", err)
	}

	return nil
}

func FindObjectByDirectPath(dev *mtp.Device, storageID uint32, path string) (uint32, error) {
	normalizedPath := normalizePath(path)

	// Try to find the object using Walk with exact case match
	var foundObject uint32
	var found bool

	_, _, _, _ = mtpx.Walk(dev, storageID, "/", true, true, false,
		func(objectID uint32, fi *mtpx.FileInfo, err error) error {
			if err != nil {
				return nil // Skip errors and continue
			}

			// Check for exact match
			if normalizePath(fi.FullPath) == normalizedPath {
				foundObject = objectID
				found = true
				return fmt.Errorf("found") // Use error to break out of walk
			}

			return nil
		})

	if found {
		util.LogVerbose("Found object with ID: %d for path: %s", foundObject, path)
		return foundObject, nil
	}

	return 0, fmt.Errorf("object not found using direct path: %s", path)
}

func FindSongByMixedCaseAndRelativePath(dev *mtp.Device, storageID uint32, path string) (uint32, error) {
	util.LogVerbose("Trying flexible matching for path: %s", path)

	// Extract filename and strip potential path prefixes
	fileName := filepath.Base(path)

	// Handle potential numeric prefix patterns (01 - Song.mp3, 01_Song.mp3, etc.)
	fileNameNoNumber := stripNumericPrefix(fileName)

	// For the folder path, extract key components
	folderPath := filepath.Dir(path)
	pathComponents := strings.Split(folderPath, "/")

	// Identify potential artist and album folders from the path if available
	var artistFolder, albumFolder string
	if len(pathComponents) >= 3 { // Has enough components for MUSIC/ARTIST/ALBUM structure
		for i, comp := range pathComponents {
			if strings.EqualFold(comp, "Music") && i+2 < len(pathComponents) {
				artistFolder = pathComponents[i+1]
				albumFolder = pathComponents[i+2]
				break
			}
		}
	}

	var foundObject uint32
	var found bool
	var matchReason string

	// Walk through all objects to find a match using multiple criteria
	_, _, _, _ = mtpx.Walk(dev, storageID, "/", true, true, false,
		func(objectID uint32, fi *mtpx.FileInfo, err error) error {
			if err != nil {
				return nil // Skip errors and continue
			}

			// Only check MP3 files
			if fi.IsDir || !strings.HasSuffix(strings.ToLower(fi.FullPath), ".mp3") {
				return nil
			}

			// Method 1: Check for exact basename match (case insensitive)
			itemFileName := filepath.Base(fi.FullPath)
			if strings.EqualFold(itemFileName, fileName) {
				foundObject = objectID
				found = true
				matchReason = "exact filename match"
				return fmt.Errorf("found") // Break out of walk
			}

			// Method 2: Check for filename match without numeric prefix
			itemFileNameNoNumber := stripNumericPrefix(itemFileName)
			if fileNameNoNumber != "" && strings.EqualFold(itemFileNameNoNumber, fileNameNoNumber) {
				foundObject = objectID
				found = true
				matchReason = "filename match without numeric prefix"
				return fmt.Errorf("found") // Break out of walk
			}

			// Method 3: If we have artist/album info, check if those appear in the path
			if artistFolder != "" && albumFolder != "" {
				itemPath := fi.FullPath
				if strings.Contains(strings.ToUpper(itemPath), strings.ToUpper(artistFolder)) &&
					strings.Contains(strings.ToUpper(itemPath), strings.ToUpper(albumFolder)) &&
					strings.Contains(strings.ToUpper(itemFileName), strings.ToUpper(fileNameNoNumber)) {
					foundObject = objectID
					found = true
					matchReason = "artist/album/name pattern match"
					return fmt.Errorf("found") // Break out of walk
				}
			}

			return nil
		})

	if found {
		util.LogVerbose("Found object with ID: %d for path: %s (%s)", foundObject, path, matchReason)
		return foundObject, nil
	}

	return 0, fmt.Errorf("object not found after flexible matching: %s", path)
}

func stripNumericPrefix(filename string) string {
	// Remove file extension first
	baseName := strings.TrimSuffix(filename, filepath.Ext(filename))

	// Common patterns: "01 - Song", "01_Song", "01 Song", "01-Song"
	// First, check if it starts with digits
	if len(baseName) < 3 {
		return baseName // Too short to have a prefix
	}

	// Check if the first two characters are digits
	if baseName[0] >= '0' && baseName[0] <= '9' && baseName[1] >= '0' && baseName[1] <= '9' {
		// Find where the actual name starts after the prefix
		for i := 2; i < len(baseName); i++ {
			if baseName[i] >= 'A' && baseName[i] <= 'Z' ||
				baseName[i] >= 'a' && baseName[i] <= 'z' {
				return baseName[i:]
			}

			// Skip common prefix separators
			if baseName[i] != ' ' && baseName[i] != '-' && baseName[i] != '_' {
				break
			}
		}
	}

	return baseName
}

func normalizePath(path string) string {
	// Remove common prefixes
	path = strings.TrimPrefix(path, "0:")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// Ensure single slashes
	for strings.Contains(path, "//") {
		path = strings.Replace(path, "//", "/", -1)
	}

	return path
}

func ExtractPlaylistSongPaths(dev *mtp.Device, storageID, objectID uint32, playlistPath string) ([]string, error) {
	// Read the playlist content
	songs, err := ReadPlaylistContent(dev, storageID, objectID)
	if err != nil {
		return nil, err
	}

	return songs, nil
}

func ReadPlaylistContent(dev *mtp.Device, storageID, objectID uint32) ([]string, error) {
	// We'll try to get the playlist content using GetObject
	var buf bytes.Buffer

	// Try to get the object data
	err := dev.GetObject(objectID, &buf, EmptyProgressFunc)
	if err != nil {
		return nil, fmt.Errorf("error reading playlist: %v", err)
	}

	content := buf.String()
	return ParsePlaylistContent(content), nil
}

func ParsePlaylistContent(content string) []string {
	var songs []string

	// Split content into lines
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines and comments (lines starting with #)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Normalize the path
		songPath := line

		// Add to songs list
		songs = append(songs, songPath)
	}

	return songs
}

func DeletePlaylistOnly(dev *mtp.Device, playlistObjectID uint32) {
	fmt.Println("Deleting only the playlist...")
	err := dev.DeleteObject(playlistObjectID)
	if err != nil {
		util.LogError("Error deleting playlist (ID: %d): %v", playlistObjectID, err)
		fmt.Printf("ERROR: Could not delete playlist: %v\n", err)

		// Try alternative deletion method
		fmt.Println("Trying alternative deletion method...")
		err = tryAlternativeDeleteMethod(dev, 0, playlistObjectID) // 0 is a placeholder for storageID, not used in this context
		if err != nil {
			util.LogError("Alternative deletion method failed: %v", err)
			fmt.Printf("Alternative deletion method failed: %v\n", err)
			fmt.Println("Playlist could not be deleted.")
		} else {
			fmt.Println("Successfully deleted playlist using alternative method.")
		}
	} else {
		fmt.Println("Successfully deleted playlist.")
	}
}

func TryAlternativeDeleteMethod(dev *mtp.Device, storageID, objectID uint32) error {
	util.LogInfo("Trying alternative deletion method for object ID %d", objectID)

	// Some devices require sending a specific MTP command rather than using DeleteObject
	// This is just one example of an alternative approach

	// 1. Try setting the object to zero size first (this works on some devices)
	info := mtp.ObjectInfo{}
	err := dev.GetObjectInfo(objectID, &info)
	if err != nil {
		return fmt.Errorf("failed to get object info: %v", err)
	}

	// Modify the object info to have zero size
	info.CompressedSize = 0

	// Send the modified object info
	_, _, _, err = dev.SendObjectInfo(storageID, info.ParentObject, &info)
	if err != nil {
		return fmt.Errorf("failed to send modified object info: %v", err)
	}

	// 2. Now try deleting the emptied object
	err = dev.DeleteObject(objectID)
	if err != nil {
		return fmt.Errorf("failed to delete modified object: %v", err)
	}

	return nil
}

func DeleteFolderRecursively(dev *mtp.Device, storageID, folderID uint32, folderPath string, requireConfirmation bool) error {
	// Safety check only for root folder
	if folderPath == "/" {
		return fmt.Errorf("refusing to delete root folder")
	}

	// Extra warning for Music folder deletions
	lowerPath := strings.ToLower(folderPath)
	if (lowerPath == "/music" || lowerPath == "/music/" || lowerPath == "/music") && requireConfirmation {
		fmt.Printf("\n!!! EXTREME CAUTION !!!\n")
		fmt.Printf("You are about to delete the ENTIRE MUSIC FOLDER and ALL its contents.\n")
		fmt.Printf("This will remove ALL playlists, songs, and artist folders.\n")
		fmt.Printf("Type 'ERASE EVERYTHING' (all caps) to proceed: ")

		var extremeConfirm string
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		extremeConfirm = scanner.Text()

		if extremeConfirm != "ERASE EVERYTHING" {
			return fmt.Errorf("main Music folder deletion cancelled")
		}
	}

	// Confirm deletion if required
	if requireConfirmation {
		fmt.Printf("WARNING: About to delete folder '%s' and ALL its contents. Continue? (y/n): ", folderPath)
		var confirm string
		fmt.Scanln(&confirm)
		if strings.ToLower(confirm) != "y" && strings.ToLower(confirm) != "yes" {
			return fmt.Errorf("operation cancelled by user")
		}
	}

	util.LogInfo("Starting recursive deletion of folder: %s (ID: %d)", folderPath, folderID)
	fmt.Printf("Deleting folder: %s\n", folderPath)

	// Get all objects in this folder
	handles := mtp.Uint32Array{}
	err := dev.GetObjectHandles(storageID, 0, folderID, &handles)
	if err != nil {
		return fmt.Errorf("error getting object handles: %w", err)
	}

	// Log the number of items to delete
	util.LogInfo("Found %d items in folder %s", len(handles.Values), folderPath)
	fmt.Printf("Found %d items to process\n", len(handles.Values))

	// Keep track of deletion stats
	deletedFiles := 0
	deletedFolders := 0
	failedItems := 0

	// Process all objects in this folder
	for i, handle := range handles.Values {
		// Show progress
		fmt.Printf("[%d/%d] ", i+1, len(handles.Values))

		info := mtp.ObjectInfo{}
		err := dev.GetObjectInfo(handle, &info)
		if err != nil {
			util.LogError("Error getting object info for handle %d: %v", handle, err)
			failedItems++
			continue
		}

		itemPath := filepath.Join(folderPath, info.Filename)

		// If it's a folder, recurse into it first
		if info.ObjectFormat == FILETYPE_FOLDER {
			fmt.Printf("Processing subfolder: %s\n", info.Filename)
			subErr := DeleteFolderRecursively(dev, storageID, handle, itemPath, false) // No confirmation needed for subfolders
			if subErr != nil {
				util.LogError("Error deleting subfolder %s: %v", itemPath, subErr)
				failedItems++
				// Continue with other objects even if this subfolder fails
			} else {
				deletedFolders++
			}
			// Skip deleting the folder itself as it's handled by the recursive call
			continue
		}

		// It's a file, delete it
		fmt.Printf("Deleting file: %s\n", info.Filename)

		// Try multiple times to delete the file
		deleteSuccess := false
		for attempt := 1; attempt <= 3; attempt++ {
			err = dev.DeleteObject(handle)
			if err == nil {
				deleteSuccess = true
				break
			}

			util.LogError("Attempt %d: Error deleting file %s (ID: %d): %v",
				attempt, itemPath, handle, err)

			// Wait before retry
			if attempt < 3 {
				time.Sleep(500 * time.Millisecond)
			}
		}

		// If normal deletion failed, try alternative method
		if !deleteSuccess {
			fmt.Println("  Trying alternative deletion method...")
			err = TryAlternativeDeleteMethod(dev, storageID, handle)
			if err != nil {
				util.LogError("Alternative deletion method failed for %s: %v", itemPath, err)
				failedItems++
				// Continue with other objects even if this one fails
			} else {
				deleteSuccess = true
				deletedFiles++
			}
		} else {
			deletedFiles++
		}
	}

	// Finally, delete the folder itself (if not root)
	if folderID != 0 { // Don't try to delete root folder
		fmt.Printf("Deleting folder itself: %s\n", folderPath)

		// Try multiple times to delete the folder
		deleteSuccess := false
		for attempt := 1; attempt <= 3; attempt++ {
			err = dev.DeleteObject(folderID)
			if err == nil {
				deleteSuccess = true
				break
			}

			util.LogError("Attempt %d: Error deleting folder %s (ID: %d): %v",
				attempt, folderPath, folderID, err)

			// Wait before retry
			if attempt < 3 {
				time.Sleep(500 * time.Millisecond)
			}
		}

		// If normal deletion failed, try alternative method
		if !deleteSuccess {
			fmt.Println("  Trying alternative deletion method for folder...")
			err = TryAlternativeDeleteMethod(dev, storageID, folderID)
			if err != nil {
				util.LogError("Alternative deletion method failed for folder %s: %v", folderPath, err)
				failedItems++
				// Don't return error here to allow showing summary
			} else {
				deletedFolders++
			}
		} else {
			deletedFolders++
		}
	}

	// Print summary
	fmt.Printf("\nDeletion complete for %s:\n", folderPath)
	fmt.Printf("- Deleted files: %d\n", deletedFiles)
	fmt.Printf("- Deleted folders: %d\n", deletedFolders)
	fmt.Printf("- Failed items: %d\n", failedItems)

	if failedItems > 0 {
		return fmt.Errorf("completed with %d failed deletions", failedItems)
	}

	return nil
}

func DeleteFolder(dev *mtp.Device, storagesRaw interface{}) {
	fmt.Println("\n=== Delete Folder and Contents ===")
	util.LogInfo("Starting folder deletion operation")

	// Get storage ID
	storageID, musicFolderID, err := SelectStorageAndMusicFolder(dev, storagesRaw)
	if err != nil {
		util.LogError("Error selecting storage: %v", err)
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Show deletion options
	fmt.Println("\nSelect a deletion option:")
	fmt.Println("1. Delete the entire Music/MUSIC folder and all contents")
	fmt.Println("2. Delete a specific subfolder")
	fmt.Println("0. Cancel")

	var optionChoice int
	fmt.Print("\nEnter option (0-2): ")
	fmt.Scanln(&optionChoice)

	if optionChoice == 0 {
		fmt.Println("Operation cancelled.")
		return
	}

	// Option to delete the entire Music folder
	if optionChoice == 1 {
		// Find both possible Music folder variations
		musicFolderPath := "/Music"
		musicFolderID := uint32(0)

		// First look for "Music" (capital M)
		musicFolderID, err = util.FindFolder(dev, storageID, PARENT_ROOT, "Music")
		if err != nil {
			// Try "MUSIC" (all caps)
			musicFolderID, err = util.FindFolder(dev, storageID, PARENT_ROOT, "MUSIC")
			if err != nil {
				util.LogError("Could not find Music folder: %v", err)
				fmt.Println("Error: Could not find Music folder")
				return
			}
			musicFolderPath = "/MUSIC"
		}

		fmt.Printf("\n!!! WARNING: About to delete the entire %s folder !!!\n", musicFolderPath)
		fmt.Println("This will erase ALL music, playlists, and folders on your device.")

		// Pass true to enable special confirmation for Music folder
		err = DeleteFolderRecursively(dev, storageID, musicFolderID, musicFolderPath, true)
		if err != nil {
			util.LogError("Error deleting Music folder: %v", err)
			fmt.Printf("Error: %v\n", err)
		} else {
			fmt.Println("Successfully deleted the entire Music folder.")
		}
		return
	}

	// Option to delete a specific subfolder
	// Show folder structure to help user select a folder
	fmt.Println("\nAvailable folders in Music directory:")

	// Get all objects in the Music folder
	handles := mtp.Uint32Array{}
	err = dev.GetObjectHandles(storageID, 0, musicFolderID, &handles)
	if err != nil {
		util.LogError("Error getting folders: %v", err)
		fmt.Printf("Error getting folders: %v\n", err)
		return
	}

	// Collect folders
	type FolderInfo struct {
		ID   uint32
		Name string
		Path string
	}

	var folders []FolderInfo

	// First find immediate subfolders of Music
	for _, handle := range handles.Values {
		info := mtp.ObjectInfo{}
		err := dev.GetObjectInfo(handle, &info)
		if err != nil {
			util.LogVerbose("Error getting object info for handle %d: %v", handle, err)
			continue
		}

		if info.ObjectFormat == FILETYPE_FOLDER {
			folders = append(folders, FolderInfo{
				ID:   handle,
				Name: info.Filename,
				Path: "/Music/" + info.Filename,
			})
			fmt.Printf("%d. %s\n", len(folders), info.Filename)
		}
	}

	if len(folders) == 0 {
		fmt.Println("No folders found in Music directory.")
		return
	}

	// Let user select a folder
	fmt.Print("\nSelect a folder to delete (1-" + fmt.Sprint(len(folders)) + "), or 0 to cancel: ")
	var selection int
	fmt.Scanln(&selection)

	if selection <= 0 || selection > len(folders) {
		fmt.Println("Operation cancelled.")
		return
	}

	selectedFolder := folders[selection-1]

	// Confirm deletion
	fmt.Printf("\n!!! WARNING !!!\n")
	fmt.Printf("You are about to delete the folder '%s' AND ALL ITS CONTENTS.\n", selectedFolder.Path)
	fmt.Printf("This operation CANNOT BE UNDONE and will delete ALL files and subfolders.\n")
	fmt.Print("Type 'DELETE ALL' (all caps) to confirm this operation: ")

	var confirmText string
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	confirmText = scanner.Text()

	if confirmText != "DELETE ALL" {
		fmt.Println("Operation cancelled.")
		return
	}

	// Perform the deletion
	err = DeleteFolderRecursively(dev, storageID, selectedFolder.ID, selectedFolder.Path, false) // No need for another confirmation
	if err != nil {
		util.LogError("Error during recursive deletion: %v", err)
		fmt.Printf("Error: %v\n", err)
		fmt.Println("Some items may have been deleted successfully.")
	} else {
		fmt.Printf("Successfully deleted folder '%s' and all its contents.\n", selectedFolder.Path)
	}
}
