# MTP Music Manager

A command-line tool for managing music files and playlists on MTP devices like Garmin watches, smartphones, and other portable media players.

## Features

- List playlists and songs on MTP devices
- Upload songs to MTP devices
- Create and upload playlists
- Delete playlists and songs
- Bulk operations (upload directory as playlist, delete playlist and all songs)
- Automatic ID3 tag sanitization
- Timeout handling for MTP operations
- Helpful error messages for common MTP connection issues

## Installation

### From Source

1. Make sure you have Go 1.19 or newer installed
2. Clone the repository
3. Build the project:

```bash
git clone https://github.com/schachte/better-sync.git
cd better-sync
go build
```

## Usage

### Basic Usage

Run the application with:

```bash
./better-sync
```

This will display a menu of available operations.

### Command Line Options

```
  -op int
        Operation to perform (0 for menu, 1-10 for specific operation)
  -scan
        Only scan for MTP devices and exit
  -timeout int
        Timeout in seconds for device initialization (default 30)
  -verbose
        Enable verbose logging
```

### Examples

Scan for MTP devices only:

```bash
./better-sync -scan
```

Show playlists on the device:

```bash
./better-sync -op 1
```

Upload a song with verbose logging:

```bash
./better-sync -op 4 -verbose
```

## Troubleshooting

If you encounter issues connecting to your MTP device, try:

1. Ensuring the device is connected and in MTP mode
2. Closing other applications that might be accessing the device (e.g., Garmin Express, Google Drive)
3. Restarting the device
4. Using the `-verbose` flag for more detailed logging
5. Checking the log files in the `logs` directory

## Structure

The project is organized into the following packages:

- `cmd/mtpmusic`: Main entry point
- `internal/device`: Device connection and storage management
- `internal/files`: File operations (MP3, playlists)
- `internal/operations`: MTP operations (upload, delete, list)
- `internal/model`: Data structures
- `internal/util`: Utility functions (logging, sanitization)

## License

[MIT License](LICENSE)

## Acknowledgments

This project uses:

- [go-mtpfs](https://github.com/ganeshrvel/go-mtpfs) - MTP protocol implementation
- [go-mtpx](https://github.com/ganeshrvel/go-mtpx) - MTP utility functions
- [id3v2](https://github.com/bogem/id3v2) - ID3v2 tag reading and writing
