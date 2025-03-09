package operations

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/ganeshrvel/go-mtpfs/mtp"
	"github.com/schachte/better-sync/pkg/model"
	"github.com/schachte/better-sync/pkg/util"
)

func ShowMenu() int {
	fmt.Print("\033[H\033[2J")

	titleColor := color.New(color.FgHiCyan, color.Bold).SprintFunc()
	sectionColor := color.New(color.FgHiYellow).SprintFunc()
	optionColor := color.New(color.FgHiWhite).SprintFunc()
	numberColor := color.New(color.FgHiGreen, color.Bold).SprintFunc()

	fmt.Println(titleColor("╔══════════════════════════════════════╗"))
	fmt.Println(titleColor("║       GARMIN BETTER SYNC CLI         ║"))
	fmt.Println(titleColor("╚══════════════════════════════════════╝"))

	fmt.Println("\n" + sectionColor("🎧 SPOTIFY MANAGEMENT:"))
	fmt.Printf("  %s %s\n", numberColor("1."), optionColor("Download Spotify playlist"))

	fmt.Println("\n" + sectionColor("🎵 SONG MANAGEMENT:"))
	fmt.Printf("  %s %s\n", numberColor("2."), optionColor("Show songs"))
	fmt.Printf("  %s %s\n", numberColor("3."), optionColor("Upload song"))
	fmt.Printf("  %s %s\n", numberColor("4."), optionColor("Delete song"))

	fmt.Println("\n" + sectionColor("📋 PLAYLIST MANAGEMENT:"))
	fmt.Printf("  %s %s\n", numberColor("5."), optionColor("Show playlists"))
	fmt.Printf("  %s %s\n", numberColor("6."), optionColor("Show playlists and songs"))
	fmt.Printf("  %s %s\n", numberColor("7."), optionColor("Create and upload playlist"))
	fmt.Printf("  %s %s\n", numberColor("8."), optionColor("Upload directory and create playlist"))
	fmt.Printf("  %s %s\n", numberColor("9."), optionColor("Delete playlist"))
	fmt.Printf("  %s %s\n", numberColor("10."), optionColor("Delete playlist and all its songs"))

	fmt.Println("\n" + sectionColor("📁 FOLDER MANAGEMENT:"))
	fmt.Printf("  %s %s\n", numberColor("11."), optionColor("Delete all music contents from device"))

	fmt.Println("\n" + sectionColor("🚪 SYSTEM:"))
	fmt.Printf("  %s %s\n", numberColor("12."), optionColor("Exit"))

	fmt.Print("\n" + color.HiMagentaString("Select an option") + color.HiWhiteString(" ❯ "))

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	option := 0
	fmt.Sscanf(input, "%d", &option)
	return option
}

