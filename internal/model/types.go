package model

import "github.com/ganeshrvel/go-mtpfs/mtp"

const (
	PARENT_ROOT     uint32 = 0
	FILETYPE_FOLDER uint16 = 0x3001
)

type StorageInfo struct {
	StorageID   uint32
	Description string
	DisplayName string
}

type MP3File struct {
	Path        string
	ObjectID    uint32
	ParentID    uint32
	StorageID   uint32
	DisplayName string
}

type Playlist struct {
	Path        string
	ObjectID    uint32
	ParentID    uint32
	StorageID   uint32
	StorageDesc string
	SongPaths   []string
}

func EmptyProgressFunc(_ int64) error {
	return nil
}

type ProgressFunc func(progress int64) error

type PlaylistEntry struct {
	StorageID   uint32
	Path        string
	ObjectID    uint32
	ParentID    uint32
	StorageDesc string
}

type SongEntry struct {
	StorageID   uint32
	Path        string
	ObjectID    uint32
	ParentID    uint32
	StorageDesc string
	DisplayName string
}

type DeviceInfo struct {
	Dev      *mtp.Device
	Storages interface{}
}
