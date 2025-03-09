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
	"github.com/schachte/better-sync/pkg/model"
	"github.com/schachte/better-sync/pkg/util"
)

func DeletePlaylist(dev *mtp.Device, storagesRaw interface{}) {
	util.LogInfo("=== Delete Playlist ===")
	util.LogVerbose("Starting playlist deletion operation")

	storagesValue := reflect.ValueOf(storagesRaw)
	if storagesValue.Kind() != reflect.Slice {
		util.LogError("Error: Storages is not a slice")
		return
	}

	allPlaylists := make([]model.PlaylistEntry, 0)

	util.LogInfo("Scanning for playlists...")

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

	fmt.Println("\n==== Available Playlists ====")
	for i, playlist := range allPlaylists {
		fmt.Printf("%d. [%s] %s\n", i+1, playlist.StorageDesc, filepath.Base(playlist.Path))
	}

	fmt.Print("\nSelect playlist to delete (1-" + fmt.Sprint(len(allPlaylists)) + "): ")
	var selection int
	fmt.Scanln(&selection)

	if selection < 1 || selection > len(allPlaylists) {
		fmt.Println("Invalid selection")
		return
	}

	selected := allPlaylists[selection-1]

	fmt.Printf("Are you sure you want to delete the playlist '%s'? (y/n): ", filepath.Base(selected.Path))
	var confirm string
	fmt.Scanln(&confirm)

	if strings.ToLower(confirm) != "y" {
		fmt.Println("Operation cancelled")
		return
	}

	err := dev.DeleteObject(selected.ObjectID)
	if err != nil {
		util.LogError("Error deleting playlist: %v", err)
		fmt.Printf("Error deleting playlist: %v\n", err)

		fmt.Println("Trying alternative deletion method...")
		err = tryAlternativeDeleteMethod(dev, selected.StorageID, selected.ObjectID)
		if err != nil {
			fmt.Printf("Alternative deletion failed: %v\n", err)
			return
		}
	}

	util.LogInfo("Playlist deleted successfully")
}

func DeleteSong(dev *mtp.Device, storagesRaw interface{}) {
	util.LogInfo("Starting song deletion operation")

	storagesValue := reflect.ValueOf(storagesRaw)
	if storagesValue.Kind() != reflect.Slice {
		util.LogError("Error: Storages is not a slice")
		return
	}

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

		util.LogInfo("Searching storage #%d: %s (ID: %d) for songs...", i+1, description, storageID)

		songPaths, err := FindMP3Files(dev, storageID)
		if err != nil {
			util.LogError("Error finding songs in storage #%d: %v", i+1, err)
			continue
		}

		for _, path := range songPaths {

			filename := filepath.Base(path)

			parentPath := filepath.Dir(path)
			if parentPath == "." || parentPath == "/" {
				parentPath = ""
			}

			parentID := PARENT_ROOT
			if parentPath != "" {
				parentID, err = GetFolderIDByPath(dev, storageID, parentPath)
				if err != nil {
					util.LogVerbose("Could not find parent folder for %s: %v", path, err)

				}
			}

			handles := mtp.Uint32Array{}
			err = dev.GetObjectHandles(storageID, 0, parentID, &handles)
			if err != nil {
				util.LogVerbose("Error getting objects in folder %s: %v", parentPath, err)
				continue
			}

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

	fmt.Println("\nAvailable songs to delete:")
	for i, song := range allSongs {
		fmt.Printf("%d. %s (%s)\n", i+1, song.DisplayName, song.Path)
	}

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

	selectedSong := allSongs[index-1]

	fmt.Printf("\nAre you sure you want to delete the song '%s'? (y/n): ", selectedSong.DisplayName)
	scanner.Scan()
	confirm := strings.ToLower(scanner.Text())
	if confirm != "y" && confirm != "yes" {
		fmt.Println("Deletion cancelled.")
		return
	}

	fmt.Println("\nWARNING: If this song is included in any playlists, those playlists may be affected.")
	fmt.Print("Continue with deletion? (y/n): ")
	scanner.Scan()
	confirmAgain := strings.ToLower(scanner.Text())
	if confirmAgain != "y" && confirmAgain != "yes" {
		fmt.Println("Deletion cancelled.")
		return
	}

	util.LogInfo("Deleting song '%s'...", selectedSong.DisplayName)
	util.LogInfo("Attempting to delete song %s (Path: %s, ObjectID: %d, StorageID: %d)",
		selectedSong.DisplayName, selectedSong.Path, selectedSong.ObjectID, selectedSong.StorageID)

	err := dev.DeleteObject(selectedSong.ObjectID)
	if err != nil {
		util.LogError("Error deleting song: %v", err)
		fmt.Printf("Error deleting song: %v\n", err)

		fmt.Print("\nWould you like to try an alternative deletion method? (y/n): ")
		scanner.Scan()
		retry := strings.ToLower(scanner.Text())
		if retry == "y" || retry == "yes" {

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
				util.LogInfo("Successfully deleted song '%s' using alternative method", selectedSong.DisplayName)
			}
		} else {
			return
		}
	} else {
		util.LogInfo("Successfully deleted song %s", selectedSong.DisplayName)
	}

	fmt.Println("\nNote: If the song was part of any playlists, you may need to update those playlists manually.")
}

