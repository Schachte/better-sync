package model

import "github.com/ganeshrvel/go-mtpfs/mtp"

const (
	PARENT_ROOT    uint32 = 0
	FILETYPEFOLDER uint16 = 0x3001
)

type StorageInfo struct {
	StorageID   uint32
	Description string
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

// PlaylistSong represents a song within a playlist
type PlaylistSong struct {
	Name     string
	Path     string
	ObjectID uint32
}

// StoragePlaylistData represents playlists in a specific storage
type StoragePlaylistData struct {
	StorageID          uint32
	StorageDescription string
	Playlists          []Playlist
}

// DevicePlaylistData represents all playlists on a device
type DevicePlaylistData struct {
	TotalPlaylists int
	Storages       []StoragePlaylistData
}

// PlaylistInfo structure for backward compatibility
type PlaylistInfo struct {
	Name      string
	Path      string
	ObjectID  uint32
	StorageID uint32
	Storage   string
}

// Song represents an MP3 file on the device
type Song struct {
	Name      string
	Path      string
	ObjectID  uint32
	StorageID uint32
	Storage   string
}

// FileUploadResult contains information about a single file upload
type FileUploadResult struct {
	Success      bool
	UploadedPath string
	ObjectID     uint32
	DisplayName  string
	Error        string
}

// UploadResult contains information about the entire upload operation
type UploadResult struct {
	Success       bool
	UploadedFiles []MP3File
	Playlist      *Playlist
	Errors        []string
}

type MP3File struct {
	Path        string
	ObjectID    uint32
	ParentID    uint32
	StorageID   uint32
	DisplayName string
}