func Execute(dev *mtp.Device, storages interface{}, operation int) {
	for {
		op := operation
		if op == 0 {
			spotifyPath := os.Getenv("SPOTIFY_DOWNLOAD_PATH")
			if spotifyPath != "" {
				os.Setenv("SPOTIFY_DOWNLOAD_PATH", "")

				storageID, musicFolderID, err := SelectStorageAndMusicFolder(dev, storages)
				if err != nil {
					util.LogError("Error selecting storage: %v", err)
					fmt.Printf("Error: %v\n", err)
				} else {
					os.Setenv("AUTO_UPLOAD_PATH", spotifyPath)
					autoPath := os.Getenv("AUTO_UPLOAD_PATH")
					if autoPath != "" {
						os.Setenv("AUTO_UPLOAD_PATH", "")
						UploadDirectoryWithPlaylistFromPath(dev, storageID, musicFolderID, autoPath)
					} else {
						UploadDirectoryWithPlaylist(dev, storageID, musicFolderID)
					}
				}
				op = ShowMenu()
			} else {
				op = ShowMenu()
			}
		}

		switch op {
		case 0: // Back
			// Return to previous menu or application state
			return
		case 1: // Download Spotify playlist
			DownloadSpotifyPlaylist()
		case 2: // Show songs
			songs, err := GetSongs(dev, storages)
			if err != nil {
				util.LogError("Error getting songs: %v", err)
				break
			}
			DisplaySongsToConsole(songs)
		case 3: // Upload song
			UploadSong(dev, storages)
		case 4: // Delete song
			DeleteSong(dev, storages)
		case 5: // Show playlists
			playlists, err := GetPlaylists(dev, storages)
			if err != nil {
				util.LogError("Error getting playlists: %v", err)
				break
			}
			DisplayPlaylistsToConsole(playlists)
		case 6: // Show playlists and songs
			result, err := GetPlaylistsWithSongs(dev, storages)
			if err != nil {
				util.LogError("Error getting playlists and songs: %v", err)
				break
			}
			PrintPlaylistsAndSongs(dev, result)
		case 7: // Create and upload playlist
			CreateAndUploadPlaylist(dev, storages)
		case 8: // Upload directory and create playlist
			storageID, musicFolderID, err := SelectStorageAndMusicFolder(dev, storages)
			if err != nil {
				util.LogError("Error selecting storage: %v", err)
				fmt.Printf("Error: %v\n", err)
				break
			}
			UploadDirectoryWithPlaylist(dev, storageID, musicFolderID)
		case 9: // Delete playlist
			DeletePlaylist(dev, storages)
		case 10: // Delete playlist and all its songs
			DeletePlaylistAndAllSongs(dev, storages)
		case 11: // Delete folder and all its contents
			storageID, musicFolderID, err := SelectStorageAndMusicFolder(dev, storages)
			if err != nil {
				util.LogError("Error selecting storage: %v", err)
				fmt.Printf("Error: %v\n", err)
				break
			}

			fmt.Printf("\n⚠️ WARNING: Are you sure you want to delete this folder and all its contents?\n")
			fmt.Print("Type 'yes' to confirm: ")
			var confirmation string
			fmt.Scanln(&confirmation)

			if strings.ToLower(confirmation) != "yes" {
				fmt.Println("Operation cancelled.")
				break
			}

			err = DeleteFolderRecursively(dev, storageID, musicFolderID, "/Music", false)
			if err != nil {
				util.LogError("Error deleting folder: %v", err)
			}
		case 12: // Exit
			color.HiYellow("Exiting program. Goodbye!")
			return
		default:
			color.HiRed("Invalid option. Please try again.")
		}

		if operation != 0 {
			break
		}

		if operation == 0 {
			fmt.Print("\n" + color.HiWhiteString("Press Enter to continue..."))
			bufio.NewReader(os.Stdin).ReadBytes('\n')
		}
	}
}

func DisplayPlaylistsToConsole(playlists []model.PlaylistInfo) {
	if len(playlists) > 0 {
		successColor := color.New(color.FgHiGreen, color.Bold).PrintFunc()
		successColor("\n✓ Found playlists:\n")
		for i, playlist := range playlists {
			nameWithoutExt := strings.TrimSuffix(playlist.Name, ".m3u8")
			fmt.Printf("%d. %s (Path: %s)\n", i+1, color.New(color.FgHiCyan, color.Bold).Sprint(nameWithoutExt), color.New(color.Faint).Sprint(playlist.Path))
		}
		fmt.Printf("\nTotal playlists: %d\n", len(playlists))
	} else {
		errorColor := color.New(color.FgHiRed).PrintFunc()
		errorColor("\n✗ No playlists found\n")
	}
}

