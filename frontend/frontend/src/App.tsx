import { useState } from "react";
import "./App.css";
import SongList from "./components/SongList";

function App() {
  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-900 text-gray-900 dark:text-gray-100">
      <header className="bg-white dark:bg-gray-800 shadow-sm py-6">
        <div className="container mx-auto px-4">
          <h1 className="text-3xl font-bold text-center">Better Sync</h1>
          <p className="text-gray-600 dark:text-gray-400 text-center mt-2">
            Manage your device music library
          </p>
        </div>
      </header>

      <main className="container mx-auto px-4 py-8">
        <div className="bg-white dark:bg-gray-800 rounded-lg shadow-md p-6">
          <SongList />
        </div>
      </main>

      <footer className="bg-white dark:bg-gray-800 shadow-inner py-4 mt-8">
        <div className="container mx-auto px-4 text-center text-sm text-gray-600 dark:text-gray-400">
          <p>Connect your device to sync and manage your music library</p>
        </div>
      </footer>
    </div>
  );
}

export default App;
