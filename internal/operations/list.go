package operations

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/ganeshrvel/go-mtpfs/mtp"
	"github.com/ganeshrvel/go-mtpx"
	"github.com/schachte/better-sync/internal/util"
)

type PlaylistInfo struct {
	Name      string
	Path      string
	ObjectID  uint32
	StorageID uint32
	Storage   string
}

func ShowSongs(dev *mtp.Device, storagesRaw interface{}) {
	fmt.Println("\n=== Songs ===")
	util.LogInfo("Searching for MP3 files...")

	storagesValue := reflect.ValueOf(storagesRaw)
	if storagesValue.Kind() != reflect.Slice {
		util.LogError("Error: Storages is not a slice")
		return
	}

	totalMP3Count := 0
	for i := 0; i < storagesValue.Len(); i++ {
		storageObj := storagesValue.Index(i).Interface()

		sid := extractUint32Field(storageObj, "Sid")
		description := extractStringField(storageObj, "StorageDescription")
		if description == "" {
			description = extractStringField(storageObj, "Description")
		}

		fmt.Printf("\nStorage #%d: %s (ID: %d)\n", i+1, description, sid)
		util.LogInfo("Searching storage #%d: %s (ID: %d)", i+1, description, sid)

		mp3Files, err := FindMP3Files(dev, sid)
		if err != nil {
			util.LogError("Error searching storage: %v", err)
			continue
		}

		storageMP3Count := len(mp3Files)
		fmt.Printf("Found %d MP3 files in storage #%d\n", storageMP3Count, i+1)
		util.LogInfo("Found %d MP3 files in storage #%d", storageMP3Count, i+1)

		for j, file := range mp3Files {
			fmt.Printf("  %d. %s\n", j+1, file)
		}

		totalMP3Count += storageMP3Count
	}

	fmt.Printf("\nTotal MP3 files found across all storages: %d\n", totalMP3Count)
	util.LogInfo("Total MP3 files found across all storages: %d", totalMP3Count)
}

func ShowPlaylistsAndSongs(dev *mtp.Device, storagesRaw interface{}) {
	// Use reflection to handle the storages
	storagesValue := reflect.ValueOf(storagesRaw)
	if storagesValue.Kind() != reflect.Slice || storagesValue.Len() == 0 {
		fmt.Println("Error: No storage available on the device")
		return
	}

	fmt.Println("\n==== Playlists and Songs on Device ====")
	totalPlaylists := 0

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
			continue
		}

		if len(playlists) > 0 {
			fmt.Printf("\nStorage: %s (ID: %d)\n", storageDesc, storageID)

			for i, playlistPath := range playlists {
				fmt.Printf("\n%d. %s\n    Path: %s\n", i+1, filepath.Base(playlistPath), playlistPath)
				totalPlaylists++

				// Find the object ID of the playlist
				objectID, err := FindObjectByPath(dev, storageID, playlistPath)
				if err != nil {
					util.LogError("Error finding playlist object ID: %v", err)
					continue
				}

				// Read the playlist content
				songs, err := ReadPlaylistContent(dev, storageID, objectID)
				if err != nil {
					util.LogError("Error reading playlist content: %v", err)
					fmt.Println("   Error reading playlist content")
					continue
				}

				if len(songs) == 0 {
					fmt.Println("   (Empty playlist)")
				} else {
					for j, song := range songs {
						fmt.Printf("   %d.%d. %s\n       Path: %s\n", i+1, j+1, filepath.Base(song), song)

						// Only show first 10 songs if there are too many
						if j >= 9 && len(songs) > 10 {
							fmt.Printf("   ...and %d more songs\n", len(songs)-10)
							break
						}
					}
				}
			}
		}
	}

	if totalPlaylists == 0 {
		fmt.Println("No playlists found on the device")
	} else {
		fmt.Printf("\nTotal playlists found: %d\n", totalPlaylists)
	}
}

