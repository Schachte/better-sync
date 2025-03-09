package util

import (
	"path/filepath"
	"regexp"
	"strings"
)

func SanitizeFolderName(name string) string {
	// For exact matching with the playlist paths, we need to preserve
	// characters like ! and _ that appear in the examples

	// Create a list of allowed characters beyond alphanumeric
	allowedSpecialChars := map[rune]bool{
		'!':  true,
		'_':  true,
		'-':  true,
		' ':  true, // Will be replaced with underscores later
		'&':  true,
		'(':  true,
		')':  true,
		'+':  true,
		'.':  true,
		'\'': true,
	}

	// Create a new string builder
	var result strings.Builder

	// Iterate through each character
	for _, char := range name {
		// Allow letters, numbers, and specific special characters
		if (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') ||
			(char >= '0' && char <= '9') || allowedSpecialChars[char] {
			// Replace spaces with underscores
			if char == ' ' {
				result.WriteRune('_')
			} else {
				result.WriteRune(char)
			}
		} else {
			// Replace any other character with underscore
			result.WriteRune('_')
		}
	}

	// Convert to string
	sanitized := result.String()

	// Remove consecutive underscores
	for strings.Contains(sanitized, "__") {
		sanitized = strings.ReplaceAll(sanitized, "__", "_")
	}

	// Trim underscores from start and end
	sanitized = strings.Trim(sanitized, "_")

	// Limit length
	if len(sanitized) > 64 {
		sanitized = sanitized[:64]
	}

	// Ensure not empty
	if sanitized == "" {
		sanitized = "unnamed"
	}

	return sanitized
}

func SanitizeFileName(name string) string {
	// Handle empty input
	if name == "" {
		return "unnamed.mp3"
	}

	// Get the file extension
	ext := filepath.Ext(name)
	var baseName string

	// Make sure we don't have a negative slice when the name equals the extension
	if len(name) > len(ext) {
		baseName = name[:len(name)-len(ext)]
	} else {
		baseName = ""
	}

	// Define allowed special characters
	allowedSpecialChars := map[rune]bool{
		'!':  true,
		'_':  true,
		'-':  true,
		' ':  true, // Will be replaced with underscores later
		'&':  true,
		'(':  true,
		')':  true,
		'+':  true,
		'.':  true,
		'\'': true,
	}

	// Create a new string builder
	var result strings.Builder

	// Iterate through each character in the base name
	for _, char := range baseName {
		// Allow letters, numbers, and specific special characters
		if (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') ||
			(char >= '0' && char <= '9') || allowedSpecialChars[char] {
			// Replace spaces with underscores
			if char == ' ' {
				result.WriteRune('_')
			} else {
				result.WriteRune(char)
			}
		} else {
			// Replace any other character with underscore
			result.WriteRune('_')
		}
	}

	// Convert to string
	sanitized := result.String()

	// Remove consecutive underscores
	for strings.Contains(sanitized, "__") {
		sanitized = strings.ReplaceAll(sanitized, "__", "_")
	}

	// Trim underscores from start and end
	sanitized = strings.Trim(sanitized, "_")

	// Limit length (leaving room for extension)
	maxLength := 64 - len(ext)
	if len(sanitized) > maxLength {
		sanitized = sanitized[:maxLength]
	}

	// Ensure not empty
	if sanitized == "" {
		sanitized = "unnamed"
	}

	// Keep the extension as is, since MP3 extensions should be standard
	return sanitized + ext
}

func NormalizePathForDevice(path string) string {
	// Remove any common protocol or drive prefixes
	path = stripPlaylistPathPrefixes(path)

	// Ensure the path starts with a slash if not empty
	if path != "" && !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	return path
}

func stripPlaylistPathPrefixes(path string) string {
	// Normalize path
	path = strings.TrimSpace(path)

	// Remove common prefixes from playlist paths
	prefixes := []string{
		"file:///", "file://", "file:",
		"0:/", "0:", // Common for portable devices
	}

	for _, prefix := range prefixes {
		if strings.HasPrefix(path, prefix) {
			path = path[len(prefix):]
			break
		}
	}

	// Ensure path starts with / for easier handling
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	return path
}

func SanitizeForPath(text string) string {
	// Remove invalid path characters
	text = strings.TrimSpace(text)

	// Replace spaces with underscores
	text = strings.ReplaceAll(text, " ", "_")

	// Remove any other problematic characters
	re := regexp.MustCompile(`[<>:"/\\|?*]`)
	text = re.ReplaceAllString(text, "_")

	return text
}

func FormatPlaylistPath(path string, devicePathStyle int) string {
	// First normalize the path by removing any prefix and ensuring consistency
	path = NormalizePathForDevice(path)

	// Apply device-specific path style
	switch devicePathStyle {
	case 1: // Standard format: 0:/MUSIC/ARTIST/ALBUM/TRACK.MP3 (all uppercase)
		path = strings.ToUpper(path)
		if !strings.HasPrefix(path, "0:") {
			path = "0:/" + strings.TrimPrefix(path, "/")
		}
	case 2: // Alternate format: /MUSIC/ARTIST/ALBUM/TRACK.MP3 (no drive prefix)
		path = strings.ToUpper(path)
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
	case 3: // Relative format: MUSIC/ARTIST/ALBUM/TRACK.MP3
		path = strings.ToUpper(path)
		path = strings.TrimPrefix(path, "/")
		path = strings.TrimPrefix(path, "0:/")
	case 4: // Case-sensitive with drive prefix: 0:/MUSIC/Artist/Album/Track.mp3
		if !strings.HasPrefix(path, "0:") {
			path = "0:/" + strings.TrimPrefix(path, "/")
		}
	default: // Default format (all caps with 0: prefix)
		path = strings.ToUpper(path)
		if !strings.HasPrefix(path, "0:") {
			path = "0:/" + strings.TrimPrefix(path, "/")
		}
	}

	// Replace any double slashes that might have been introduced
	path = strings.ReplaceAll(path, "//", "/")

	LogVerbose("Formatted playlist path to: %s", path)
	return path
}
