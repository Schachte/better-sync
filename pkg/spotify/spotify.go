package spotify

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

const (
	redirectURI = "http://localhost:8080/callback"
	authURL     = "https://accounts.spotify.com/authorize"
	tokenURL    = "https://accounts.spotify.com/api/token"
	// Full scope to read all available Spotify data
	scope = "user-read-private user-read-email playlist-read-private playlist-read-collaborative user-library-read user-top-read user-read-currently-playing user-read-recently-played user-read-playback-state user-read-playback-position streaming user-follow-read"
)

type Client struct {
	ClientID     string
	CodeVerifier string
	State        string
	DB           *DBManager
}

type Token struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

type UserProfile struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Country     string `json:"country"`
	Images      []struct {
		URL string `json:"url"`
	} `json:"images"`
}

type Playlist struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Public      bool   `json:"public"`
	Images      []struct {
		URL string `json:"url"`
	} `json:"images"`
	Tracks struct {
		Total int `json:"total"`
	} `json:"tracks"`
	Owner struct {
		ID   string `json:"id"`
		Name string `json:"display_name,omitempty"`
	} `json:"owner"`
	// Added to identify source in combined results
	Source string `json:"-"`
}

// New response types for additional endpoints
type PagingObject struct {
	Href     string      `json:"href"`
	Items    interface{} `json:"items"`
	Limit    int         `json:"limit"`
	Next     string      `json:"next"`
	Offset   int         `json:"offset"`
	Previous string      `json:"previous"`
	Total    int         `json:"total"`
}

type PlaylistsResponse struct {
	Playlists PagingObject `json:"playlists"`
}

type SavedAlbum struct {
	Album struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Images []struct {
			URL string `json:"url"`
		} `json:"images"`
		Artists []struct {
			Name string `json:"name"`
		} `json:"artists"`
		Tracks struct {
			Total int `json:"total"`
		} `json:"tracks"`
	} `json:"album"`
}

type Track struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Album struct {
		Name string `json:"name"`
	} `json:"album"`
	Artists []struct {
		Name string `json:"name"`
	} `json:"artists"`
}

// AllPlaylistsResponse combines results from multiple endpoints
type AllPlaylistsResponse struct {
	UserPlaylists     []Playlist   `json:"user_playlists"`
	FollowedPlaylists []Playlist   `json:"followed_playlists"`
	FeaturedPlaylists []Playlist   `json:"featured_playlists"`
	TopSongsPlaylists []Playlist   `json:"top_songs_playlists"`
	SavedAlbums       []SavedAlbum `json:"saved_albums"`
	TopTracks         []Track      `json:"top_tracks"`
}

func NewClient(dbPath string) (*Client, error) {
	state, err := generateRandomString(16)
	if err != nil {
		state = "random_state"
	}

	dbManager, err := NewDBManager(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database manager: %w", err)
	}

	return &Client{
		ClientID: os.Getenv("SPOTIFY_CLIENT_ID"),
		State:    state,
		DB:       dbManager,
	}, nil
}

func generateRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes)[:length], nil
}

func generateCodeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

func (c *Client) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var err error
	c.CodeVerifier, err = generateRandomString(64)
	if err != nil {
		http.Error(w, "Failed to generate code verifier", http.StatusInternalServerError)
		return
	}

	codeChallenge := generateCodeChallenge(c.CodeVerifier)

	// Get the redirect path from query parameters
	redirectPath := r.URL.Query().Get("redirect")

	// Create a state that includes both random string and redirect path
	stateValue := c.State
	if redirectPath != "" {
		// Encode the redirect path and append it to the state
		encodedRedirect := base64.URLEncoding.EncodeToString([]byte(redirectPath))
		stateValue = fmt.Sprintf("%s:%s", c.State, encodedRedirect)
	}

	u, err := url.Parse(authURL)
	if err != nil {
		http.Error(w, "Failed to parse auth URL", http.StatusInternalServerError)
		return
	}

	q := u.Query()
	q.Add("client_id", c.ClientID)
	q.Add("response_type", "code")
	q.Add("redirect_uri", redirectURI)
	q.Add("scope", scope)
	q.Add("state", stateValue)
	q.Add("code_challenge_method", "S256")
	q.Add("code_challenge", codeChallenge)
	u.RawQuery = q.Encode()

	http.Redirect(w, r, u.String(), http.StatusFound)
}