func DisplaySongsToConsole(songs []model.Song) {
	if len(songs) > 0 {
		successColor := color.New(color.FgHiGreen, color.Bold).PrintFunc()
		songNameColor := color.New(color.FgHiCyan).SprintFunc()
		pathColor := color.New(color.FgHiWhite).SprintFunc()
		indexColor := color.New(color.FgHiYellow, color.Bold).SprintFunc()
		totalColor := color.New(color.FgHiMagenta, color.Bold).SprintFunc()

		musicEmoji := "🎵"

		fmt.Println()
		successColor("🎧 Found songs")
		fmt.Println(strings.Repeat("─", 50))

		for i, song := range songs {
			fmt.Printf("  %s %s %s\n",
				indexColor(fmt.Sprintf("%d.", i+1)),
				musicEmoji,
				songNameColor(song.Name))
			fmt.Printf("     %s %s\n",
				strings.Repeat(" ", len(fmt.Sprintf("%d", i+1))+1),
				pathColor(fmt.Sprintf("Path: %s", song.Path)))

			if i < len(songs)-1 {
				fmt.Println(strings.Repeat("  ", 2) + strings.Repeat("·", 30))
			}
		}

		fmt.Println(strings.Repeat("─", 50))
		fmt.Printf("\n%s Total songs: %s\n", musicEmoji, totalColor(fmt.Sprintf("%d", len(songs))))
	} else {
		errorColor := color.New(color.FgHiRed, color.Bold).PrintFunc()
		errorColor("\n❌ No songs found\n")
	}
}

func PrintPlaylistsAndSongs(dev *mtp.Device, result *model.DevicePlaylistData) {
	if result == nil || len(result.Storages) == 0 {
		errorColor := color.New(color.FgHiRed).PrintFunc()
		errorColor("\n✗ No playlists or songs found\n")
		return
	}

	headerColor := color.New(color.FgHiCyan, color.Bold).PrintFunc()
	storageColor := color.New(color.FgHiYellow, color.Bold).PrintFunc()
	playlistColor := color.New(color.FgHiGreen).PrintFunc()

	headerColor("\n==== Playlists and Songs on Device ====\n")

	totalSongs := 0

	for _, storage := range result.Storages {
		storageColor(fmt.Sprintf("\nStorage: %s (ID: %d)\n",
			storage.StorageDescription, storage.StorageID))

		for i, playlist := range storage.Playlists {
			playlistName := filepath.Base(playlist.Path)
			playlistColor(fmt.Sprintf("\n%d. %s\n", i+1, playlistName))
			fmt.Printf("   Path: %s\n", playlist.Path)

			songCount := len(playlist.SongPaths)
			totalSongs += songCount

			if songCount == 0 {
				fmt.Println("   (Empty playlist)")
			} else {
				for j, songPath := range playlist.SongPaths {
					fmt.Printf("   %d.%d. %s\n", i+1, j+1, filepath.Base(songPath))
					fmt.Printf("       Path: %s\n", songPath)

					if j >= 9 && songCount > 10 {
						fmt.Printf("   ...and %d more songs\n", songCount-10)
						break
					}
				}
			}
		}
	}

	fmt.Printf("\nTotal: %d playlists with %d songs\n", result.TotalPlaylists, totalSongs)
}

func ConvertToPlaylistInfoList(data *model.DevicePlaylistData) []model.PlaylistInfo {
	if data == nil {
		return []model.PlaylistInfo{}
	}

	var result []model.PlaylistInfo

	for _, storage := range data.Storages {
		for _, playlist := range storage.Playlists {
			info := model.PlaylistInfo{
				Name:      filepath.Base(playlist.Path),
				Path:      playlist.Path,
				ObjectID:  playlist.ObjectID,
				StorageID: playlist.StorageID,
				Storage:   storage.StorageDescription,
			}
			result = append(result, info)
		}
	}

	return result
}

