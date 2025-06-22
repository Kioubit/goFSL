package main

import (
	"crypto/subtle"
	"embed"
	"github.com/gorilla/websocket"
	"goFSL/config"
	"io/fs"
	"net/http"
)

//go:embed www/*
var wwwContent embed.FS

var upgrader = websocket.Upgrader{}

func startHTTPServer() error {
	http.Handle("/", mainPageHandler())
	http.Handle("/u/", BasicAuth(mainPageHandler().ServeHTTP))
	http.HandleFunc("/d/getFileMeta", getFileMeta)
	http.Handle("/u/upload", BasicAuth(uploadHandler))
	http.HandleFunc("/d/download", downloadHandler)
	http.HandleFunc("/d/deleteFile", deleteFileHandler)
	return http.ListenAndServe(":8080", nil)
}

func BasicAuth(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, password, _ := r.BasicAuth()

		found := false
		requiresAuth := false
		for _, account := range config.GlobalConfig.Accounts {
			requiresAuth = true
			if subtle.ConstantTimeCompare([]byte(account.Username), []byte(user)) == 1 ||
				subtle.ConstantTimeCompare([]byte(account.Password), []byte(password)) == 1 {
				found = true
			}
		}

		if !found && requiresAuth {
			w.Header().Set("WWW-Authenticate", "Basic realm=Restricted")
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func mainPageHandler() http.Handler {
	fSys := fs.FS(wwwContent)
	html, _ := fs.Sub(fSys, "www")
	return http.FileServer(http.FS(html))
}

func deleteFileHandler(w http.ResponseWriter, r *http.Request) {
	fileID := r.URL.Query().Get("id")
	providedDeletionToken := r.URL.Query().Get("deletionToken")
	decryptedID, deletionToken, _, err := retrieveFileFromDB(fileID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if subtle.ConstantTimeCompare([]byte(deletionToken), []byte(providedDeletionToken)) != 1 {
		http.Error(w, "Incorrect deletion token", http.StatusBadRequest)
		return
	}
	deleteFile(decryptedID)
	_, _ = w.Write([]byte("File deleted"))
}