func (c *Client) HandleCallback(w http.ResponseWriter, r *http.Request) {
	stateParam := r.URL.Query().Get("state")
	stateParts := strings.Split(stateParam, ":")

	// Extract the base state and redirect path
	receivedState := stateParts[0]
	var redirectPath string

	if len(stateParts) > 1 {
		// Decode the redirect path if it exists
		decodedBytes, err := base64.URLEncoding.DecodeString(stateParts[1])
		if err == nil {
			redirectPath = string(decodedBytes)
		}
	}

	// Verify the base state matches
	if receivedState != c.State {
		http.Error(w, "State mismatch", http.StatusBadRequest)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Code not found", http.StatusBadRequest)
		return
	}

	token, err := c.ExchangeCodeForToken(code)
	if err != nil {
		http.Error(w, "Failed to exchange code: "+err.Error(), http.StatusInternalServerError)
		return
	}

	userProfile, err := c.GetUserProfile(token.AccessToken)
	if err != nil {
		http.Error(w, "Failed to get user profile: "+err.Error(), http.StatusInternalServerError)
		return
	}

	userAuth, err := c.DB.SaveUserAuth(userProfile, token)
	if err != nil {
		http.Error(w, "Failed to save user data: "+err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Println("User authenticated:", userAuth)
	http.SetCookie(w, &http.Cookie{
		Name:     "spotify_user_id",
		Value:    userProfile.ID,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   3600 * 24 * 30,
		SameSite: http.SameSiteLaxMode,
	})

	fmt.Println("Redirect path from state:", redirectPath)
	if redirectPath != "" {
		http.Redirect(w, r, redirectPath, http.StatusFound)
		return
	}

	http.Redirect(w, r, "/?login=success", http.StatusFound)
}

func (c *Client) ExchangeCodeForToken(code string) (*Token, error) {
	data := url.Values{}
	data.Set("client_id", c.ClientID)
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", redirectURI)
	data.Set("code_verifier", c.CodeVerifier)

	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bad response: %d %s", resp.StatusCode, string(bodyBytes))
	}

	var token Token
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, err
	}

	return &token, nil
}

func (c *Client) GetUserProfile(accessToken string) (*UserProfile, error) {
	req, err := http.NewRequest("GET", "https://api.spotify.com/v1/me", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bad response: %d %s", resp.StatusCode, string(bodyBytes))
	}

	var profile UserProfile
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return nil, err
	}

	return &profile, nil
}

func GetLoginHTML() string {
	return `
	<!DOCTYPE html>
	<html lang="en">
	<head>
		<meta charset="UTF-8">
		<title>Spotify OAuth PKCE Example</title>
		<style>
			body { font-family: Arial, sans-serif; text-align: center; margin-top: 50px; }
			a { display: inline-block; background: #1DB954; color: white; padding: 10px 20px; 
				text-decoration: none; border-radius: 24px; font-weight: bold; }
		</style>
	</head>
	<body>
		<h1>Spotify OAuth with PKCE</h1>
		<p>Click below to log in with Spotify</p>
		<a href="/login">Connect to Spotify</a>
	</body>
	</html>
	`
}

