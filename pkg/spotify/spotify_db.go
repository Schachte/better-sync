package spotify

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// DBManager handles SQLite database operations for Spotify data
type DBManager struct {
	db *sql.DB
}

// UserAuth represents a user's authentication information
type UserAuth struct {
	ID           int64
	SpotifyID    string
	DisplayName  string
	Email        string
	AccessToken  string
	RefreshToken string
	TokenExpiry  time.Time
}

// Playlist represents a Spotify playlist
// type Playlist struct {
// 	ID          int64
// 	SpotifyID   string
// 	UserID      int64 // Foreign key to users.id
// 	Name        string
// 	Description string
// 	ImageURL    string
// 	TrackCount  int
// 	Public      bool
// 	CreatedAt   time.Time
// 	UpdatedAt   time.Time
// }

// Track represents a Spotify track
// type Track struct {
// 	ID         int64
// 	SpotifyID  string
// 	Name       string
// 	ArtistName string
// 	AlbumName  string
// 	Duration   int // Duration in milliseconds
// 	PreviewURL string
// 	ImageURL   string
// 	AddedAt    time.Time
// }

// PlaylistTrack represents the many-to-many relationship between playlists and tracks
type PlaylistTrack struct {
	PlaylistID int64
	TrackID    int64
	Position   int
	AddedAt    time.Time
}

// NewDBManager creates a new database manager
func NewDBManager(dbPath string) (*DBManager, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	manager := &DBManager{db: db}

	// Initialize the database schema
	if err := manager.InitSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return manager, nil
}

// Close closes the database connection
func (m *DBManager) Close() error {
	return m.db.Close()
}

// InitSchema creates the database tables if they don't exist
func (m *DBManager) InitSchema() error {
	// Create users table
	_, err := m.db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			spotify_id TEXT UNIQUE NOT NULL,
			display_name TEXT,
			email TEXT,
			access_token TEXT NOT NULL,
			refresh_token TEXT NOT NULL,
			token_expiry TIMESTAMP NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create users table: %w", err)
	}

	// Create playlists table
	_, err = m.db.Exec(`
		CREATE TABLE IF NOT EXISTS playlists (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			spotify_id TEXT UNIQUE NOT NULL,
			user_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			image_url TEXT,
			track_count INTEGER DEFAULT 0,
			public BOOLEAN DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create playlists table: %w", err)
	}

	// Create tracks table
	_, err = m.db.Exec(`
		CREATE TABLE IF NOT EXISTS tracks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			spotify_id TEXT UNIQUE NOT NULL,
			name TEXT NOT NULL,
			artist_name TEXT NOT NULL,
			album_name TEXT,
			duration INTEGER,
			preview_url TEXT,
			image_url TEXT,
			added_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create tracks table: %w", err)
	}

	// Create playlist_tracks junction table
	_, err = m.db.Exec(`
		CREATE TABLE IF NOT EXISTS playlist_tracks (
			playlist_id INTEGER NOT NULL,
			track_id INTEGER NOT NULL,
			position INTEGER NOT NULL,
			added_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (playlist_id, track_id),
			FOREIGN KEY (playlist_id) REFERENCES playlists (id) ON DELETE CASCADE,
			FOREIGN KEY (track_id) REFERENCES tracks (id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create playlist_tracks table: %w", err)
	}

	// Create indexes for better performance
	_, err = m.db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_users_spotify_id ON users (spotify_id);
		CREATE INDEX IF NOT EXISTS idx_playlists_user_id ON playlists (user_id);
		CREATE INDEX IF NOT EXISTS idx_playlists_spotify_id ON playlists (spotify_id);
		CREATE INDEX IF NOT EXISTS idx_tracks_spotify_id ON tracks (spotify_id);
		CREATE INDEX IF NOT EXISTS idx_playlist_tracks_track_id ON playlist_tracks (track_id);
	`)
	if err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}

	return nil
}