func FindOrCreateMusicFolder(dev *mtp.Device, storageID uint32) (uint32, error) {
	// Try to find existing Music folder at root level
	folderID, err := util.FindFolder(dev, storageID, PARENT_ROOT, "Music")
	if err != nil {
		util.LogError("Error finding Music folder: %v", err)
		// If not found, create it
		if err.Error() == "folder not found" {
			util.LogInfo("Music folder not found, creating it")
			folderID, err = util.CreateFolder(dev, storageID, PARENT_ROOT, "Music")
			if err != nil {
				return 0, fmt.Errorf("error creating Music folder: %v", err)
			}
			util.LogInfo("Created Music folder with ID: %d", folderID)
			return folderID, nil
		}
		return 0, err
	}

	util.LogInfo("Found existing Music folder with ID: %d", folderID)
	return folderID, nil
}

func FindObjectByPath(dev *mtp.Device, storageID uint32, path string) (uint32, error) {
	// Normalize path
	path = strings.TrimSpace(path)

	// Remove "0:" prefix if present
	path = strings.TrimPrefix(path, "0:")

	// Ensure path starts with a slash
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// Normalize music directory to "/Music"
	path = strings.Replace(path, "/MUSIC/", "/Music/", 1)
	path = strings.Replace(path, "/music/", "/Music/", 1)

	util.LogVerbose("Normalized path for lookup: %s", path)

	// First try to find the object by manually traversing the path
	objectID, err := FindObjectByPathManual(dev, storageID, path)
	if err == nil {
		util.LogVerbose("Found object by path: %s (ID: %d)", path, objectID)
		return objectID, nil
	}

	util.LogVerbose("Manual path traversal failed: %v, trying alternative methods", err)

	// If that fails, try to find by matching filename among all MP3 files
	mp3Files, err := FindMP3Files(dev, storageID)
	if err != nil {
		return 0, fmt.Errorf("error searching for MP3 files: %v", err)
	}

	// Get the filename from the path
	filename := filepath.Base(path)

	// Extract the base name without the number prefix (if any)
	// For example, "03 ALPHA.MP3" -> "ALPHA.MP3"
	baseNameWithoutNumber := filename
	parts := strings.SplitN(filename, " ", 2)
	if len(parts) == 2 && isNumeric(parts[0]) {
		baseNameWithoutNumber = parts[1]
	}

	util.LogVerbose("Looking for file with base name: %s or %s", filename, baseNameWithoutNumber)

	// Look for exact filename match or match without number prefix
	for _, mp3Path := range mp3Files {
		mp3Filename := filepath.Base(mp3Path)

		if mp3Filename == filename ||
			mp3Filename == baseNameWithoutNumber ||
			strings.EqualFold(mp3Filename, filename) ||
			strings.EqualFold(mp3Filename, baseNameWithoutNumber) {
			util.LogVerbose("Found file with matching name: %s", mp3Path)
			// Since we already have the path from findMP3Files, we can try to traverse to it
			objectID, err := FindObjectByPathManual(dev, storageID, mp3Path)
			if err == nil && objectID != 0 {
				return objectID, nil
			}
		}
	}

	// Try a more flexible approach - look for files with similar paths
	for _, mp3Path := range mp3Files {
		// Check if the last parts of the path match (ignoring case)
		pathParts := strings.Split(strings.ToLower(path), "/")
		mp3Parts := strings.Split(strings.ToLower(mp3Path), "/")

		// If we have at least 3 parts (e.g., /Music/Artist/Song.mp3)
		if len(pathParts) >= 3 && len(mp3Parts) >= 3 {
			// Check if the artist folder matches
			if pathParts[len(pathParts)-2] == mp3Parts[len(mp3Parts)-2] {
				// Now check if the filename matches or contains the same base name
				pathFilename := pathParts[len(pathParts)-1]
				mp3Filename := mp3Parts[len(mp3Parts)-1]

				// Extract base names without extensions and numbers
				pathBaseName := extractBaseName(pathFilename)
				mp3BaseName := extractBaseName(mp3Filename)

				if pathBaseName == mp3BaseName ||
					strings.Contains(mp3BaseName, pathBaseName) ||
					strings.Contains(pathBaseName, mp3BaseName) {
					util.LogVerbose("Found similar file: %s for requested file: %s", mp3Path, path)
					objectID, err := FindObjectByPathManual(dev, storageID, mp3Path)
					if err == nil && objectID != 0 {
						return objectID, nil
					}
				}
			}
		}
	}

	return 0, fmt.Errorf("could not find object with path: %s", path)
}