// Original method - renamed to distinguish from enhanced version
func (c *Client) GetUserLibrary(userID string) ([]Playlist, error) {
	userAuth, err := c.DB.GetUserAuth(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user auth: %w", err)
	}

	req, err := http.NewRequest("GET", "https://api.spotify.com/v1/me/playlists", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+userAuth.AccessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Print raw response body for debugging
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	fmt.Printf("Raw response body: %s\n", string(bodyBytes))

	// Reset the response body for subsequent reads
	resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bad response: %d %s", resp.StatusCode, string(bodyBytes))
	}

	var playlistResponse struct {
		Items []Playlist `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&playlistResponse); err != nil {
		return nil, err
	}

	// Filter to only include playlists not owned by the user
	var savedPlaylists []Playlist
	for _, playlist := range playlistResponse.Items {
		if playlist.Owner.ID != userID {
			savedPlaylists = append(savedPlaylists, playlist)
		}
	}

	return savedPlaylists, nil
}

func (c *Client) GetUserPlaylists(userID string) ([]Playlist, error) {
	userAuth, err := c.DB.GetUserAuth(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user auth: %w", err)
	}

	reqURL := fmt.Sprintf("https://api.spotify.com/v1/users/%s/playlists", userID)
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+userAuth.AccessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bad response: %d %s", resp.StatusCode, string(bodyBytes))
	}

	var playlistResponse struct {
		Items []Playlist `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&playlistResponse); err != nil {
		return nil, err
	}

	return playlistResponse.Items, nil
}

// New methods for enhanced playlist retrieval

// GetAllUserPlaylists retrieves user-created playlists with pagination support
func (c *Client) GetAllUserPlaylists(userID string, limit int) ([]Playlist, error) {
	userAuth, err := c.DB.GetUserAuth(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user auth: %w", err)
	}

	var allPlaylists []Playlist
	offset := 0
	hasMore := true

	for hasMore {
		reqURL := fmt.Sprintf("https://api.spotify.com/v1/me/playlists?limit=%d&offset=%d", limit, offset)
		req, err := http.NewRequest("GET", reqURL, nil)
		if err != nil {
			return nil, err
		}

		req.Header.Set("Authorization", "Bearer "+userAuth.AccessToken)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("bad response: %d %s", resp.StatusCode, string(bodyBytes))
		}

		var playlistResponse struct {
			Items []Playlist `json:"items"`
			Next  string     `json:"next"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&playlistResponse); err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()

		// Add source identifier
		for i := range playlistResponse.Items {
			playlistResponse.Items[i].Source = "user_playlists"
		}

		allPlaylists = append(allPlaylists, playlistResponse.Items...)

		if playlistResponse.Next == "" {
			hasMore = false
		} else {
			offset += limit
		}
	}

	return allPlaylists, nil
}

// GetFollowedPlaylists retrieves playlists the user is following
func (c *Client) GetFollowedPlaylists(userID string) ([]Playlist, error) {
	userAuth, err := c.DB.GetUserAuth(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user auth: %w", err)
	}
	fmt.Println("User ID:", userID)
	fmt.Println("User Auth:", userAuth)

	userPlaylists, err := c.GetAllUserPlaylists(userID, 50)
	if err != nil {
		return nil, err
	}

	// Filter to only include playlists not owned by the user
	var followedPlaylists []Playlist
	for _, playlist := range userPlaylists {
		if playlist.Owner.ID != userID {
			playlist.Source = "followed_playlists"
			followedPlaylists = append(followedPlaylists, playlist)
		}
	}

	return followedPlaylists, nil
}

// GetFeaturedPlaylists retrieves Spotify's featured playlists
func (c *Client) GetFeaturedPlaylists(userID string) ([]Playlist, error) {
	userAuth, err := c.DB.GetUserAuth(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user auth: %w", err)
	}

	reqURL := "https://api.spotify.com/v1/browse/featured-playlists?limit=50"
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+userAuth.AccessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bad response: %d %s", resp.StatusCode, string(bodyBytes))
	}

	var featuredResponse PlaylistsResponse
	if err := json.NewDecoder(resp.Body).Decode(&featuredResponse); err != nil {
		return nil, err
	}

	// Convert the items to Playlist type
	items, ok := featuredResponse.Playlists.Items.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected items type in featured playlists response")
	}

	var playlists []Playlist
	for _, item := range items {
		// Convert item to map
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		// Convert map to JSON
		jsonData, err := json.Marshal(itemMap)
		if err != nil {
			continue
		}

		// Unmarshal JSON to Playlist struct
		var playlist Playlist
		if err := json.Unmarshal(jsonData, &playlist); err != nil {
			continue
		}

		playlist.Source = "featured_playlists"
		playlists = append(playlists, playlist)
	}

	return playlists, nil
}

// SearchForPlaylists searches for specific playlists like "Top Songs"
func (c *Client) SearchForPlaylists(userID, query string) ([]Playlist, error) {
	userAuth, err := c.DB.GetUserAuth(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user auth: %w", err)
	}

	// URL encode the query
	queryParams := url.Values{}
	queryParams.Add("q", query)
	queryParams.Add("type", "playlist")
	queryParams.Add("limit", "50")

	reqURL := "https://api.spotify.com/v1/search?" + queryParams.Encode()
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+userAuth.AccessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bad response: %d %s", resp.StatusCode, string(bodyBytes))
	}

	var searchResponse struct {
		Playlists struct {
			Items []Playlist `json:"items"`
		} `json:"playlists"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&searchResponse); err != nil {
		return nil, err
	}

	// Add source identifier
	for i := range searchResponse.Playlists.Items {
		searchResponse.Playlists.Items[i].Source = "search_results"
	}

	return searchResponse.Playlists.Items, nil
}

