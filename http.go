package main

import (
	"crypto/subtle"
	"embed"
	"goFSL/config"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/bcrypt"
)

//go:embed www/*
var wwwContent embed.FS

var upgrader = websocket.Upgrader{}

func startHTTPServer(httpPort int) error {
	http.Handle("/", mainPageHandler())
	http.Handle("/u/", BasicAuth(mainPageHandler().ServeHTTP))
	http.HandleFunc("/d/getFileMeta", getFileMeta)
	http.Handle("/u/upload", BasicAuth(uploadHandler))
	http.HandleFunc("/d/download", downloadHandler)
	http.HandleFunc("/d/deleteFile", deleteFileHandler)
	return http.ListenAndServe(":"+strconv.Itoa(httpPort), nil)
}

func BasicAuth(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requiresAuth := len(config.GlobalConfig.AccountList) != 0

		user, password, _ := r.BasicAuth()

		authOK := false
		for _, account := range config.GlobalConfig.AccountList {
			if account.Username == user {
				authOK = bcrypt.CompareHashAndPassword([]byte(account.Password), []byte(password)) == nil
				break
			}
		}

		if !authOK && requiresAuth {
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
	if err := deleteFile(decryptedID); err != nil {
		slog.Warn("error deleting file", "err", err, "id", decryptedID)
	}
	_, _ = w.Write([]byte("File deleted"))
}
