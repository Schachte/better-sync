package util

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/joho/godotenv"
)

type SpotifyTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type SpotifyPlaylist struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Id          string `json:"id"`
}

func GetSpotifyAccessToken() (string, error) {
	err := godotenv.Load()
	if err != nil {
		return "", fmt.Errorf("error loading .env file: %v", err)
	}

	clientID := os.Getenv("SPOTIFY_CLIENT_ID")
	clientSecret := os.Getenv("SPOTIFY_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		return "", fmt.Errorf("SPOTIFY_CLIENT_ID or SPOTIFY_CLIENT_SECRET not set in .env file")
	}

	auth := base64.StdEncoding.EncodeToString([]byte(clientID + ":" + clientSecret))

	data := url.Values{}
	data.Set("grant_type", "client_credentials")

	req, err := http.NewRequest("POST", "https://accounts.spotify.com/api/token", strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %v", err)
	}

	var tokenResponse SpotifyTokenResponse
	err = json.Unmarshal(body, &tokenResponse)
	if err != nil {
		return "", fmt.Errorf("error parsing token response: %v", err)
	}

	return tokenResponse.AccessToken, nil
}

func ExtractPlaylistID(input string) (string, error) {
	if len(input) == 22 && !strings.Contains(input, "/") {
		return input, nil
	}

	patterns := []string{
		`spotify\.com/playlist/([a-zA-Z0-9]+)`,
		`spotify:playlist:([a-zA-Z0-9]+)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(input)
		if len(matches) > 1 {
			return matches[1], nil
		}
	}

	return "", fmt.Errorf("could not extract playlist ID from input: %s", input)
}

func GetSpotifyPlaylistName(input string) (string, error) {
	playlistID, err := ExtractPlaylistID(input)
	if err != nil {
		return "", err
	}
	LogVerbose("Extracted Playlist ID: %s", playlistID)

	token, err := GetSpotifyAccessToken()
	if err != nil {
		return "", fmt.Errorf("error getting access token: %v", err)
	}

	LogVerbose("Successfully obtained access token")

	url := fmt.Sprintf("https://api.spotify.com/v1/playlists/%s", playlistID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("error creating playlist request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error sending playlist request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading playlist response: %v", err)
	}

	var playlist SpotifyPlaylist
	err = json.Unmarshal(body, &playlist)
	if err != nil {
		return "", fmt.Errorf("error parsing playlist response: %v", err)
	}

	return playlist.Name, nil
}
