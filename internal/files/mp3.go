package files

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bogem/id3v2"
	"github.com/schachte/better-sync/internal/util"
)

// CleanM3U8Playlist cleans a playlist file to ensure compatibility
func CleanM3U8Playlist(filePath string) error {
	// Read the file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("error reading playlist file: %w", err)
	}

	content := string(data)

	// Split into lines
	lines := strings.Split(content, "\n")
	var cleanedLines []string

	// Process each line
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Keep header and empty lines as-is
		if line == "" || strings.HasPrefix(line, "#") {
			cleanedLines = append(cleanedLines, line)
			continue
		}

		// Normalize path separators
		line = strings.ReplaceAll(line, "\\", "/")

		// Ensure path starts with slash if it doesn't have one and isn't a comment
		if !strings.HasPrefix(line, "/") && !strings.HasPrefix(line, "#") {
			line = "/" + line
		}

		cleanedLines = append(cleanedLines, line)
	}

	// Join and write back
	cleanedContent := strings.Join(cleanedLines, "\n")

	err = os.WriteFile(filePath, []byte(cleanedContent), 0644)
	if err != nil {
		return fmt.Errorf("error writing cleaned playlist file: %w", err)
	}

	return nil
}

// SanitizeID3Tags sanitizes the ID3 tags in an MP3 file
func SanitizeID3Tags(filePath string) error {
	// Open the MP3 file
	tag, err := id3v2.Open(filePath, id3v2.Options{Parse: true})
	if err != nil {
		return fmt.Errorf("error opening file for ID3 tag editing: %w", err)
	}
	defer tag.Close()

	// Get existing tags
	artist := tag.Artist()
	album := tag.Album()
	title := tag.Title()

	// If no title, try to extract from filename
	if title == "" {
		baseName := filepath.Base(filePath)
		ext := filepath.Ext(baseName)
		title = strings.TrimSuffix(baseName, ext)
		tag.SetTitle(title)
	}

	// If no artist, try to extract from filename or use default
	if artist == "" {
		artist = GetArtistFromFileName(filePath)
		if artist == "" {
			artist = "Unknown Artist"
		}
		tag.SetArtist(artist)
	}

	// If no album, try to extract from filename or use default
	if album == "" {
		album = getAlbumFromFileName(filePath)
		if album == "" {
			album = "Unknown Album"
		}
		tag.SetAlbum(album)
	}

	// Save the changes
	return tag.Save()
}

// Simple function to extract artist from filename (replace with actual ID3 tag reading)
func GetArtistFromFileName(filename string) string {
	// This is a simple example - in real world you'd use ID3 tags
	// Tries to match "Artist - Title.mp3" format
	re := regexp.MustCompile(`^(?:\d+\s+)?([^-]+)\s*-\s*.+\.mp3$`)
	matches := re.FindStringSubmatch(strings.ToLower(filename))

	if len(matches) > 1 {
		return util.SanitizeForPath(matches[1])
	}

	// Default if no pattern match
	return "UNKNOWN_ARTIST"
}

// getAlbumFromFileName attempts to extract album name from the directory name
func getAlbumFromFileName(filename string) string {
	// Use parent directory name as album
	dir := filepath.Dir(filename)
	if dir != "." && dir != "/" {
		return filepath.Base(dir)
	}
	return ""
}
