package device

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/ganeshrvel/go-mtpfs/mtp"
	"github.com/ganeshrvel/go-mtpx"
	"github.com/schachte/better-sync/internal/util"
)

type StorageInfo struct {
	StorageID          uint32
	StorageDescription string
}

func FetchStorages(dev *mtp.Device, timeout time.Duration) (interface{}, error) {
	util.LogVerbose("Fetching storages with timeout of %v...", timeout)
	fmt.Println("Fetching device storage information...")

	// Create channels for communicating results
	type storageResult struct {
		storages interface{}
		err      error
	}
	storageCh := make(chan storageResult, 1)

	go func() {
		// Get the storage information using mtpx
		storages, err := mtpx.FetchStorages(dev)
		if err != nil {
			util.LogError("Failed to fetch storages: %v", err)
			storageCh <- storageResult{nil, err}
			return
		}

		// Use reflection to determine if we got storage info
		storagesValue := reflect.ValueOf(storages)
		if storagesValue.Kind() != reflect.Slice || storagesValue.Len() == 0 {
			util.LogError("No storage found on device")
			storageCh <- storageResult{nil, fmt.Errorf("no storage found on device")}
			return
		}

		util.LogVerbose("Found %d storage(s) on device", storagesValue.Len())

		// Log information about each storage
		for i := 0; i < storagesValue.Len(); i++ {
			storage := storagesValue.Index(i).Interface()

			// Extract information using reflection since we don't know the exact type
			sid := extractUint32Field(storage, "Sid")
			desc := extractStringField(storage, "StorageDescription")
			if desc == "" {
				desc = extractStringField(storage, "Description")
			}

			util.LogVerbose("Storage #%d: %s (ID: %d)", i+1, desc, sid)
		}

		storageCh <- storageResult{storages, nil}
	}()

	// Wait for storage fetch with timeout
	select {
	case result := <-storageCh:
		if result.err != nil {
			return nil, result.err
		}
		return result.storages, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("storage fetch timed out after %v", timeout)
	}
}

func SelectStorage(dev *mtp.Device, storagesRaw interface{}) (uint32, error) {
	// Convert to slice for easier handling
	storagesValue := reflect.ValueOf(storagesRaw)
	if storagesValue.Kind() != reflect.Slice || storagesValue.Len() == 0 {
		return 0, fmt.Errorf("no storage found on device")
	}

	// If only one storage, select it automatically
	if storagesValue.Len() == 1 {
		storage := storagesValue.Index(0).Interface()
		sid := extractUint32Field(storage, "Sid")
		desc := extractStringField(storage, "StorageDescription")
		if desc == "" {
			desc = extractStringField(storage, "Description")
		}

		fmt.Printf("Using the only available storage: %s (ID: %d)\n", desc, sid)
		return sid, nil
	}

	// Show available storages
	fmt.Println("\nAvailable storages:")
	for i := 0; i < storagesValue.Len(); i++ {
		storage := storagesValue.Index(i).Interface()
		sid := extractUint32Field(storage, "Sid")
		desc := extractStringField(storage, "StorageDescription")
		if desc == "" {
			desc = extractStringField(storage, "Description")
		}

		fmt.Printf("%d. %s (ID: %d)\n", i+1, desc, sid)
	}

	// Let user select
	var selection int
	fmt.Print("\nSelect storage (1-" + fmt.Sprint(storagesValue.Len()) + "): ")
	_, err := fmt.Scanln(&selection)
	if err != nil || selection < 1 || selection > storagesValue.Len() {
		return 0, fmt.Errorf("invalid selection")
	}

	// Get the selected storage
	storage := storagesValue.Index(selection - 1).Interface()
	sid := extractUint32Field(storage, "Sid")
	desc := extractStringField(storage, "StorageDescription")
	if desc == "" {
		desc = extractStringField(storage, "Description")
	}

	fmt.Printf("Selected storage: %s (ID: %d)\n", desc, sid)
	return sid, nil
}

const (
	PARENT_ROOT     uint32 = 0
	FILETYPE_FOLDER uint16 = 0x3001
)

func FetchStoragesWithTimeout(dev *mtp.Device, timeout time.Duration) (interface{}, error) {
	util.LogVerbose("Fetching storages with timeout of %v...", timeout)
	fmt.Println("Requesting storage information from device...")

	type storageResult struct {
		storages interface{}
		err      error
	}

	storageCh := make(chan storageResult, 1)

	go func() {
		fmt.Println("Starting storage fetch...")
		storages, err := mtpx.FetchStorages(dev)

		if err != nil {
			util.LogError("Storage fetch failed: %v", err)
			fmt.Printf("Storage fetch failed: %v\n", err)
		} else {
			// Log information about the storages
			storagesValue := reflect.ValueOf(storages)
			if storagesValue.Kind() == reflect.Slice {
				fmt.Printf("Found %d storage(s) on device\n", storagesValue.Len())
				util.LogVerbose("Found %d storage(s) on device", storagesValue.Len())
			}
		}

		storageCh <- storageResult{storages, err}
	}()

	// Wait for storage fetch with timeout
	select {
	case result := <-storageCh:
		if result.err != nil {
			return nil, result.err
		}
		util.LogVerbose("Storages fetched successfully")
		return result.storages, nil
	case <-time.After(timeout):
		fmt.Println("\nStorage fetch timed out! Possible causes:")
		fmt.Println(" - Device communication issues")
		fmt.Println(" - Device is busy or unresponsive")
		fmt.Println(" - Internal device errors")
		return nil, fmt.Errorf("fetching storages timed out after %v", timeout)
	}
}

