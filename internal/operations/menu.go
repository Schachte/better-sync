package operations

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/ganeshrvel/go-mtpfs/mtp"
	"github.com/schachte/better-sync/internal/util"
)

// ShowMenu displays the main menu and returns the selected option
func ShowMenu() int {
	fmt.Println("\n==== MTP Music Manager ====")
	fmt.Println("1. Show playlists")
	fmt.Println("2. Show songs")
	fmt.Println("3. Show playlists and songs")
	fmt.Println("4. Upload song")
	fmt.Println("5. Create and upload playlist")
	fmt.Println("6. Delete playlist")
	fmt.Println("7. Delete song")
	fmt.Println("8. Upload directory and create playlist")
	fmt.Println("9. Delete playlist and all its songs")
	fmt.Println("10. Delete folder and all its contents")
	fmt.Println("11. Exit")
	fmt.Print("\nSelect an option: ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	option := 0
	fmt.Sscanf(input, "%d", &option)
	return option
}

// Execute runs the specified operation or shows the menu if operation is 0
func Execute(dev *mtp.Device, storages interface{}, operation int) {
	for {
		op := operation
		if op == 0 {
			op = ShowMenu()
		}

		switch op {
		case 1:
			ShowPlaylists(dev, storages)
		case 2:
			ShowSongs(dev, storages)
		case 3:
			ShowPlaylistsAndSongs(dev, storages)
		case 4:
			UploadSong(dev, storages)
		case 5:
			CreateAndUploadPlaylist(dev, storages)
		case 6:
			DeletePlaylist(dev, storages)
		case 7:
			DeleteSong(dev, storages)
		case 8:
			// Get storage and music folder
			storageID, musicFolderID, err := SelectStorageAndMusicFolder(dev, storages)
			if err != nil {
				util.LogError("Error selecting storage: %v", err)
				fmt.Printf("Error: %v\n", err)
				break
			}
			UploadDirectoryWithPlaylist(dev, storageID, musicFolderID)
		case 9:
			fmt.Println("\nDo you want to see a list of available playlists? (y/n)")
			reader := bufio.NewReader(os.Stdin)
			listResponse, _ := reader.ReadString('\n')
			listResponse = strings.TrimSpace(listResponse)

			if strings.ToLower(listResponse) == "y" {
				playlists := ShowPlaylists(dev, storages)
				for _, playlist := range playlists {
					fmt.Printf("  %s\n", strings.ToUpper(strings.TrimSuffix(strings.TrimSuffix(playlist.Name, ".m3u8"), ".M3U8")))
				}
			}

			fmt.Print("\nEnter the name of the playlist to delete: ")
			playlistName, _ := reader.ReadString('\n')
			playlistName = strings.TrimSpace(playlistName)
			// Ensure playlist name ends in .M3U8 and is uppercase
			if !strings.HasSuffix(strings.ToUpper(playlistName), ".M3U8") {
				playlistName = strings.ToUpper(playlistName + ".M3U8")
			} else {
				playlistName = strings.ToUpper(playlistName)
			}

			EnhancedDeletePlaylistAndAllSongs(dev, storages, playlistName)
		case 10:
			// Get storage and music folder
			storageID, musicFolderID, err := SelectStorageAndMusicFolder(dev, storages)
			if err != nil {
				util.LogError("Error selecting storage: %v", err)
				fmt.Printf("Error: %v\n", err)
				break
			}
			DeleteFolderRecursively(dev, storageID, musicFolderID, "/Music", true)
		case 11:
			fmt.Println("Exiting program.")
			return
		default:
			fmt.Println("Invalid option. Please try again.")
		}

		// If operation was provided via command line flag, exit after completion
		if operation != 0 {
			break
		}
	}
}
