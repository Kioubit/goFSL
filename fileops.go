package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"goFSL/config"
	"goFSL/db"
	"goFSL/id"
	"log/slog"
	"net/http"
	"os"
	"strconv"
)

type fileMetaData struct {
	Expiry             int64
	DownloadsRemaining int
	MaxChunkSize       int
	DownloadSize       uint64
	UserMetaData       string
}

func getFileMeta(w http.ResponseWriter, r *http.Request) {
	encryptedID := r.URL.Query().Get("id")
	_, _, fMetadata, err := retrieveFileFromDB(encryptedID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	js, err := json.Marshal(fMetadata)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(js)
}

func retrieveFileFromDB(encryptedID string) (int64, string, fileMetaData, error) {
	decryptedID, err := id.DecryptID(encryptedID)
	if err != nil {
		return 0, "", fileMetaData{}, errors.New("invalid ID")
	}
	var deletionToken string
	fMeta := fileMetaData{}
	err = db.SystemDB.QueryRow("SELECT Expiry, DownloadsRemaining, MaxChunkSize, DownloadSize, UserMetaData, DeletionToken FROM files WHERE ID =?",
		decryptedID).Scan(&fMeta.Expiry, &fMeta.DownloadsRemaining, &fMeta.MaxChunkSize, &fMeta.DownloadSize, &fMeta.UserMetaData, &deletionToken)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, "", fileMetaData{}, errors.New("file not found")
		}
		return 0, "", fileMetaData{}, errors.New("internal server error")
	}
	return int64(decryptedID), deletionToken, fMeta, nil
}

func deleteFile(ID int64) {
	_, _ = db.SystemDB.Exec("DELETE FROM files WHERE ID=?", ID)
	err := os.Remove(config.DataDir + "files/" + strconv.FormatInt(ID, 10))
	if err != nil {
		slog.Error("error deleting file", "err", err)
	}
}