func tryAlternativeDeleteMethod(dev *mtp.Device, storageID, objectID uint32) error {
	util.LogInfo("Trying alternative deletion method for object ID %d", objectID)

	info := mtp.ObjectInfo{}
	err := dev.GetObjectInfo(objectID, &info)
	if err != nil {
		return fmt.Errorf("failed to get object info: %v", err)
	}

	info.CompressedSize = 0

	_, _, _, err = dev.SendObjectInfo(storageID, info.ParentObject, &info)
	if err != nil {
		return fmt.Errorf("failed to send modified object info: %v", err)
	}

	err = dev.DeleteObject(objectID)
	if err != nil {
		return fmt.Errorf("failed to delete modified object: %v", err)
	}

	return nil
}

func FindObjectByDirectPath(dev *mtp.Device, storageID uint32, path string) (uint32, error) {
	normalizedPath := normalizePath(path)

	var foundObject uint32
	var found bool

	_, _, _, _ = mtpx.Walk(dev, storageID, "/", true, true, false,
		func(objectID uint32, fi *mtpx.FileInfo, err error) error {
			if err != nil {
				return nil
			}

			if normalizePath(fi.FullPath) == normalizedPath {
				foundObject = objectID
				found = true
				return fmt.Errorf("found")
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

	fileName := filepath.Base(path)

	fileNameNoNumber := stripNumericPrefix(fileName)

	folderPath := filepath.Dir(path)
	pathComponents := strings.Split(folderPath, "/")

	var artistFolder, albumFolder string
	if len(pathComponents) >= 3 {
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

	_, _, _, _ = mtpx.Walk(dev, storageID, "/", true, true, false,
		func(objectID uint32, fi *mtpx.FileInfo, err error) error {
			if err != nil {
				return nil
			}

			if fi.IsDir || !strings.HasSuffix(strings.ToLower(fi.FullPath), ".mp3") {
				return nil
			}

			itemFileName := filepath.Base(fi.FullPath)
			if strings.EqualFold(itemFileName, fileName) {
				foundObject = objectID
				found = true
				matchReason = "exact filename match"
				return fmt.Errorf("found")
			}

			itemFileNameNoNumber := stripNumericPrefix(itemFileName)
			if fileNameNoNumber != "" && strings.EqualFold(itemFileNameNoNumber, fileNameNoNumber) {
				foundObject = objectID
				found = true
				matchReason = "filename match without numeric prefix"
				return fmt.Errorf("found")
			}

			if artistFolder != "" && albumFolder != "" {
				itemPath := fi.FullPath
				if strings.Contains(strings.ToUpper(itemPath), strings.ToUpper(artistFolder)) &&
					strings.Contains(strings.ToUpper(itemPath), strings.ToUpper(albumFolder)) &&
					strings.Contains(strings.ToUpper(itemFileName), strings.ToUpper(fileNameNoNumber)) {
					foundObject = objectID
					found = true
					matchReason = "artist/album/name pattern match"
					return fmt.Errorf("found")
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

	baseName := strings.TrimSuffix(filename, filepath.Ext(filename))

	if len(baseName) < 3 {
		return baseName
	}

	if baseName[0] >= '0' && baseName[0] <= '9' && baseName[1] >= '0' && baseName[1] <= '9' {

		for i := 2; i < len(baseName); i++ {
			if baseName[i] >= 'A' && baseName[i] <= 'Z' ||
				baseName[i] >= 'a' && baseName[i] <= 'z' {
				return baseName[i:]
			}

			if baseName[i] != ' ' && baseName[i] != '-' && baseName[i] != '_' {
				break
			}
		}
	}

	return baseName
}

func normalizePath(path string) string {

	path = strings.TrimPrefix(path, "0:")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	for strings.Contains(path, "//") {
		path = strings.Replace(path, "//", "/", -1)
	}

	return path
}

func ExtractPlaylistSongPaths(dev *mtp.Device, storageID, objectID uint32, playlistPath string) ([]string, error) {

	songs, err := ReadPlaylistContent(dev, storageID, objectID)
	if err != nil {
		return nil, err
	}

	return songs, nil
}

func ReadPlaylistContent(dev *mtp.Device, storageID, objectID uint32) ([]string, error) {

	var buf bytes.Buffer

	err := dev.GetObject(objectID, &buf, model.EmptyProgressFunc)
	if err != nil {
		return nil, fmt.Errorf("error reading playlist: %v", err)
	}

	content := buf.String()
	return ParsePlaylistContent(content), nil
}

func ParsePlaylistContent(content string) []string {
	var songs []string

	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		songPath := line

		songs = append(songs, songPath)
	}

	return songs
}

func DeletePlaylistOnly(dev *mtp.Device, playlistObjectID uint32) {
	util.LogInfo("Deleting only the playlist...")
	err := dev.DeleteObject(playlistObjectID)
	if err != nil {
		util.LogError("Error deleting playlist (ID: %d): %v", playlistObjectID, err)
		fmt.Printf("ERROR: Could not delete playlist: %v\n", err)

		util.LogInfo("Trying alternative deletion method...")
		err = tryAlternativeDeleteMethod(dev, 0, playlistObjectID)
		if err != nil {
			util.LogError("Alternative deletion method failed: %v", err)
			fmt.Printf("Alternative deletion method failed: %v\n", err)
			fmt.Println("Playlist could not be deleted.")
		} else {
			util.LogInfo("Successfully deleted playlist using alternative method.")
		}
	} else {
		util.LogInfo("Successfully deleted playlist.")
	}
}

func TryAlternativeDeleteMethod(dev *mtp.Device, storageID, objectID uint32) error {
	util.LogInfo("Trying alternative deletion method for object ID %d", objectID)

	info := mtp.ObjectInfo{}
	err := dev.GetObjectInfo(objectID, &info)
	if err != nil {
		return fmt.Errorf("failed to get object info: %v", err)
	}

	info.CompressedSize = 0

	_, _, _, err = dev.SendObjectInfo(storageID, info.ParentObject, &info)
	if err != nil {
		return fmt.Errorf("failed to send modified object info: %v", err)
	}

	err = dev.DeleteObject(objectID)
	if err != nil {
		return fmt.Errorf("failed to delete modified object: %v", err)
	}

	return nil
}

func DeleteFolderRecursively(dev *mtp.Device, storageID, folderID uint32, folderPath string, requireConfirmation bool) error {
	if folderPath == "/" {
		return fmt.Errorf("refusing to delete root folder")
	}

	lowerPath := strings.ToLower(folderPath)
	if (lowerPath == "/music" || lowerPath == "/music/") && requireConfirmation {
		return fmt.Errorf("main Music folder deletion cancelled due to safety restriction")
	}

	util.LogInfo("Starting recursive deletion of folder contents: %s (ID: %d)", folderPath, folderID)
	util.LogInfo("Deleting contents of folder: %s", folderPath)

	handles := mtp.Uint32Array{}
	err := dev.GetObjectHandles(storageID, 0, folderID, &handles)
	if err != nil {
		return fmt.Errorf("error getting object handles: %w", err)
	}

	util.LogInfo("Found %d items in folder %s", len(handles.Values), folderPath)
	util.LogInfo("Found %d items to process", len(handles.Values))

	deletedFiles := 0
	deletedFolders := 0
	failedItems := 0

	for i, handle := range handles.Values {
		util.LogVerbose("[%d/%d] Processing item", i+1, len(handles.Values))

		info := mtp.ObjectInfo{}
		err := dev.GetObjectInfo(handle, &info)
		if err != nil {
			util.LogError("Error getting object info for handle %d: %v", handle, err)
			failedItems++
			continue
		}

		itemPath := filepath.Join(folderPath, info.Filename)

		if info.ObjectFormat == FILETYPE_FOLDER {
			util.LogInfo("Processing subfolder: %s", info.Filename)
			subErr := DeleteFolderRecursively(dev, storageID, handle, itemPath, false)
			if subErr != nil {
				util.LogError("Error deleting subfolder %s: %v", itemPath, subErr)
				failedItems++
			} else {
				deletedFolders++
			}
			continue
		}

		util.LogInfo("Deleting file: %s", info.Filename)

		deleteSuccess := false
		for attempt := 1; attempt <= 3; attempt++ {
			err = dev.DeleteObject(handle)
			if err == nil {
				deleteSuccess = true
				break
			}

			util.LogError("Attempt %d: Error deleting file %s (ID: %d): %v",
				attempt, itemPath, handle, err)

			if attempt < 3 {
				time.Sleep(500 * time.Millisecond)
			}
		}

		if !deleteSuccess {
			util.LogInfo("Trying alternative deletion method...")
			err = TryAlternativeDeleteMethod(dev, storageID, handle)
			if err != nil {
				util.LogError("Alternative deletion method failed for %s: %v", itemPath, err)
				failedItems++
			} else {
				deleteSuccess = true
				deletedFiles++
			}
		} else {
			deletedFiles++
		}
	}

	// Only delete the folder itself if it's not the Music folder
	if folderID != 0 && !strings.EqualFold(folderPath, "/music") && !strings.EqualFold(folderPath, "/music/") {
		util.LogInfo("Deleting folder itself: %s", folderPath)

		deleteSuccess := false
		for attempt := 1; attempt <= 3; attempt++ {
			err = dev.DeleteObject(folderID)
			if err == nil {
				deleteSuccess = true
				break
			}

			util.LogError("Attempt %d: Error deleting folder %s (ID: %d): %v",
				attempt, folderPath, folderID, err)

			if attempt < 3 {
				time.Sleep(500 * time.Millisecond)
			}
		}

		if !deleteSuccess {
			util.LogInfo("Trying alternative deletion method for folder...")
			err = TryAlternativeDeleteMethod(dev, storageID, folderID)
			if err != nil {
				util.LogError("Alternative deletion method failed for folder %s: %v", folderPath, err)
				failedItems++
			} else {
				deletedFolders++
			}
		} else {
			deletedFolders++
		}
	}

	util.LogInfo("\nDeletion complete for %s:", folderPath)
	util.LogInfo("- Deleted files: %d", deletedFiles)
	util.LogInfo("- Deleted folders: %d", deletedFolders)
	util.LogInfo("- Failed items: %d", failedItems)

	if failedItems > 0 {
		return fmt.Errorf("completed with %d failed deletions", failedItems)
	}

	return nil
}

func DeleteFolder(dev *mtp.Device, storagesRaw interface{}) {
	util.LogInfo("\n=== Delete Folder and Contents ===")
	util.LogInfo("Starting folder deletion operation")

	storageID, musicFolderID, err := SelectStorageAndMusicFolder(dev, storagesRaw)
	if err != nil {
		util.LogError("Error selecting storage: %v", err)
		fmt.Printf("Error: %v\n", err)
		return
	}

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

	if optionChoice == 1 {

		musicFolderPath := "/Music"
		musicFolderID := uint32(0)

		musicFolderID, err = util.FindFolder(dev, storageID, PARENT_ROOT, "Music")
		if err != nil {

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

		err = DeleteFolderRecursively(dev, storageID, musicFolderID, musicFolderPath, true)
		if err != nil {
			util.LogError("Error deleting Music folder: %v", err)
			fmt.Printf("Error: %v\n", err)
		} else {
			util.LogInfo("Successfully deleted the entire Music folder.")
		}
		return
	}

	fmt.Println("\nAvailable folders in Music directory:")

	handles := mtp.Uint32Array{}
	err = dev.GetObjectHandles(storageID, 0, musicFolderID, &handles)
	if err != nil {
		util.LogError("Error getting folders: %v", err)
		fmt.Printf("Error getting folders: %v\n", err)
		return
	}

	type FolderInfo struct {
		ID   uint32
		Name string
		Path string
	}

	var folders []FolderInfo

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

	fmt.Print("\nSelect a folder to delete (1-" + fmt.Sprint(len(folders)) + "), or 0 to cancel: ")
	var selection int
	fmt.Scanln(&selection)

	if selection <= 0 || selection > len(folders) {
		fmt.Println("Operation cancelled.")
		return
	}

	selectedFolder := folders[selection-1]

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

	err = DeleteFolderRecursively(dev, storageID, selectedFolder.ID, selectedFolder.Path, false)
	if err != nil {
		util.LogError("Error during recursive deletion: %v", err)
		fmt.Printf("Error: %v\n", err)
		util.LogInfo("Some items may have been deleted successfully.")
	} else {
		util.LogInfo("Successfully deleted folder '%s' and all its contents.", selectedFolder.Path)
	}
}

func ExtractAndUploadAlbumArt(dev *mtp.Device, storageID uint32, parentID uint32, sourceFilePath string, artistName string, albumName string) (uint32, error) {
	util.LogInfo("Extracting album art from %s", sourceFilePath)

	file, err := os.Open(sourceFilePath)
	if err != nil {
		return 0, fmt.Errorf("failed to open source file: %v", err)
	}
	defer file.Close()

	artData, format, err := extractAlbumArtFromMP3(sourceFilePath)
	if err != nil {
		util.LogVerbose("No album art found or error extracting: %v", err)
		return 0, nil
	}

	if len(artData) == 0 {
		util.LogVerbose("No album art data found")
		return 0, nil
	}

	artFilename := fmt.Sprintf("%s - %s.%s", artistName, albumName, format)
	artFilename = sanitizeFilename(artFilename)

	existingArtID, err := findObjectByName(dev, storageID, parentID, artFilename)
	if err == nil && existingArtID != 0 {
		util.LogVerbose("Album art already exists with ID: %d", existingArtID)
		return existingArtID, nil
	}

	artInfo := mtp.ObjectInfo{
		StorageID:        storageID,
		ObjectFormat:     getMTPFormatByExtension(format),
		ParentObject:     parentID,
		Filename:         artFilename,
		CompressedSize:   uint32(len(artData)),
		ModificationDate: time.Now(),
	}

	_, _, sendObjectID, err := dev.SendObjectInfo(storageID, parentID, &artInfo)
	if err != nil {
		return 0, fmt.Errorf("failed to send album art object info: %v", err)
	}

	err = dev.SendObject(bytes.NewReader(artData), int64(len(artData)), model.EmptyProgressFunc)
	if err != nil {
		dev.DeleteObject(sendObjectID)
		return 0, fmt.Errorf("failed to send album art data: %v", err)
	}

	util.LogInfo("Successfully uploaded album art: %s (ID: %d)", artFilename, sendObjectID)
	return sendObjectID, nil
}

func extractAlbumArtFromMP3(filePath string) ([]byte, string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	header := make([]byte, 10)
	_, err = file.Read(header)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read file header: %v", err)
	}

	if string(header[0:3]) != "ID3" {
		return nil, "", fmt.Errorf("no ID3v2 tag found")
	}

	util.LogInfo("ID3 tag found, but full implementation of album art extraction requires an ID3 parsing library")
	util.LogInfo("To complete this implementation, add a Go ID3 parsing library to your project")

	return nil, "", fmt.Errorf("album art extraction not fully implemented")
}

func sanitizeFilename(filename string) string {
	invalidChars := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	result := filename

	for _, char := range invalidChars {
		result = strings.ReplaceAll(result, char, "_")
	}

	result = strings.TrimSpace(result)

	if len(result) > 255 {
		extension := filepath.Ext(result)
		nameWithoutExt := strings.TrimSuffix(result, extension)
		result = nameWithoutExt[:255-len(extension)] + extension
	}

	return result
}

func findObjectByName(dev *mtp.Device, storageID uint32, parentID uint32, filename string) (uint32, error) {
	handles := mtp.Uint32Array{}
	err := dev.GetObjectHandles(storageID, 0, parentID, &handles)
	if err != nil {
		return 0, fmt.Errorf("error getting object handles: %v", err)
	}

	for _, handle := range handles.Values {
		info := mtp.ObjectInfo{}
		err := dev.GetObjectInfo(handle, &info)
		if err != nil {
			continue
		}

		if info.Filename == filename {
			return handle, nil
		}
	}

	return 0, fmt.Errorf("object not found: %s", filename)
}

func getMTPFormatByExtension(ext string) uint16 {
	ext = strings.ToLower(strings.TrimPrefix(ext, "."))

	switch ext {
	case "jpg", "jpeg":
		return 0x3801 // JPEG
	case "png":
		return 0x380B // PNG
	case "gif":
		return 0x3807 // GIF
	case "bmp":
		return 0x3804 // BMP
	default:
		return 0x3000 // Unknown image format
	}
}

func EmptyProgressFunc(sent int64, total int64) uint32 {
	return 0
}
