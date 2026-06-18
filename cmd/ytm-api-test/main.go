// Package main provides a simple API test server for ytm-go features.
//
// Purpose:
//
//	Expose search, suggestions, playlist details, artist details, and lyrics over HTTP.
//
// Key Components:
//   - main: starts HTTP server on port 8081
//   - searchHandler: handles search query request
//   - suggestHandler: handles autocomplete suggestions query
//   - artistHandler: handles artist profile requests
//   - playlistHandler: handles playlist/album detail requests
//   - lyricsHandler: handles lyrics content requests
//
// Dependencies:
//   - context
//   - encoding/json
//   - net/http
//   - github.com/dlcuy22/ytm-go
//
// Error Types:
//   - None
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dlcuy22/ytm-go"
)

var client *ytm.Client

func main() {
	client = ytm.NewClient()

	http.HandleFunc("/", serveIndex)
	http.HandleFunc("/api/search", handleSearch)
	http.HandleFunc("/api/suggest", handleSuggest)
	http.HandleFunc("/api/artist", handleArtist)
	http.HandleFunc("/api/playlist", handlePlaylist)
	http.HandleFunc("/api/lyrics", handleLyrics)

	fmt.Println("Server running at http://localhost:8081")
	log.Fatal(http.ListenAndServe(":8081", nil))
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, "cmd/ytm-api-test/index.html")
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing query parameter 'q'"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	results, err := client.Search(ctx, q, "", false)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Simple helper structures to help serialize MediaItem interface correctly if required,
	// but standard json marshaller of ytm.SearchResults already resolves concrete implementations.
	writeJSON(w, http.StatusOK, results)
}

func handleSuggest(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing query parameter 'q'"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	suggestions, err := client.GetSearchSuggestions(ctx, q)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, suggestions)
}

func handleArtist(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing parameter 'id'"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	artist, err := client.LoadArtist(ctx, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, artist)
}

func handlePlaylist(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing parameter 'id'"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Handle standard playlist loading (also covers albums)
	playlist, err := client.LoadPlaylist(ctx, id, nil, nil, nil, false)
	if err != nil {
		// Try fallback with useNonMusicAPI if needed, or just return the error
		if strings.HasPrefix(id, "VL") {
			playlist, err = client.LoadPlaylist(ctx, id, nil, nil, nil, true)
		}
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	writeJSON(w, http.StatusOK, playlist)
}

func handleLyrics(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing parameter 'id'"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	lyrics, err := client.GetSongLyrics(ctx, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"lyrics": lyrics})
}
