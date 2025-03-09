import { useState, useEffect } from "react";
import { GetSongs } from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";

// Define the Song interface based on your Go struct
interface Song {
  Name: string;
  Path: string;
  // Add any other properties that your Song struct has
}

const SongList = () => {
  const [songs, setSongs] = useState<Song[]>([]);
  const [loading, setLoading] = useState<boolean>(true);

  useEffect(() => {
    // Load initial songs if available
    const loadSongs = async () => {
      try {
        const songList = await GetSongs();
        console.log("Loaded songs:", songList);
        setSongs(songList || []);
        setLoading(false);
      } catch (error) {
        console.error("Error loading songs:", error);
        setLoading(false);
      }
    };

    // Listen for new songs from the backend
    EventsOn("songs-loaded", (newSongs: Song[]) => {
      console.log("Received songs from event:", newSongs);
      setSongs(newSongs || []);
      setLoading(false);
    });

    loadSongs();
  }, []);

  if (loading) {
    return (
      <div className="p-4 text-center">
        <div className="animate-pulse">Loading songs from your device...</div>
      </div>
    );
  }

  if (!songs || songs.length === 0) {
    return (
      <div className="p-4 text-center">
        <div className="bg-yellow-50 border-l-4 border-yellow-400 p-4 mb-4">
          <div className="flex">
            <div className="flex-shrink-0">
              <svg
                className="h-5 w-5 text-yellow-400"
                viewBox="0 0 20 20"
                fill="currentColor"
              >
                <path
                  fillRule="evenodd"
                  d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z"
                  clipRule="evenodd"
                />
              </svg>
            </div>
            <div className="ml-3">
              <p className="text-sm text-yellow-700">
                No songs found. Connect your device to see songs.
              </p>
            </div>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="p-4">
      <h2 className="text-2xl font-bold mb-4">Your Songs ({songs.length})</h2>
      <div className="grid gap-4 grid-cols-1 md:grid-cols-2 lg:grid-cols-3">
        {songs.map((song, index) => (
          <div
            key={index}
            className="bg-white dark:bg-gray-800 rounded-lg shadow p-4 hover:shadow-lg transition-shadow duration-200"
          >
            <h3 className="font-semibold text-lg mb-2">{song.Name}</h3>
            <p className="text-sm text-gray-600 dark:text-gray-400 truncate">
              {song.Path}
            </p>
          </div>
        ))}
      </div>
    </div>
  );
};

export default SongList;
