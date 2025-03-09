package util

import (
	"path/filepath"
	"regexp"
	"strings"
)

func SanitizeFolderName(name string) string {

	allowedSpecialChars := map[rune]bool{
		'!':  true,
		'_':  true,
		'-':  true,
		' ':  true,
		'&':  true,
		'(':  true,
		')':  true,
		'+':  true,
		'.':  true,
		'\'': true,
	}

	var result strings.Builder

	for _, char := range name {

		if (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') ||
			(char >= '0' && char <= '9') || allowedSpecialChars[char] {

			if char == ' ' {
				result.WriteRune('_')
			} else {
				result.WriteRune(char)
			}
		} else {

			result.WriteRune('_')
		}
	}

	sanitized := result.String()

	for strings.Contains(sanitized, "__") {
		sanitized = strings.ReplaceAll(sanitized, "__", "_")
	}

	sanitized = strings.Trim(sanitized, "_")

	if len(sanitized) > 64 {
		sanitized = sanitized[:64]
	}

	if sanitized == "" {
		sanitized = "unnamed"
	}

	return sanitized
}

func SanitizeFileName(name string) string {

	if name == "" {
		return "unnamed.mp3"
	}

	ext := filepath.Ext(name)
	var baseName string

	if len(name) > len(ext) {
		baseName = name[:len(name)-len(ext)]
	} else {
		baseName = ""
	}

	allowedSpecialChars := map[rune]bool{
		'!':  true,
		'_':  true,
		'-':  true,
		' ':  true,
		'&':  true,
		'(':  true,
		')':  true,
		'+':  true,
		'.':  true,
		'\'': true,
	}

	var result strings.Builder

	for _, char := range baseName {

		if (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') ||
			(char >= '0' && char <= '9') || allowedSpecialChars[char] {

			if char == ' ' {
				result.WriteRune('_')
			} else {
				result.WriteRune(char)
			}
		} else {

			result.WriteRune('_')
		}
	}

	sanitized := result.String()

	for strings.Contains(sanitized, "__") {
		sanitized = strings.ReplaceAll(sanitized, "__", "_")
	}

	sanitized = strings.Trim(sanitized, "_")

	maxLength := 64 - len(ext)
	if len(sanitized) > maxLength {
		sanitized = sanitized[:maxLength]
	}

	if sanitized == "" {
		sanitized = "unnamed"
	}

	return sanitized + ext
}

func NormalizePathForDevice(path string) string {

	path = stripPlaylistPathPrefixes(path)

	if path != "" && !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	return path
}

func stripPlaylistPathPrefixes(path string) string {

	path = strings.TrimSpace(path)

	prefixes := []string{
		"file:///", "file://", "file:",
		"0:/", "0:",
	}

	for _, prefix := range prefixes {
		if strings.HasPrefix(path, prefix) {
			path = path[len(prefix):]
			break
		}
	}

	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	return path
}

func SanitizeForPath(text string) string {

	text = strings.TrimSpace(text)

	text = strings.ReplaceAll(text, " ", "_")

	re := regexp.MustCompile(`[<>:"/\\|?*]`)
	text = re.ReplaceAllString(text, "_")

	return text
}

func FormatPlaylistPath(path string, devicePathStyle int) string {

	path = NormalizePathForDevice(path)

	switch devicePathStyle {
	case 1:
		path = strings.ToUpper(path)
		if !strings.HasPrefix(path, "0:") {
			path = "0:/" + strings.TrimPrefix(path, "/")
		}
	case 2:
		path = strings.ToUpper(path)
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
	case 3:
		path = strings.ToUpper(path)
		path = strings.TrimPrefix(path, "/")
		path = strings.TrimPrefix(path, "0:/")
	case 4:
		if !strings.HasPrefix(path, "0:") {
			path = "0:/" + strings.TrimPrefix(path, "/")
		}
	default:
		path = strings.ToUpper(path)
		if !strings.HasPrefix(path, "0:") {
			path = "0:/" + strings.TrimPrefix(path, "/")
		}
	}

	path = strings.ReplaceAll(path, "//", "/")

	LogVerbose("Formatted playlist path to: %s", path)
	return path
}