func isNumeric(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

func extractBaseName(filename string) string {
	// Remove extension
	baseName := strings.TrimSuffix(filename, filepath.Ext(filename))

	// Remove leading numbers and spaces
	parts := strings.SplitN(baseName, " ", 2)
	if len(parts) == 2 && isNumeric(parts[0]) {
		baseName = parts[1]
	}

	// Convert to uppercase for comparison
	return strings.ToUpper(baseName)
}

func FindObjectByPathManual(dev *mtp.Device, storageID uint32, path string) (uint32, error) {
	// Normalize the path: remove leading/trailing whitespace and ensure it starts with /
	path = strings.TrimSpace(path)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// Generate path variations to try
	variations := GeneratePathVariations(path)

	// Try each variation
	for _, pathVar := range variations {
		util.LogVerbose("Trying path variation: %s", pathVar)

		// Split the path into components
		components := strings.Split(pathVar, "/")
		// Filter out empty components
		var parts []string
		for _, comp := range components {
			if comp != "" {
				parts = append(parts, comp)
			}
		}

		// Start at the root
		currentID := PARENT_ROOT

		// Traverse the path component by component
		for i, component := range parts {
			// Get all objects in the current folder
			handles := mtp.Uint32Array{}
			err := dev.GetObjectHandles(storageID, 0, currentID, &handles)
			if err != nil {
				util.LogVerbose("Error getting objects in folder (ID: %d): %v", currentID, err)
				break // Try next variation
			}

			// Look for the current component
			found := false
			for _, handle := range handles.Values {
				info := mtp.ObjectInfo{}
				err = dev.GetObjectInfo(handle, &info)
				if err != nil {
					util.LogVerbose("Error getting object info for handle %d: %v", handle, err)
					continue
				}

				// Check if the filename matches the current path component
				// Try case-sensitive and case-insensitive matching
				if info.Filename == component || strings.EqualFold(info.Filename, component) {
					// If this is the last component, we found our object
					if i == len(parts)-1 {
						util.LogVerbose("Found object: %s (ID: %d)", component, handle)
						return handle, nil
					}

					// Otherwise, continue traversing
					currentID = handle
					found = true
					break
				}
			}

			if !found {
				// This component wasn't found, try next variation
				break
			}
		}
	}

	return 0, fmt.Errorf("could not find object with path: %s", path)
}

func GeneratePathVariations(path string) []string {
	// Normalize path
	path = strings.TrimSpace(path)

	// 1. With and without leading slash
	pathVariations := []string{
		path,
		"/" + strings.TrimPrefix(path, "/"),
		strings.TrimPrefix(path, "/"),
	}

	// 2. Try with common path prefixes
	withoutPrefix := strings.TrimPrefix(path, "/")
	pathVariations = append(pathVariations,
		"0:"+path,
		"0:/"+withoutPrefix,
	)

	// 3. Try with uppercase and lowercase variations
	var caseVariations []string
	for _, p := range pathVariations {
		caseVariations = append(caseVariations,
			strings.ToUpper(p),
			strings.ToLower(p),
		)
	}
	pathVariations = append(pathVariations, caseVariations...)

	return pathVariations
}

func GetFolderIDByPath(dev *mtp.Device, storageID uint32, path string) (uint32, error) {
	// Remove leading slash if present
	if strings.HasPrefix(path, "/") {
		path = path[1:]
	}

	// If empty path, return root
	if path == "" {
		return PARENT_ROOT, nil
	}

	// Split path into components
	components := strings.Split(path, "/")
	currentFolderID := PARENT_ROOT

	for _, component := range components {
		if component == "" {
			continue
		}

		// Find the folder with this name in the current folder
		folderID, err := util.FindFolder(dev, storageID, currentFolderID, component)
		if err != nil {
			return 0, fmt.Errorf("folder '%s' not found in path '%s': %v",
				component, path, err)
		}

		// Move to this folder
		currentFolderID = folderID
	}

	return currentFolderID, nil
}

func FindPlaylistsInFolder(dev *mtp.Device, storageID, folderID uint32, folderPath string) ([]string, error) {
	var playlists []string

	// Get all object handles in this folder
	handles, err := util.GetObjectHandlesWithRetry(dev, storageID, 0, folderID)
	if err != nil {
		return nil, fmt.Errorf("error getting object handles: %v", err)
	}

	// Check each object
	for _, handle := range handles.Values {
		// Get object info
		info, err := util.GetObjectInfoWithRetry(dev, handle)
		if err != nil {
			util.LogVerbose("Error getting object info for handle %d: %v", handle, err)
			continue
		}

		// Check if it's a playlist file
		ext := strings.ToLower(filepath.Ext(info.Filename))
		if ext == ".m3u8" || ext == ".m3u" || ext == ".pls" {
			// It's a playlist
			path := filepath.Join(folderPath, info.Filename)
			playlists = append(playlists, path)

			util.LogVerbose("Found playlist via direct search: %s (ID: %d, Format: %d)",
				path, handle, info.ObjectFormat)
		} else if info.ObjectFormat == FILETYPE_FOLDER {
			// It's a folder, recurse into it
			subfolderPath := filepath.Join(folderPath, info.Filename)
			subPlaylists, err := FindPlaylistsInFolder(dev, storageID, handle, subfolderPath)
			if err != nil {
				util.LogVerbose("Error searching subfolder %s: %v", subfolderPath, err)
			} else {
				playlists = append(playlists, subPlaylists...)
			}
		}
	}

	return playlists, nil
}

func FindPlaylists(dev *mtp.Device, storageID uint32) ([]string, error) {
	var playlists []string
	util.LogInfo("Searching for playlists in both /MUSIC and /Music directories")

	// Try both uppercase and lowercase Music folder paths
	musicFolderPaths := []string{"/Music"}

	for _, musicPath := range musicFolderPaths {
		// Try to find the Music folder
		musicFolderID, err := GetFolderIDByPath(dev, storageID, musicPath)
		if err != nil {
			util.LogInfo("%s folder not found: %v", musicPath, err)
			// Continue to the next path if this one doesn't exist
			continue
		}

		// Get all object handles in the Music folder (non-recursive)
		handles := mtp.Uint32Array{}
		err = dev.GetObjectHandles(storageID, 0, musicFolderID, &handles)
		if err != nil {
			util.LogError("Error getting object handles in %s folder: %v", musicPath, err)
			continue
		}

		// Check each object in the Music folder
		for _, handle := range handles.Values {
			// Get object info
			info := mtp.ObjectInfo{}
			err = dev.GetObjectInfo(handle, &info)
			if err != nil {
				util.LogVerbose("Error getting object info for handle %d: %v", handle, err)
				continue
			}

			// Check if it's a playlist file
			ext := strings.ToLower(filepath.Ext(info.Filename))
			if ext == ".m3u8" || ext == ".m3u" || ext == ".pls" {
				// It's a playlist
				path := musicPath + "/" + info.Filename
				playlists = append(playlists, path)

				util.LogVerbose("Found playlist: %s (ID: %d, Format: %d)",
					path, handle, info.ObjectFormat)
			}
		}

		util.LogInfo("Playlists found in %s: %d", musicPath, len(playlists))
	}

	util.LogInfo("Total playlists found: %d", len(playlists))
	return playlists, nil
}

func EnhancedDeletePlaylistAndAllSongs(dev *mtp.Device, storagesRaw interface{}, playlistName string) error {
	fmt.Println("\n=== Delete Playlist ===")
	util.LogVerbose("Starting playlist deletion operation for %s", playlistName)

	// Get all playlists using the new function that returns PlaylistInfo
	playlists := ShowPlaylists(dev, storagesRaw)

	// Find the playlist that matches the given name
	var targetPlaylist *PlaylistInfo
	for i, playlist := range playlists {
		util.LogVerbose("Playlist: %s", playlist.Name)
		// Check if the playlist name matches (case insensitive)
		if strings.EqualFold(playlist.Name, playlistName) ||
			strings.EqualFold(filepath.Base(playlist.Path), playlistName) {
			targetPlaylist = &playlists[i]
			break
		}
	}

	if targetPlaylist == nil {
		util.LogError("Playlist '%s' not found", playlistName)
		return fmt.Errorf("playlist '%s' not found", playlistName)
	}

	// Read the playlist content to get all songs
	songs, err := ReadPlaylistContent(dev, targetPlaylist.StorageID, targetPlaylist.ObjectID)
	if err != nil {
		return fmt.Errorf("error reading playlist content: %w", err)
	}

	fmt.Printf("Playlist contains %d songs\n", len(songs))
	util.LogInfo("Playlist contains %d songs", len(songs))

	// Delete all songs in the playlist
	deletedSongs := 0
	for _, songPath := range songs {
		// Normalize the song path:
		// 1. Remove "0:" prefix if present
		normalizedPath := strings.TrimPrefix(songPath, "0:")

		// 2. Ensure path starts with a slash
		if !strings.HasPrefix(normalizedPath, "/") {
			normalizedPath = "/" + normalizedPath
		}

		// 3. Normalize music directory to "/Music"
		normalizedPath = strings.Replace(normalizedPath, "/MUSIC/", "/Music/", 1)
		normalizedPath = strings.Replace(normalizedPath, "/music/", "/Music/", 1)

		util.LogVerbose("Original song path: %s", songPath)
		util.LogVerbose("Normalized song path: %s", normalizedPath)

		util.LogVerbose("Deleting song: %s", normalizedPath)
		songObjectID, err := FindObjectByPath(dev, targetPlaylist.StorageID, normalizedPath)
		if err != nil {
			util.LogError("Could not find song '%s': %v", normalizedPath, err)
			fmt.Printf("Could not find song: %s\n", normalizedPath)
			continue
		}

		err = dev.DeleteObject(songObjectID)
		if err != nil {
			util.LogError("Failed to delete song '%s' (ID: %d): %v", normalizedPath, songObjectID, err)
			fmt.Printf("Failed to delete song: %s\n", normalizedPath)
		} else {
			util.LogInfo("Deleted song: %s (ID: %d)", normalizedPath, songObjectID)
			fmt.Printf("Deleted song: %s\n", normalizedPath)
			deletedSongs++
		}
	}

	// Delete the playlist itself
	err = dev.DeleteObject(targetPlaylist.ObjectID)
	if err != nil {
		return fmt.Errorf("failed to delete playlist '%s' (ID: %d): %w",
			targetPlaylist.Path, targetPlaylist.ObjectID, err)
	}

	util.LogInfo("Deleted playlist: %s (ID: %d)", targetPlaylist.Path, targetPlaylist.ObjectID)
	fmt.Printf("Successfully deleted playlist '%s' and %d/%d songs\n",
		targetPlaylist.Name, deletedSongs, len(songs))

	return nil
}

func FindMP3Files(dev *mtp.Device, storageID uint32) ([]string, error) {
	var mp3Files []string
	var emptyFiles []string

	util.LogInfo("Searching for MP3 files in both case variations of music directories")

	// Walk through all objects in the storage
	// Try both /Music and /MUSIC paths
	musicPaths := []string{"/Music"}
	var totalFiles int64 // Changed to int64 to match mtpx.Walk count type
	var foundAnyFiles bool = false

	for _, basePath := range musicPaths {
		util.LogInfo("Searching in %s directory", basePath)
		_, count, _, walkErr := mtpx.Walk(dev, storageID, basePath, true, true, false,
			func(objectID uint32, fi *mtpx.FileInfo, err error) error {
				if err != nil {
					return err
				}

				// Check if this is a file (not a directory) with .mp3 extension (case-insensitive)
				if !fi.IsDir && strings.ToLower(filepath.Ext(fi.FullPath)) == ".mp3" {
					// Check if this is an empty file (failed upload)
					if fi.Size == 0 {
						emptyPath := strings.ToUpper(fi.FullPath)
						emptyFiles = append(emptyFiles, emptyPath)
						util.LogError("Found EMPTY MP3: %s (Size: 0 bytes, ID: %d)", fi.FullPath, objectID)
					} else {
						// Store path in uppercase to match playlist format
						mp3Files = append(mp3Files, strings.ToUpper(fi.FullPath))
						util.LogVerbose("Found MP3: %s (Size: %d bytes, ID: %d)", fi.FullPath, fi.Size, objectID)
						foundAnyFiles = true
					}
				}

				return nil
			})

		if walkErr != nil {
			// Log the error but don't abort - continue to the next music folder
			util.LogError("Error walking path %s: %v", basePath, walkErr)
		} else {
			totalFiles += count
			util.LogInfo("Found %d total objects in %s directory", count, basePath)
		}
	}

	// If we found empty MP3 files but no valid ones, report this specially
	if len(emptyFiles) > 0 && len(mp3Files) == 0 {
		util.LogError("Found %d EMPTY MP3 files (failed uploads) but no valid MP3 files", len(emptyFiles))
		util.LogInfo("HINT: Your files were uploaded but the data transfer failed.")
		util.LogInfo("Try re-uploading with the updated file transfer system.")

		// Help diagnose by returning the empty files too
		return emptyFiles, fmt.Errorf("found %d EMPTY MP3 files but no valid ones", len(emptyFiles))
	}

	if !foundAnyFiles {
		util.LogError("No MP3 files found in any music directories")
		util.LogInfo("HINT: If you've uploaded files but can't find them, check:")
		util.LogInfo("1. The files were uploaded successfully (no errors during upload)")
		util.LogInfo("2. The case of the music directory: the app now searches both /MUSIC and /Music")
		util.LogInfo("3. The directory structure on the device (using a direct file browser)")
	} else {
		util.LogInfo("Found %d valid MP3 files and %d empty MP3 files across all music directories",
			len(mp3Files), len(emptyFiles))
	}

	return mp3Files, nil
}

func ShowPlaylists(dev *mtp.Device, storagesRaw interface{}) []PlaylistInfo {
	fmt.Println("\n=== Playlists ===")
	util.LogInfo("Searching for playlists...")

	storagesValue := reflect.ValueOf(storagesRaw)
	if storagesValue.Kind() != reflect.Slice {
		util.LogError("Error: Storages is not a slice")
		return []PlaylistInfo{}
	}

	var totalPlaylists []PlaylistInfo
	for i := 0; i < storagesValue.Len(); i++ {
		storageObj := storagesValue.Index(i).Interface()

		sid := extractUint32Field(storageObj, "Sid")
		description := extractStringField(storageObj, "StorageDescription")
		if description == "" {
			description = extractStringField(storageObj, "Description")
		}

		fmt.Printf("\nStorage #%d: %s (ID: %d)\n", i+1, description, sid)
		util.LogInfo("Searching storage #%d: %s (ID: %d)", i+1, description, sid)

		playlists, err := FindPlaylists(dev, sid)
		if err != nil {
			util.LogError("Error searching storage: %v", err)
			continue
		}

		storagePlaylistCount := len(playlists)
		fmt.Printf("Found %d playlists in storage #%d\n", storagePlaylistCount, i+1)
		util.LogInfo("Found %d playlists in storage #%d", storagePlaylistCount, i+1)

		for j, playlistPath := range playlists {
			fmt.Printf("  %d. %s\n", j+1, playlistPath)

			// Get playlist object ID
			objectID, err := FindObjectByPath(dev, sid, playlistPath)
			if err != nil {
				util.LogError("Error finding playlist object ID: %v", err)
				continue
			}

			totalPlaylists = append(totalPlaylists, PlaylistInfo{
				Name:      filepath.Base(playlistPath),
				Path:      playlistPath,
				ObjectID:  objectID,
				StorageID: sid,
				Storage:   description,
			})
		}
	}

	fmt.Printf("\nTotal playlists found across all storages: %d\n", len(totalPlaylists))
	util.LogInfo("Total playlists found across all storages: %d", len(totalPlaylists))

	return totalPlaylists
}
