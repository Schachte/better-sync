package model

import "github.com/ganeshrvel/go-mtpfs/mtp"

// Constants missing from mtp package
const (
	PARENT_ROOT     uint32 = 0
	FILETYPE_FOLDER uint16 = 0x3001
)

// StorageInfo contains information about an MTP storage
type StorageInfo struct {
	StorageID   uint32
	Description string
	DisplayName string
}

// MP3File represents an MP3 file on the MTP device
type MP3File struct {
	Path        string
	ObjectID    uint32
	ParentID    uint32
	StorageID   uint32
	DisplayName string
}

// Playlist represents a playlist on the MTP device
type Playlist struct {
	Path        string
	ObjectID    uint32
	ParentID    uint32
	StorageID   uint32
	StorageDesc string
	SongPaths   []string
}

// EmptyProgressFunc is a placeholder progress function for MTP operations
func EmptyProgressFunc(_ int64) error {
	return nil
}

// Progress function type for file transfer operations
type ProgressFunc func(progress int64) error

// PlaylistEntry represents a playlist found on the device
type PlaylistEntry struct {
	StorageID   uint32
	Path        string
	ObjectID    uint32
	ParentID    uint32
	StorageDesc string
}

// SongEntry represents a song file found on the device
type SongEntry struct {
	StorageID   uint32
	Path        string
	ObjectID    uint32
	ParentID    uint32
	StorageDesc string
	DisplayName string
}

// DeviceInfo contains device information
type DeviceInfo struct {
	Dev      *mtp.Device
	Storages interface{}
}