func DeletePlaylistAndAllSongs(dev *mtp.Device, storages interface{}) {
	headerColor := color.New(color.FgHiCyan, color.Bold)
	promptColor := color.New(color.FgHiYellow)
	successColor := color.New(color.FgHiGreen, color.Bold)
	errorColor := color.New(color.FgHiRed, color.Bold)
	playlistColor := color.New(color.FgHiMagenta)

	headerColor.Println("\n╔═══════════════════════════════════╗")
	headerColor.Println("║  DELETE PLAYLIST AND ALL SONGS    ║")
	headerColor.Println("╚═══════════════════════════════════╝")

	promptColor.Println("\nDo you want to see a list of available playlists? (y/n)")
	reader := bufio.NewReader(os.Stdin)
	listResponse, _ := reader.ReadString('\n')
	listResponse = strings.TrimSpace(listResponse)

	if strings.ToLower(listResponse) == "y" {
		playlists, err := GetPlaylists(dev, storages)
		if err != nil {
			errorColor.Printf("\n❌ Error getting playlists: %v\n", err)
			return
		}

		if len(playlists) == 0 {
			errorColor.Println("\n❌ No playlists found on device")
			return
		}

		headerColor.Println("\n📋 Available Playlists:")
		fmt.Println(strings.Repeat("─", 50))

		for i, playlist := range playlists {
			playlistName := strings.ToUpper(strings.TrimSuffix(strings.TrimSuffix(playlist.Name, ".m3u8"), ".M3U8"))
			playlistColor.Printf("  %d. %s\n", i+1, playlistName)
		}

		fmt.Println(strings.Repeat("─", 50))
	}

	promptColor.Print("\n📝 Enter the name of the playlist to delete: ")
	playlistName, _ := reader.ReadString('\n')
	playlistName = strings.TrimSpace(playlistName)

	if playlistName == "" {
		errorColor.Println("\n❌ No playlist name provided. Operation cancelled.")
		return
	}

	if !strings.HasSuffix(strings.ToUpper(playlistName), ".M3U8") {
		playlistName = strings.ToUpper(playlistName + ".M3U8")
	} else {
		playlistName = strings.ToUpper(playlistName)
	}

	promptColor.Printf("\n⚠️  Are you sure you want to delete playlist '%s' and all its songs? (y/n): ",
		strings.TrimSuffix(playlistName, ".M3U8"))
	confirmResponse, _ := reader.ReadString('\n')
	confirmResponse = strings.TrimSpace(confirmResponse)

	if strings.ToLower(confirmResponse) != "y" {
		promptColor.Println("\nOperation cancelled.")
		return
	}

	fmt.Println("\n🔄 Processing deletion request...")

	if err := EnhancedDeletePlaylistAndAllSongs(dev, storages, playlistName); err != nil {
		util.LogError("Error deleting playlist and songs: %v", err)
		errorColor.Printf("\n❌ Error: %v\n", err)
		return
	}

	successColor.Printf("\n✅ Successfully deleted playlist '%s' and all its songs\n",
		strings.TrimSuffix(playlistName, ".M3U8"))
}