// GetSavedAlbums retrieves user's saved albums
func (c *Client) GetSavedAlbums(userID string) ([]SavedAlbum, error) {
	userAuth, err := c.DB.GetUserAuth(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user auth: %w", err)
	}

	reqURL := "https://api.spotify.com/v1/me/albums?limit=50"
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+userAuth.AccessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bad response: %d %s", resp.StatusCode, string(bodyBytes))
	}

	var albumsResponse struct {
		Items []SavedAlbum `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&albumsResponse); err != nil {
		return nil, err
	}

	return albumsResponse.Items, nil
}

// GetTopTracks retrieves user's personalized top tracks
func (c *Client) GetTopTracks(userID string) ([]Track, error) {
	userAuth, err := c.DB.GetUserAuth(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user auth: %w", err)
	}

	reqURL := "https://api.spotify.com/v1/me/top/tracks?limit=50&time_range=medium_term"
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+userAuth.AccessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bad response: %d %s", resp.StatusCode, string(bodyBytes))
	}

	var tracksResponse struct {
		Items []Track `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tracksResponse); err != nil {
		return nil, err
	}

	return tracksResponse.Items, nil
}

// GetCompleteLibrary retrieves all playlist types including Top Songs
func (c *Client) GetCompleteLibrary(userID string) (*AllPlaylistsResponse, error) {
	result := &AllPlaylistsResponse{}
	var err error

	// Get user-created playlists
	result.UserPlaylists, err = c.GetAllUserPlaylists(userID, 50)
	if err != nil {
		fmt.Printf("Error getting user playlists: %v\n", err)
		// Continue with other requests even if this one fails
	}

	// Get followed playlists
	result.FollowedPlaylists, err = c.GetFollowedPlaylists(userID)
	if err != nil {
		fmt.Printf("Error getting followed playlists: %v\n", err)
	}

	// Get featured playlists
	result.FeaturedPlaylists, err = c.GetFeaturedPlaylists(userID)
	if err != nil {
		fmt.Printf("Error getting featured playlists: %v\n", err)
	}

	// Search for Top Songs playlists
	result.TopSongsPlaylists, err = c.SearchForPlaylists(userID, "Your Top Songs")
	if err != nil {
		fmt.Printf("Error searching for Top Songs playlists: %v\n", err)
	}

	// Get saved albums
	result.SavedAlbums, err = c.GetSavedAlbums(userID)
	if err != nil {
		fmt.Printf("Error getting saved albums: %v\n", err)
	}

	// Get top tracks
	result.TopTracks, err = c.GetTopTracks(userID)
	if err != nil {
		fmt.Printf("Error getting top tracks: %v\n", err)
	}

	return result, nil
}

// Handler for accessing the complete library
func (c *Client) HandleCompleteLibrary(w http.ResponseWriter, r *http.Request) {
	// Get user ID from cookie
	cookie, err := r.Cookie("spotify_user_id")
	if err != nil {
		http.Error(w, "User not logged in", http.StatusUnauthorized)
		return
	}
	userID := cookie.Value

	// Get complete library
	library, err := c.GetCompleteLibrary(userID)
	if err != nil {
		http.Error(w, "Failed to get complete library: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return as JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(library)
}
