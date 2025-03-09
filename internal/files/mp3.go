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

func CleanM3U8Playlist(filePath string) error {

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("error reading playlist file: %w", err)
	}

	content := string(data)

	lines := strings.Split(content, "\n")
	var cleanedLines []string

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line == "" || strings.HasPrefix(line, "#") {
			cleanedLines = append(cleanedLines, line)
			continue
		}

		line = strings.ReplaceAll(line, "\\", "/")

		if !strings.HasPrefix(line, "/") && !strings.HasPrefix(line, "#") {
			line = "/" + line
		}

		cleanedLines = append(cleanedLines, line)
	}

	cleanedContent := strings.Join(cleanedLines, "\n")

	err = os.WriteFile(filePath, []byte(cleanedContent), 0644)
	if err != nil {
		return fmt.Errorf("error writing cleaned playlist file: %w", err)
	}

	return nil
}

func SanitizeID3Tags(filePath string) error {

	tag, err := id3v2.Open(filePath, id3v2.Options{Parse: true})
	if err != nil {
		return fmt.Errorf("error opening file for ID3 tag editing: %w", err)
	}
	defer tag.Close()

	artist := tag.Artist()
	album := tag.Album()
	title := tag.Title()

	if title == "" {
		baseName := filepath.Base(filePath)
		ext := filepath.Ext(baseName)
		title = strings.TrimSuffix(baseName, ext)
		tag.SetTitle(title)
	}

	if artist == "" {
		artist = GetArtistFromFileName(filePath)
		if artist == "" {
			artist = "Unknown Artist"
		}
		tag.SetArtist(artist)
	}

	if album == "" {
		album = getAlbumFromFileName(filePath)
		if album == "" {
			album = "Unknown Album"
		}
		tag.SetAlbum(album)
	}

	return tag.Save()
}

func GetArtistFromFileName(filename string) string {

	re := regexp.MustCompile(`^(?:\d+\s+)?([^-]+)\s*-\s*.+\.mp3$`)
	matches := re.FindStringSubmatch(strings.ToLower(filename))

	if len(matches) > 1 {
		return util.SanitizeForPath(matches[1])
	}

	return "UNKNOWN_ARTIST"
}

func getAlbumFromFileName(filename string) string {

	dir := filepath.Dir(filename)
	if dir != "." && dir != "/" {
		return filepath.Base(dir)
	}
	return ""
}
