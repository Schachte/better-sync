package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/schachte/better-sync/pkg/device"
	"github.com/schachte/better-sync/pkg/operations"
	"github.com/schachte/better-sync/pkg/spotify"
	"github.com/schachte/better-sync/pkg/util"
)

const (
	dbPath = "./spotify.db"
)

func main() {
	verboseFlag := flag.Bool("verbose", false, "Enable verbose logging")
	operationFlag := flag.Int("op", 0, "Operation to perform (0 for menu, 1-10 for specific operation)")
	scanOnlyFlag := flag.Bool("scan", false, "Only scan for MTP devices and exit")
	timeoutSecFlag := flag.Int("timeout", 30, "Timeout in seconds for device initialization")
	flag.Parse()

	util.SetupLogging(*verboseFlag)

	spotifyClient, err := spotify.NewClient(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize Spotify client: %v", err)
	}

	go startHTTPServer(spotifyClient)

	util.LogVerbose("Starting MTP Music Manager")

	timeout := time.Duration(*timeoutSecFlag) * time.Second
	dev, err := device.Initialize(timeout)
	if err != nil {
		util.LogError("Failed to initialize device: %v", err)
		device.CheckForCommonMTPConflicts(err)
		os.Exit(1)
	}
	defer dev.Close()

	if *scanOnlyFlag {
		fmt.Println("MTP device successfully detected. Exiting.")
		os.Exit(0)
	}

	storages, err := device.FetchStorages(dev, timeout)
	if err != nil {
		util.LogError("Failed to fetch storages: %v", err)
		os.Exit(1)
	}

	operations.Execute(dev, storages, *operationFlag)

	util.LogVerbose("Program completed")
}

func getUserFromRequest(r *http.Request, client *spotify.Client) (*spotify.UserAuth, error) {
	cookie, err := r.Cookie("spotify_user_id")
	if err != nil {
		return nil, nil
	}

	userAuth, err := client.DB.GetUserAuth(cookie.Value)
	if err != nil {
		return nil, err
	}

	valid, err := client.DB.IsTokenValid(cookie.Value)
	if err != nil {
		return nil, err
	}

	if !valid {
		return nil, nil
	}

	return userAuth, nil
}

func playlistsHandler(client *spotify.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := getUserFromRequest(r, client)
		if err != nil {
			log.Printf("Error getting user: %v", err)
			http.Redirect(w, r, "/?error=auth", http.StatusFound)
			return
		}

		if user == nil {
			fmt.Println("Redirecting to login")
			http.Redirect(w, r, "/login?redirect=/playlists", http.StatusFound)
			return
		}

		playlists, err := client.GetCompleteLibrary(user.SpotifyID)
		if err != nil {
			log.Printf("Error getting playlists: %v", err)
			http.Error(w, "Failed to get playlists", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(playlists)
	}
}

func makeLogoutHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:     "spotify_user_id",
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			MaxAge:   -1,
		})

		http.Redirect(w, r, "/?logout=success", http.StatusFound)
	}
}

func startHTTPServer(spotifyClient *spotify.Client) {
	http.HandleFunc("/login", spotifyClient.HandleLogin)
	http.HandleFunc("/callback", spotifyClient.HandleCallback)
	http.HandleFunc("/playlists", playlistsHandler(spotifyClient))
	http.HandleFunc("/logout", makeLogoutHandler())

	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server starting on http://localhost:%s", port)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Printf("HTTP server error: %v", err)
	}
}