// SaveUserAuth saves or updates user authentication information
func (m *DBManager) SaveUserAuth(user *UserProfile, token *Token) (*UserAuth, error) {
	// Check if user already exists
	var userAuth UserAuth
	err := m.db.QueryRow(`
		SELECT id, spotify_id, display_name, email, access_token, refresh_token, token_expiry
		FROM users
		WHERE spotify_id = ?
	`, user.ID).Scan(
		&userAuth.ID,
		&userAuth.SpotifyID,
		&userAuth.DisplayName,
		&userAuth.Email,
		&userAuth.AccessToken,
		&userAuth.RefreshToken,
		&userAuth.TokenExpiry,
	)

	// Calculate token expiry
	expiryTime := time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)

	if err == sql.ErrNoRows {
		// User does not exist, insert new record
		result, err := m.db.Exec(`
			INSERT INTO users (spotify_id, display_name, email, access_token, refresh_token, token_expiry)
			VALUES (?, ?, ?, ?, ?, ?)
		`, user.ID, user.DisplayName, user.Email, token.AccessToken, token.RefreshToken, expiryTime)
		if err != nil {
			return nil, fmt.Errorf("failed to insert user: %w", err)
		}

		id, err := result.LastInsertId()
		if err != nil {
			return nil, fmt.Errorf("failed to get last insert ID: %w", err)
		}

		return &UserAuth{
			ID:           id,
			SpotifyID:    user.ID,
			DisplayName:  user.DisplayName,
			Email:        user.Email,
			AccessToken:  token.AccessToken,
			RefreshToken: token.RefreshToken,
			TokenExpiry:  expiryTime,
		}, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	// User exists, update record
	_, err = m.db.Exec(`
		UPDATE users
		SET display_name = ?, email = ?, access_token = ?, refresh_token = ?, token_expiry = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, user.DisplayName, user.Email, token.AccessToken, token.RefreshToken, expiryTime, userAuth.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	return &UserAuth{
		ID:           userAuth.ID,
		SpotifyID:    user.ID,
		DisplayName:  user.DisplayName,
		Email:        user.Email,
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		TokenExpiry:  expiryTime,
	}, nil
}

// GetUserAuth retrieves user authentication information by Spotify ID
func (m *DBManager) GetUserAuth(spotifyID string) (*UserAuth, error) {
	var userAuth UserAuth
	err := m.db.QueryRow(`
		SELECT id, spotify_id, display_name, email, access_token, refresh_token, token_expiry
		FROM users
		WHERE spotify_id = ?
	`, spotifyID).Scan(
		&userAuth.ID,
		&userAuth.SpotifyID,
		&userAuth.DisplayName,
		&userAuth.Email,
		&userAuth.AccessToken,
		&userAuth.RefreshToken,
		&userAuth.TokenExpiry,
	)

	if err == sql.ErrNoRows {
		return nil, nil // User not found
	} else if err != nil {
		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	return &userAuth, nil
}

// IsTokenValid checks if a user's token is still valid
func (m *DBManager) IsTokenValid(spotifyID string) (bool, error) {
	userAuth, err := m.GetUserAuth(spotifyID)
	if err != nil {
		return false, err
	}
	if userAuth == nil {
		return false, nil
	}

	// Token is valid if it hasn't expired yet
	return time.Now().Before(userAuth.TokenExpiry), nil
}

// // SavePlaylist saves a playlist to the database
// // func (m *DBManager) SavePlaylist(userID int64, spotifyPlaylist map[string]interface{}) (*Playlist, error) {
// // // Extract playlist info from Spotify API response
// // spotifyID := spotifyPlaylist["id"].(string)
// // name := spotifyPlaylist["name"].(string)

// // // Get description (may be nil)
// // var description string
// // if desc, ok := spotifyPlaylist["description"]; ok && desc != nil {
// // 	description = desc.(string)
// // }

// // // Get image URL if available
// // var imageURL string
// // if images, ok := spotifyPlaylist["images"].([]interface{}); ok && len(images) > 0 {
// // 	if imageObj, ok := images[0].(map[string]interface{}); ok {
// // 		imageURL = imageObj["url"].(string)
// // 	}
// // }

// // // Get track count
// // var trackCount int
// // if tracks, ok := spotifyPlaylist["tracks"].(map[string]interface{}); ok {
// // 	if total, ok := tracks["total"]; ok {
// // 		trackCount = int(total.(float64))
// // 	}
// // }

// // // Get public status
// // isPublic := false
// // if pub, ok := spotifyPlaylist["public"].(bool); ok {
// // 	isPublic = pub
// // }

// // // Check if the playlist already exists
// // var playlistID int64
// // err := m.db.QueryRow(`
// // 	SELECT id FROM playlists WHERE spotify_id = ?
// // `, spotifyID).Scan(&playlistID)

// // if err == sql.ErrNoRows {
// // 	// Playlist doesn't exist, insert it
// // 	result, err := m.db.Exec(`
// // 		INSERT INTO playlists (spotify_id, user_id, name, description, image_url, track_count, public)
// // 		VALUES (?, ?, ?, ?, ?, ?, ?)
// // 	`, spotifyID, userID, name, description, imageURL, trackCount, isPublic)
// // 	if err != nil {
// // 		return nil, fmt.Errorf("failed to insert playlist: %w", err)
// // 	}

// // 	playlistID, err = result.LastInsertId()
// // 	if err != nil {
// // 		return nil, fmt.Errorf("failed to get last insert ID: %w", err)
// // 	}
// // } else if err != nil {
// // 	return nil, fmt.Errorf("failed to query playlist: %w", err)
// // } else {
// // 	// Playlist exists, update it
// // 	_, err = m.db.Exec(`
// // 		UPDATE playlists
// // 		SET name = ?, description = ?, image_url = ?, track_count = ?, public = ?, updated_at = CURRENT_TIMESTAMP
// // 		WHERE id = ?
// // 	`, name, description, imageURL, trackCount, isPublic, playlistID)
// // 	if err != nil {
// // 		return nil, fmt.Errorf("failed to update playlist: %w", err)
// // 	}
// // }

// // // Return the playlist
// // return m.GetPlaylistByID(playlistID)
// // }

// // // GetPlaylistByID retrieves a playlist by its database ID
// // func (m *DBManager) GetPlaylistByID(id int64) (*Playlist, error) {
// // 	var playlist Playlist
// // 	err := m.db.QueryRow(`
// // 		SELECT id, spotify_id, user_id, name, description, image_url, track_count, public, created_at, updated_at
// // 		FROM playlists
// // 		WHERE id = ?
// // 	`, id).Scan(
// // 		&playlist.ID,
// // 		&playlist.SpotifyID,
// // 		&playlist.UserID,
// // 		&playlist.Name,
// // 		&playlist.Description,
// // 		&playlist.ImageURL,
// // 		&playlist.TrackCount,
// // 		&playlist.Public,
// // 		&playlist.CreatedAt,
// // 		&playlist.UpdatedAt,
// // 	)

// // 	if err == sql.ErrNoRows {
// // 		return nil, nil // Playlist not found
// // 	} else if err != nil {
// // 		return nil, fmt.Errorf("failed to query playlist: %w", err)
// // 	}

// // 	return &playlist, nil
// // }

// // GetUserPlaylists retrieves all playlists for a user
// func (m *DBManager) GetUserPlaylists(userID int64) ([]Playlist, error) {
// 	rows, err := m.db.Query(`
// 		SELECT id, spotify_id, user_id, name, description, image_url, track_count, public, created_at, updated_at
// 		FROM playlists
// 		WHERE user_id = ?
// 		ORDER BY name
// 	`, userID)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to query playlists: %w", err)
// 	}
// 	defer rows.Close()

// 	var playlists []Playlist
// 	for rows.Next() {
// 		var playlist Playlist
// 		err := rows.Scan(
// 			&playlist.ID,
// 			&playlist.SpotifyID,
// 			&playlist.UserID,
// 			&playlist.Name,
// 			&playlist.Description,
// 			&playlist.ImageURL,
// 			&playlist.TrackCount,
// 			&playlist.Public,
// 			&playlist.CreatedAt,
// 			&playlist.UpdatedAt,
// 		)
// 		if err != nil {
// 			return nil, fmt.Errorf("failed to scan playlist row: %w", err)
// 		}
// 		playlists = append(playlists, playlist)
// 	}

// 	if err := rows.Err(); err != nil {
// 		return nil, fmt.Errorf("error after iterating playlist rows: %w", err)
// 	}

// 	return playlists, nil
// }

// // SaveTrack saves a track to the database
// func (m *DBManager) SaveTrack(spotifyTrack map[string]interface{}) (*Track, error) {
// 	// Extract track info from Spotify API response
// 	spotifyID := spotifyTrack["id"].(string)
// 	name := spotifyTrack["name"].(string)

// 	// Get artist name
// 	var artistName string
// 	if artists, ok := spotifyTrack["artists"].([]interface{}); ok && len(artists) > 0 {
// 		if artist, ok := artists[0].(map[string]interface{}); ok {
// 			artistName = artist["name"].(string)
// 		}
// 	}

// 	// Get album name and image URL
// 	var albumName, imageURL string
// 	if album, ok := spotifyTrack["album"].(map[string]interface{}); ok {
// 		if albumObj, ok := album["name"]; ok {
// 			albumName = albumObj.(string)
// 		}
// 		if images, ok := album["images"].([]interface{}); ok && len(images) > 0 {
// 			if imageObj, ok := images[0].(map[string]interface{}); ok {
// 				imageURL = imageObj["url"].(string)
// 			}
// 		}
// 	}

// 	// Get duration and preview URL
// 	var duration int
// 	var previewURL string
// 	if durationMs, ok := spotifyTrack["duration_ms"]; ok {
// 		duration = int(durationMs.(float64))
// 	}
// 	if preview, ok := spotifyTrack["preview_url"]; ok && preview != nil {
// 		previewURL = preview.(string)
// 	}

// 	// Check if the track already exists
// 	var trackID int64
// 	err := m.db.QueryRow(`
// 		SELECT id FROM tracks WHERE spotify_id = ?
// 	`, spotifyID).Scan(&trackID)

// 	if err == sql.ErrNoRows {
// 		// Track doesn't exist, insert it
// 		result, err := m.db.Exec(`
// 			INSERT INTO tracks (spotify_id, name, artist_name, album_name, duration, preview_url, image_url)
// 			VALUES (?, ?, ?, ?, ?, ?, ?)
// 		`, spotifyID, name, artistName, albumName, duration, previewURL, imageURL)
// 		if err != nil {
// 			return nil, fmt.Errorf("failed to insert track: %w", err)
// 		}

// 		trackID, err = result.LastInsertId()
// 		if err != nil {
// 			return nil, fmt.Errorf("failed to get last insert ID: %w", err)
// 		}
// 	} else if err != nil {
// 		return nil, fmt.Errorf("failed to query track: %w", err)
// 	} else {
// 		// Track exists, update it
// 		_, err = m.db.Exec(`
// 			UPDATE tracks
// 			SET name = ?, artist_name = ?, album_name = ?, duration = ?, preview_url = ?, image_url = ?
// 			WHERE id = ?
// 		`, name, artistName, albumName, duration, previewURL, imageURL, trackID)
// 		if err != nil {
// 			return nil, fmt.Errorf("failed to update track: %w", err)
// 		}
// 	}

// 	// Return the track
// 	return m.GetTrackByID(trackID)
// }

// // GetTrackByID retrieves a track by its database ID
// func (m *DBManager) GetTrackByID(id int64) (*Track, error) {
// 	var track Track
// 	err := m.db.QueryRow(`
// 		SELECT id, spotify_id, name, artist_name, album_name, duration, preview_url, image_url, added_at
// 		FROM tracks
// 		WHERE id = ?
// 	`, id).Scan(
// 		&track.ID,
// 		&track.SpotifyID,
// 		&track.Name,
// 		&track.ArtistName,
// 		&track.AlbumName,
// 		&track.Duration,
// 		&track.PreviewURL,
// 		&track.ImageURL,
// 		&track.AddedAt,
// 	)

// 	if err == sql.ErrNoRows {
// 		return nil, nil // Track not found
// 	} else if err != nil {
// 		return nil, fmt.Errorf("failed to query track: %w", err)
// 	}

// 	return &track, nil
// }

// // AddTrackToPlaylist adds a track to a playlist
// func (m *DBManager) AddTrackToPlaylist(playlistID, trackID int64, position int) error {
// 	// Check if the relationship already exists
// 	var exists int
// 	err := m.db.QueryRow(`
// 		SELECT 1 FROM playlist_tracks
// 		WHERE playlist_id = ? AND track_id = ?
// 	`, playlistID, trackID).Scan(&exists)

// 	if err == sql.ErrNoRows {
// 		// Relationship doesn't exist, create it
// 		_, err := m.db.Exec(`
// 			INSERT INTO playlist_tracks (playlist_id, track_id, position)
// 			VALUES (?, ?, ?)
// 		`, playlistID, trackID, position)
// 		if err != nil {
// 			return fmt.Errorf("failed to insert playlist track: %w", err)
// 		}
// 	} else if err != nil {
// 		return fmt.Errorf("failed to query playlist track: %w", err)
// 	} else {
// 		// Relationship exists, update position
// 		_, err := m.db.Exec(`
// 			UPDATE playlist_tracks
// 			SET position = ?
// 			WHERE playlist_id = ? AND track_id = ?
// 		`, position, playlistID, trackID)
// 		if err != nil {
// 			return fmt.Errorf("failed to update playlist track position: %w", err)
// 		}
// 	}

// 	// Update the track count in the playlist
// 	return m.updatePlaylistTrackCount(playlistID)
// }

// // RemoveTrackFromPlaylist removes a track from a playlist
// func (m *DBManager) RemoveTrackFromPlaylist(playlistID, trackID int64) error {
// 	_, err := m.db.Exec(`
// 		DELETE FROM playlist_tracks
// 		WHERE playlist_id = ? AND track_id = ?
// 	`, playlistID, trackID)
// 	if err != nil {
// 		return fmt.Errorf("failed to delete playlist track: %w", err)
// 	}

// 	// Update the track count in the playlist
// 	return m.updatePlaylistTrackCount(playlistID)
// }

// // GetPlaylistTracks retrieves all tracks in a playlist
// func (m *DBManager) GetPlaylistTracks(playlistID int64) ([]Track, error) {
// 	rows, err := m.db.Query(`
// 		SELECT t.id, t.spotify_id, t.name, t.artist_name, t.album_name, t.duration, t.preview_url, t.image_url, t.added_at
// 		FROM tracks t
// 		JOIN playlist_tracks pt ON t.id = pt.track_id
// 		WHERE pt.playlist_id = ?
// 		ORDER BY pt.position
// 	`, playlistID)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to query playlist tracks: %w", err)
// 	}
// 	defer rows.Close()

// 	var tracks []Track
// 	for rows.Next() {
// 		var track Track
// 		err := rows.Scan(
// 			&track.ID,
// 			&track.SpotifyID,
// 			&track.Name,
// 			&track.ArtistName,
// 			&track.AlbumName,
// 			&track.Duration,
// 			&track.PreviewURL,
// 			&track.ImageURL,
// 			&track.AddedAt,
// 		)
// 		if err != nil {
// 			return nil, fmt.Errorf("failed to scan track row: %w", err)
// 		}
// 		tracks = append(tracks, track)
// 	}

// 	if err := rows.Err(); err != nil {
// 		return nil, fmt.Errorf("error after iterating track rows: %w", err)
// 	}

// 	return tracks, nil
// }

// // updatePlaylistTrackCount updates the track count in a playlist
// func (m *DBManager) updatePlaylistTrackCount(playlistID int64) error {
// 	_, err := m.db.Exec(`
// 		UPDATE playlists
// 		SET track_count = (
// 			SELECT COUNT(*) FROM playlist_tracks
// 			WHERE playlist_id = ?
// 		),
// 		updated_at = CURRENT_TIMESTAMP
// 		WHERE id = ?
// 	`, playlistID, playlistID)

// 	if err != nil {
// 		return fmt.Errorf("failed to update playlist track count: %w", err)
// 	}

// 	return nil
// }

// // DeletePlaylist deletes a playlist and its track associations
// func (m *DBManager) DeletePlaylist(playlistID int64) error {
// 	// Start a transaction
// 	tx, err := m.db.Begin()
// 	if err != nil {
// 		return fmt.Errorf("failed to begin transaction: %w", err)
// 	}

// 	// Delete playlist tracks first (due to foreign key constraints)
// 	_, err = tx.Exec(`DELETE FROM playlist_tracks WHERE playlist_id = ?`, playlistID)
// 	if err != nil {
// 		tx.Rollback()
// 		return fmt.Errorf("failed to delete playlist tracks: %w", err)
// 	}

// 	// Delete the playlist
// 	_, err = tx.Exec(`DELETE FROM playlists WHERE id = ?`, playlistID)
// 	if err != nil {
// 		tx.Rollback()
// 		return fmt.Errorf("failed to delete playlist: %w", err)
// 	}

// 	// Commit the transaction
// 	if err := tx.Commit(); err != nil {
// 		return fmt.Errorf("failed to commit transaction: %w", err)
// 	}

// 	return nil
// }

// // SearchTracks searches for tracks in the database
// func (m *DBManager) SearchTracks(query string) ([]Track, error) {
// 	// Use LIKE for search with wildcard
// 	searchQuery := "%" + query + "%"

// 	rows, err := m.db.Query(`
// 		SELECT id, spotify_id, name, artist_name, album_name, duration, preview_url, image_url, added_at
// 		FROM tracks
// 		WHERE name LIKE ? OR artist_name LIKE ? OR album_name LIKE ?
// 		ORDER BY name
// 		LIMIT 50
// 	`, searchQuery, searchQuery, searchQuery)
