package util

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/ganeshrvel/go-mtpfs/mtp"
)

func ExtractUint32Field(obj interface{}, fieldName string) uint32 {
	if obj == nil {
		return 0
	}

	val := reflect.ValueOf(obj)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return 0
		}
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return 0
	}

	field := val.FieldByName(fieldName)
	if !field.IsValid() || field.Kind() != reflect.Uint32 {
		return 0
	}
	return uint32(field.Uint())
}

func ExtractStringField(obj interface{}, fieldName string) string {
	if obj == nil {
		return ""
	}

	val := reflect.ValueOf(obj)

	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return ""
		}
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return ""
	}

	field := val.FieldByName(fieldName)
	if !field.IsValid() || field.Kind() != reflect.String {
		return ""
	}
	return field.String()
}

func WrapError(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf(format+": %w", append(args, err)...)
}

func ExtractTrackInfo(path string) string {
	filename := filepath.Base(path)
	filename = strings.TrimSuffix(filename, filepath.Ext(filename))
	displayName := strings.ReplaceAll(filename, "_", " ")
	parts := strings.Split(path, "/")

	artist := "UNKNOWN ARTIST"
	title := displayName

	for i := 0; i < len(parts)-2; i++ {
		if strings.EqualFold(parts[i], "MUSIC") && i+1 < len(parts) {
			artist = strings.ReplaceAll(parts[i+1], "_", " ")
			break
		}
	}

	if artist == "UNKNOWN ARTIST" {
		if strings.Contains(displayName, " - ") {
			splitParts := strings.SplitN(displayName, " - ", 2)
			if len(splitParts) == 2 {
				artist = splitParts[0]
				title = splitParts[1]
			}
		}
	}

	return fmt.Sprintf("%s - %s", artist, title)
}

func GetObjectInfoWithRetry(dev *mtp.Device, handle uint32) (mtp.ObjectInfo, error) {
	var info mtp.ObjectInfo
	maxRetries := 3

	for attempt := 0; attempt < maxRetries; attempt++ {
		err := dev.GetObjectInfo(handle, &info)
		if err == nil {
			return info, nil
		}

		LogVerbose("Retry %d: Error getting object info for handle %d: %v",
			attempt+1, handle, err)
		time.Sleep(100 * time.Millisecond)
	}

	return info, fmt.Errorf("failed after %d retries", maxRetries)
}

func GetObjectHandlesWithRetry(dev *mtp.Device, storageID, objFormatCode, parent uint32) (mtp.Uint32Array, error) {
	var handles mtp.Uint32Array
	maxRetries := 3

	for attempt := 0; attempt < maxRetries; attempt++ {
		err := dev.GetObjectHandles(storageID, objFormatCode, parent, &handles)
		if err == nil {
			return handles, nil
		}

		LogVerbose("Retry %d: Error getting object handles for storageID %d, objFormatCode %d, parent %d: %v",
			attempt+1, storageID, objFormatCode, parent, err)
		time.Sleep(100 * time.Millisecond)
	}

	return handles, fmt.Errorf("failed after %d retries", maxRetries)
}

func FindOrCreateMusicFolder(dev *mtp.Device, storageID uint32) (uint32, error) {
	PARENT_ROOT := uint32(0)

	folderID, err := FindFolder(dev, storageID, PARENT_ROOT, "Music")
	if err != nil {
		LogError("Error finding Music folder: %v", err)

		if err.Error() == "folder not found" {
			LogInfo("Music folder not found, creating it")
			folderID, err = CreateFolder(dev, storageID, PARENT_ROOT, "Music")
			if err != nil {
				return 0, fmt.Errorf("error creating Music folder: %v", err)
			}
			LogInfo("Created Music folder with ID: %d", folderID)
			return folderID, nil
		}
		return 0, err
	}

	LogInfo("Found existing Music folder with ID: %d", folderID)
	return folderID, nil
}

func FindFolder(dev *mtp.Device, storageID, parentID uint32, folderName string) (uint32, error) {
	FILETYPE_FOLDER := uint16(0x3001)

	handles := mtp.Uint32Array{}
	err := dev.GetObjectHandles(storageID, 0, parentID, &handles)
	if err != nil {
		return 0, fmt.Errorf("error getting object handles: %v", err)
	}

	for _, handle := range handles.Values {
		info := mtp.ObjectInfo{}
		err = dev.GetObjectInfo(handle, &info)
		if err != nil {
			LogError("Error getting object info for handle %d: %v", handle, err)
			continue
		}

		if info.ObjectFormat == FILETYPE_FOLDER && info.Filename == folderName {
			return handle, nil
		}
	}

	return 0, fmt.Errorf("folder not found")
}

func CreateFolder(dev *mtp.Device, storageID, parentID uint32, folderName string) (uint32, error) {
	FILETYPE_FOLDER := uint16(0x3001)

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
