package operations

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/ganeshrvel/go-mtpfs/mtp"
	"github.com/ganeshrvel/go-mtpx"
	"github.com/schachte/better-sync/internal/model"
	"github.com/schachte/better-sync/internal/util"
)

func FindOrCreateMusicFolder(dev *mtp.Device, storageID uint32) (uint32, error) {

	folderID, err := util.FindFolder(dev, storageID, PARENT_ROOT, "Music")
	if err != nil {
		util.LogError("Error finding Music folder: %v", err)

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

	path = strings.TrimSpace(path)

	path = strings.TrimPrefix(path, "0:")

	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	path = strings.Replace(path, "/MUSIC/", "/Music/", 1)
	path = strings.Replace(path, "/music/", "/Music/", 1)

	util.LogVerbose("Normalized path for lookup: %s", path)

	objectID, err := FindObjectByPathManual(dev, storageID, path)
	if err == nil {
		util.LogVerbose("Found object by path: %s (ID: %d)", path, objectID)
		return objectID, nil
	}

	util.LogVerbose("Manual path traversal failed: %v, trying alternative methods", err)

	mp3Files, err := FindMP3Files(dev, storageID)
	if err != nil {
		return 0, fmt.Errorf("error searching for MP3 files: %v", err)
	}

	filename := filepath.Base(path)

	baseNameWithoutNumber := filename
	parts := strings.SplitN(filename, " ", 2)
	if len(parts) == 2 && isNumeric(parts[0]) {
		baseNameWithoutNumber = parts[1]
	}

	util.LogVerbose("Looking for file with base name: %s or %s", filename, baseNameWithoutNumber)

	for _, mp3Path := range mp3Files {
		mp3Filename := filepath.Base(mp3Path)

		if mp3Filename == filename ||
			mp3Filename == baseNameWithoutNumber ||
			strings.EqualFold(mp3Filename, filename) ||
			strings.EqualFold(mp3Filename, baseNameWithoutNumber) {
			util.LogVerbose("Found file with matching name: %s", mp3Path)

			objectID, err := FindObjectByPathManual(dev, storageID, mp3Path)
			if err == nil && objectID != 0 {
				return objectID, nil
			}
		}
	}

	for _, mp3Path := range mp3Files {

		pathParts := strings.Split(strings.ToLower(path), "/")
		mp3Parts := strings.Split(strings.ToLower(mp3Path), "/")

		if len(pathParts) >= 3 && len(mp3Parts) >= 3 {

			if pathParts[len(pathParts)-2] == mp3Parts[len(mp3Parts)-2] {

				pathFilename := pathParts[len(pathParts)-1]
				mp3Filename := mp3Parts[len(mp3Parts)-1]

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

	baseName := strings.TrimSuffix(filename, filepath.Ext(filename))

	parts := strings.SplitN(baseName, " ", 2)
	if len(parts) == 2 && isNumeric(parts[0]) {
		baseName = parts[1]
	}

	return strings.ToUpper(baseName)
}

func FindObjectByPathManual(dev *mtp.Device, storageID uint32, path string) (uint32, error) {

	path = strings.TrimSpace(path)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	variations := GeneratePathVariations(path)

	for _, pathVar := range variations {
		util.LogVerbose("Trying path variation: %s", pathVar)

		components := strings.Split(pathVar, "/")

		var parts []string
		for _, comp := range components {
			if comp != "" {
				parts = append(parts, comp)
			}
		}

		currentID := PARENT_ROOT

		for i, component := range parts {

			handles := mtp.Uint32Array{}
			err := dev.GetObjectHandles(storageID, 0, currentID, &handles)
			if err != nil {
				util.LogVerbose("Error getting objects in folder (ID: %d): %v", currentID, err)
				break // Try next variation
			}

			found := false
			for _, handle := range handles.Values {
				info := mtp.ObjectInfo{}
				err = dev.GetObjectInfo(handle, &info)
				if err != nil {
					util.LogVerbose("Error getting object info for handle %d: %v", handle, err)
					continue
				}

				if info.Filename == component || strings.EqualFold(info.Filename, component) {

					if i == len(parts)-1 {
						util.LogVerbose("Found object: %s (ID: %d)", component, handle)
						return handle, nil
					}

					currentID = handle
					found = true
					break
				}
			}

			if !found {

				break
			}
		}
	}

	return 0, fmt.Errorf("could not find object with path: %s", path)
}

func GeneratePathVariations(path string) []string {

	path = strings.TrimSpace(path)

	pathVariations := []string{
		path,
		"/" + strings.TrimPrefix(path, "/"),
		strings.TrimPrefix(path, "/"),
	}

	withoutPrefix := strings.TrimPrefix(path, "/")
	pathVariations = append(pathVariations,
		"0:"+path,
		"0:/"+withoutPrefix,
	)

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

	if strings.HasPrefix(path, "/") {
		path = path[1:]
	}

	if path == "" {
		return PARENT_ROOT, nil
	}

	components := strings.Split(path, "/")
	currentFolderID := PARENT_ROOT

	for _, component := range components {
		if component == "" {
			continue
		}

		folderID, err := util.FindFolder(dev, storageID, currentFolderID, component)
		if err != nil {
			return 0, fmt.Errorf("folder '%s' not found in path '%s': %v",
				component, path, err)
		}

		currentFolderID = folderID
	}

	return currentFolderID, nil
}

func FindPlaylistsInFolder(dev *mtp.Device, storageID, folderID uint32, folderPath string) ([]string, error) {
	var playlists []string

	handles, err := util.GetObjectHandlesWithRetry(dev, storageID, 0, folderID)
	if err != nil {
		return nil, fmt.Errorf("error getting object handles: %v", err)
	}

	for _, handle := range handles.Values {

		info, err := util.GetObjectInfoWithRetry(dev, handle)
		if err != nil {
			util.LogVerbose("Error getting object info for handle %d: %v", handle, err)
			continue
		}

		ext := strings.ToLower(filepath.Ext(info.Filename))
		if ext == ".m3u8" || ext == ".m3u" || ext == ".pls" {

			path := filepath.Join(folderPath, info.Filename)
			playlists = append(playlists, path)

			util.LogVerbose("Found playlist via direct search: %s (ID: %d, Format: %d)",
				path, handle, info.ObjectFormat)
		} else if info.ObjectFormat == FILETYPE_FOLDER {

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

	musicFolderPaths := []string{"/Music"}

	for _, musicPath := range musicFolderPaths {

		musicFolderID, err := GetFolderIDByPath(dev, storageID, musicPath)
		if err != nil {
			util.LogInfo("%s folder not found: %v", musicPath, err)

			continue
		}

		handles := mtp.Uint32Array{}
		err = dev.GetObjectHandles(storageID, 0, musicFolderID, &handles)
		if err != nil {
			util.LogError("Error getting object handles in %s folder: %v", musicPath, err)
			continue
		}

		for _, handle := range handles.Values {

			info := mtp.ObjectInfo{}
			err = dev.GetObjectInfo(handle, &info)
			if err != nil {
				util.LogVerbose("Error getting object info for handle %d: %v", handle, err)
				continue
			}

			ext := strings.ToLower(filepath.Ext(info.Filename))
			if ext == ".m3u8" || ext == ".m3u" || ext == ".pls" {

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

// EnhancedDeletePlaylistAndAllSongs deletes a playlist and all its songs
func EnhancedDeletePlaylistAndAllSongs(dev *mtp.Device, storagesRaw interface{}, playlistName string) error {
	fmt.Println("\n=== Delete Playlist ===")
	util.LogVerbose("Starting playlist deletion operation for %s", playlistName)

	playlists, err := GetPlaylists(dev, storagesRaw)
	if err != nil {
		return fmt.Errorf("error getting playlists: %w", err)
	}

	var targetPlaylist *model.PlaylistInfo
	for i, playlist := range playlists {
		util.LogVerbose("Playlist: %s", playlist.Name)

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

	songs, err := ReadPlaylistContent(dev, targetPlaylist.StorageID, targetPlaylist.ObjectID)
	if err != nil {
		return fmt.Errorf("error reading playlist content: %w", err)
	}

	fmt.Printf("Playlist contains %d songs\n", len(songs))
	util.LogInfo("Playlist contains %d songs", len(songs))

	deletedSongs := 0
	for _, songPath := range songs {
		normalizedPath := strings.TrimPrefix(songPath, "0:")

		if !strings.HasPrefix(normalizedPath, "/") {
			normalizedPath = "/" + normalizedPath
		}

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

	musicPaths := []string{"/Music"}
	var totalFiles int64
	var foundAnyFiles bool = false

	for _, basePath := range musicPaths {
		util.LogInfo("Searching in %s directory", basePath)
		_, count, _, walkErr := mtpx.Walk(dev, storageID, basePath, true, true, false,
			func(objectID uint32, fi *mtpx.FileInfo, err error) error {
				if err != nil {
					return err
				}

				if !fi.IsDir && strings.ToLower(filepath.Ext(fi.FullPath)) == ".mp3" {

					if fi.Size == 0 {
						emptyPath := strings.ToUpper(fi.FullPath)
						emptyFiles = append(emptyFiles, emptyPath)
						util.LogError("Found EMPTY MP3: %s (Size: 0 bytes, ID: %d)", fi.FullPath, objectID)
					} else {

						mp3Files = append(mp3Files, strings.ToUpper(fi.FullPath))
						util.LogVerbose("Found MP3: %s (Size: %d bytes, ID: %d)", fi.FullPath, fi.Size, objectID)
						foundAnyFiles = true
					}
				}

				return nil
			})

		if walkErr != nil {

			util.LogError("Error walking path %s: %v", basePath, walkErr)
		} else {
			totalFiles += count
			util.LogInfo("Found %d total objects in %s directory", count, basePath)
		}
	}

	if len(emptyFiles) > 0 && len(mp3Files) == 0 {
		util.LogError("Found %d EMPTY MP3 files (failed uploads) but no valid MP3 files", len(emptyFiles))
		util.LogInfo("HINT: Your files were uploaded but the data transfer failed.")
		util.LogInfo("Try re-uploading with the updated file transfer system.")

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

// GetSongs retrieves all MP3 files from the device
func GetSongs(dev *mtp.Device, storagesRaw interface{}) ([]model.Song, error) {
	var allSongs []model.Song

	storagesValue := reflect.ValueOf(storagesRaw)
	if storagesValue.Kind() != reflect.Slice {
		return nil, fmt.Errorf("invalid storages data format: not a slice")
	}

	for i := 0; i < storagesValue.Len(); i++ {
		storageObj := storagesValue.Index(i).Interface()
		storageID := extractUint32Field(storageObj, "Sid")
		storageDesc := extractStringField(storageObj, "StorageDescription")
		if storageDesc == "" {
			storageDesc = extractStringField(storageObj, "Description")
		}

		mp3Files, err := FindMP3Files(dev, storageID)
		if err != nil {
			util.LogError("Error searching storage %s (ID: %d): %v", storageDesc, storageID, err)
			continue
		}

		util.LogVerbose("Found %d MP3 files in storage: %s", len(mp3Files), storageDesc)

		for _, filePath := range mp3Files {
			objectID, err := FindObjectByPath(dev, storageID, filePath)
			if err != nil {
				util.LogVerbose("Could not find object ID for file %s: %v", filePath, err)
				objectID = 0 // Use 0 to indicate unknown object ID
			}

			song := model.Song{
				Name:      filepath.Base(filePath),
				Path:      filePath,
				ObjectID:  objectID,
				StorageID: storageID,
				Storage:   storageDesc,
			}

			allSongs = append(allSongs, song)
		}
	}

	util.LogVerbose("Retrieved %d total MP3 files across all storages", len(allSongs))
	return allSongs, nil
}

// GetPlaylists retrieves all playlists from the device across all storages
func GetPlaylists(dev *mtp.Device, storagesRaw interface{}) ([]model.PlaylistInfo, error) {
	storagesValue := reflect.ValueOf(storagesRaw)
	if storagesValue.Kind() != reflect.Slice {
		return nil, fmt.Errorf("invalid storages data format: not a slice")
	}

	var allPlaylists []model.PlaylistInfo

	for i := 0; i < storagesValue.Len(); i++ {
		storageObj := storagesValue.Index(i).Interface()
		storageID := extractUint32Field(storageObj, "Sid")
		storageDesc := extractStringField(storageObj, "StorageDescription")
		if storageDesc == "" {
			storageDesc = extractStringField(storageObj, "Description")
		}

		playlists, err := FindPlaylists(dev, storageID)
		if err != nil {
			util.LogError("Failed to find playlists in storage %s (ID: %d): %v",
				storageDesc, storageID, err)
			continue
		}

		util.LogVerbose("Found %d playlists in storage: %s", len(playlists), storageDesc)

		for _, playlistPath := range playlists {
			objectID, err := FindObjectByPath(dev, storageID, playlistPath)
			if err != nil {
				util.LogError("Failed to find object ID for playlist %s: %v",
					playlistPath, err)
				continue
			}

			playlist := model.PlaylistInfo{
				Name:      filepath.Base(playlistPath),
				Path:      playlistPath,
				ObjectID:  objectID,
				StorageID: storageID,
				Storage:   storageDesc,
			}

			allPlaylists = append(allPlaylists, playlist)
		}
	}

	util.LogVerbose("Retrieved %d total playlists across all storages", len(allPlaylists))
	return allPlaylists, nil
}

// GetPlaylistsWithSongs retrieves all playlists and their songs from the device
func GetPlaylistsWithSongs(dev *mtp.Device, storagesRaw interface{}) (*model.DevicePlaylistData, error) {
	// Reuse GetPlaylists function (which returns PlaylistInfo)
	playlistInfos, err := GetPlaylists(dev, storagesRaw)
	if err != nil {
		return nil, err
	}

	result := &model.DevicePlaylistData{
		TotalPlaylists: len(playlistInfos),
		Storages:       []model.StoragePlaylistData{},
	}

	// Group playlists by storage
	storageMap := make(map[uint32]*model.StoragePlaylistData)

	for _, playlistInfo := range playlistInfos {
		// Get or create storage entry
		storage, exists := storageMap[playlistInfo.StorageID]
		if !exists {
			storage = &model.StoragePlaylistData{
				StorageID:          playlistInfo.StorageID,
				StorageDescription: playlistInfo.Storage,
				Playlists:          []model.Playlist{},
			}
			storageMap[playlistInfo.StorageID] = storage
		}

		// Get songs in playlist
		songPaths, err := ReadPlaylistContent(dev, playlistInfo.StorageID, playlistInfo.ObjectID)
		if err != nil {
			util.LogError("Failed to read content for playlist %s (ID: %d): %v",
				playlistInfo.Path, playlistInfo.ObjectID, err)
			songPaths = []string{} // Empty slice in case of error
		} else {
			util.LogVerbose("Found %d songs in playlist %s", len(songPaths), playlistInfo.Path)
		}

		// Convert PlaylistInfo to model.Playlist (which has different fields)
		parentID, err := GetParentIDForObject(dev, playlistInfo.ObjectID)
		if err != nil {
			util.LogVerbose("Could not determine parent ID for playlist %s: %v", playlistInfo.Path, err)
			parentID = model.PARENT_ROOT
		}

		playlist := model.Playlist{
			Path:        playlistInfo.Path,
			ObjectID:    playlistInfo.ObjectID,
			ParentID:    parentID,
			StorageID:   playlistInfo.StorageID,
			StorageDesc: playlistInfo.Storage,
			SongPaths:   songPaths,
		}

		storage.Playlists = append(storage.Playlists, playlist)
	}

	// Convert map to slice
	for _, storage := range storageMap {
		result.Storages = append(result.Storages, *storage)
	}

	util.LogVerbose("Processed %d playlists with songs across %d storages",
		result.TotalPlaylists, len(result.Storages))

	return result, nil
}

// Helper function to get parent ID for an object
func GetParentIDForObject(dev *mtp.Device, objectID uint32) (uint32, error) {
	info := mtp.ObjectInfo{}
	err := dev.GetObjectInfo(objectID, &info)
	if err != nil {
		return 0, fmt.Errorf("failed to get object info: %w", err)
	}
	return info.ParentObject, nil
}