func DownloadSpotifyPlaylist() {
	headerColor := color.New(color.FgHiCyan, color.Bold)
	promptColor := color.New(color.FgHiYellow)
	successColor := color.New(color.FgHiGreen, color.Bold)
	errorColor := color.New(color.FgHiRed, color.Bold)
	infoColor := color.New(color.FgHiWhite)

	headerColor.Println("\n╔═══════════════════════════════════╗")
	headerColor.Println("║    DOWNLOAD SPOTIFY PLAYLIST      ║")
	headerColor.Println("╚═══════════════════════════════════╝")

	reader := bufio.NewReader(os.Stdin)

	promptColor.Print("\n🔗 Enter Spotify playlist URL: ")
	playlistURL, err := reader.ReadString('\n')
	playlistURL = strings.TrimSpace(playlistURL)
	if err != nil || playlistURL == "" {
		errorColor.Println("\n❌ Invalid input. Operation cancelled.")
		return
	}

	playlistName, err := util.GetSpotifyPlaylistName(playlistURL)
	if err != nil {
		errorColor.Printf("\n❌ Error getting playlist name: %v\n", err)
		promptColor.Print("\n📝 Enter playlist name: ")
		playlistName, err = reader.ReadString('\n')
		playlistName = strings.TrimSpace(playlistName)
		if err != nil || playlistName == "" {
			errorColor.Println("\n❌ Invalid input. Operation cancelled.")
			return
		}
		return
	}

	playlistName = strings.TrimSpace(playlistName)
	if err != nil || playlistName == "" {
		errorColor.Println("\n❌ Invalid input. Operation cancelled.")
		return
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	defaultDir := filepath.Join(homeDir, "Documents", "music")

	promptColor.Printf("\n📂 Enter download path [default: %s]: ", defaultDir)
	downloadPath, err := reader.ReadString('\n')
	downloadPath = strings.TrimSpace(downloadPath)
	if err != nil {
		errorColor.Println("\n❌ Error reading input.")
		return
	}

	if downloadPath == "" {
		downloadPath = defaultDir
	}

	destDir := filepath.Join(downloadPath, playlistName)
	os.MkdirAll(destDir, 0755)

	headerColor.Println("\n📋 Configuration Summary:")
	infoColor.Printf("  🔗 Playlist URL: ")
	successColor.Printf("%s\n", playlistURL)
	infoColor.Printf("  📝 Playlist Name: ")
	successColor.Printf("%s\n", playlistName)
	infoColor.Printf("  📂 Destination: ")
	successColor.Printf("%s\n", destDir)

	promptColor.Print("\n⚠️  Proceed with download? (y/n): ")
	confirm, _ := reader.ReadString('\n')
	confirm = strings.TrimSpace(confirm)
	if strings.ToLower(confirm) != "y" {
		promptColor.Println("\nOperation cancelled.")
		return
	}

	infoColor.Println("\n⏳ Downloading playlist...")

	cmd := exec.Command("spotdl", "download", playlistURL, "--output", filepath.Join(destDir, "{title}"))

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		errorColor.Printf("\n❌ Failed to start spotdl: %v\n", err)
		errorColor.Println("\n❓ Is spotdl installed? Install with: pip install spotdl")
		return
	}

	scanner := bufio.NewScanner(io.MultiReader(stdout, stderr))
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}

	if err := cmd.Wait(); err != nil {
		errorColor.Printf("\n❌ Error during download: %v\n", err)
		return
	}

	successColor.Printf("\n✅ Download complete! Files saved to: %s\n", destDir)

	promptColor.Print("\n📲 Do you want to upload this playlist to your Garmin device? (y/n): ")
	uploadConfirm, _ := reader.ReadString('\n')
	uploadConfirm = strings.TrimSpace(uploadConfirm)

	if strings.ToLower(uploadConfirm) == "y" {
		infoColor.Println("\n🔄 Preparing to upload to Garmin device...")
		os.Setenv("SPOTIFY_DOWNLOAD_PATH", destDir)
		return
	}
}

// UploadDirectoryWithPlaylistFromPath is a variant of UploadDirectoryWithPlaylist
// that uses a pre-set path from a Spotify download
func UploadDirectoryWithPlaylistFromPath(dev *mtp.Device, storageID, musicFolderID uint32, directoryPath string) {
	headerColor := color.New(color.FgHiCyan, color.Bold)
	infoColor := color.New(color.FgHiWhite)

	headerColor.Println("\n╔═══════════════════════════════════╗")
	headerColor.Println("║  UPLOAD SPOTIFY PLAYLIST TO GARMIN ║")
	headerColor.Println("╚═══════════════════════════════════╝")

	infoColor.Printf("\n📁 Using directory: %s\n", directoryPath)

	// Set this global variable to be used by UploadDirectoryWithPlaylist
	os.Setenv("PRESET_DIRECTORY_PATH", directoryPath)
	os.Setenv("PRESET_CONFIRM_UPLOAD", "yes")

	// Call the original function which will now use our preset path
	UploadDirectoryWithPlaylist(dev, storageID, musicFolderID)

	// Clear environment variables
	os.Setenv("PRESET_DIRECTORY_PATH", "")
	os.Setenv("PRESET_CONFIRM_UPLOAD", "")
}