func SelectStorageAndMusicFolder(dev *mtp.Device, storagesRaw interface{}) (uint32, uint32, error) {
	// Convert to slice for easier handling
	storagesValue := reflect.ValueOf(storagesRaw)
	if storagesValue.Kind() != reflect.Slice || storagesValue.Len() == 0 {
		return 0, 0, fmt.Errorf("no storage found on device")
	}

	// If there's only one storage, use it automatically
	if storagesValue.Len() == 1 {
		firstStorage := storagesValue.Index(0).Interface()
		storageID := extractUint32Field(firstStorage, "Sid")
		storageDesc := extractStringField(firstStorage, "Description")

		util.LogVerbose("Automatically selected storage: %s (ID: %d)", storageDesc, storageID)
		fmt.Printf("Automatically selected storage: %s (ID: %d)\n", storageDesc, storageID)

		// Find or create music folder on the selected storage
		musicFolderID, err := util.FindOrCreateMusicFolder(dev, storageID)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to find or create Music folder: %v", err)
		}

		return storageID, musicFolderID, nil
	}

	// If multiple storages, ask user to select one
	fmt.Println("\nAvailable storages:")
	for i := 0; i < storagesValue.Len(); i++ {
		storageObj := storagesValue.Index(i).Interface()
		storageID := extractUint32Field(storageObj, "Sid")
		description := extractStringField(storageObj, "Description")
		if description == "" {
			description = extractStringField(storageObj, "StorageDescription")
		}
		fmt.Printf("%d. %s (ID: %d)\n", i+1, description, storageID)
	}

	fmt.Print("Select storage (1-", storagesValue.Len(), "): ")
	var selection int
	fmt.Scanln(&selection)

	if selection < 1 || selection > storagesValue.Len() {
		return 0, 0, fmt.Errorf("invalid storage selection: %d", selection)
	}

	// Get the selected storage
	storageObj := storagesValue.Index(selection - 1).Interface()
	storageID := extractUint32Field(storageObj, "Sid")
	storageDesc := extractStringField(storageObj, "Description")
	if storageDesc == "" {
		storageDesc = extractStringField(storageObj, "StorageDescription")
	}

	fmt.Printf("Selected storage: %s (ID: %d)\n", storageDesc, storageID)
	util.LogVerbose("Selected storage: %s (ID: %d)", storageDesc, storageID)

	// Find or create music folder on the selected storage
	musicFolderID, err := util.FindOrCreateMusicFolder(dev, storageID)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to find or create Music folder: %v", err)
	}

	return storageID, musicFolderID, nil
}

func CreateFolder(dev *mtp.Device, storageID, parentID uint32, folderName string) (uint32, error) {
	info := mtp.ObjectInfo{
		StorageID:        storageID,
		ObjectFormat:     FILETYPE_FOLDER,
		ParentObject:     parentID,
		Filename:         folderName,
		AssociationType:  0,
		AssociationDesc:  0,
		SequenceNumber:   0,
		ModificationDate: time.Now(),
	}

	var newObjectID uint32
	_, _, newObjectID, err := dev.SendObjectInfo(storageID, parentID, &info)
	if err != nil {
		return 0, fmt.Errorf("error sending folder info: %v", err)
	}

	return newObjectID, nil
}

func FindOrCreateFolder(dev *mtp.Device, storageID, parentID uint32, folderName string) (uint32, error) {
	folderName = strings.ToUpper(folderName)
	// First try to find the folder
	folderID, err := util.FindFolder(dev, storageID, parentID, folderName)
	if err == nil {
		// Folder found, return its ID
		util.LogVerbose("Using existing folder: %s (ID: %d)", folderName, folderID)
		return folderID, nil
	}

	// If folder not found, try to create it
	util.LogVerbose("Folder '%s' not found, attempting to create it", folderName)
	folderID, err = CreateFolder(dev, storageID, parentID, folderName)
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

func extractUint32Field(obj interface{}, fieldName string) uint32 {
	val := reflect.ValueOf(obj)
	field := val.FieldByName(fieldName)
	if !field.IsValid() || field.Kind() != reflect.Uint32 {
		return 0 // Default value if field doesn't exist or is not uint32
	}
	return uint32(field.Uint())
}

func extractStringField(obj interface{}, fieldName string) string {
	val := reflect.ValueOf(obj)
	field := val.FieldByName(fieldName)
	if !field.IsValid() || field.Kind() != reflect.String {
		return "" // Default value if field doesn't exist or is not string
	}
	return field.String()
}
