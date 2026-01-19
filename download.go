package main

import (
	"bufio"
	"database/sql"
	"errors"
	"goFSL/config"
	"goFSL/db"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
)

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	encryptedID := r.URL.Query().Get("id")
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "could not upgrade to websocket", http.StatusBadRequest)
		return
	}
	defer func() {
		_ = c.Close()
	}()

	err = c.SetReadDeadline(time.Now().Add(ConnDeadline))
	if err != nil {
		return
	}
	err = c.SetWriteDeadline(time.Now().Add(ConnDeadline))
	if err != nil {
		return
	}

	decryptedID, _, fMeta, err := retrieveFileFromDB(encryptedID)
	if err != nil {
		sendWebsocketError(c, "File not found")
		return
	}

	if fMeta.Expiry < time.Now().Unix() {
		sendWebsocketError(c, "File expired")
		return
	}

	downloadCompleted := false
	var dbRemainingDownloads = 0
	if fMeta.DownloadsRemaining != -1 {
		err = db.SystemDB.QueryRow("UPDATE files SET DownloadsRemaining = DownloadsRemaining - 1 WHERE ID = ? AND DownloadsRemaining > 0 RETURNING DownloadsRemaining + 1",
			decryptedID).Scan(&dbRemainingDownloads)
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				slog.Error("DB error", "err", err)
				sendWebsocketError(c, "")
				return
			}
		}
	}
	if dbRemainingDownloads == 0 {
		sendWebsocketError(c, "Maximum download count exceeded")
		return
	}
	defer func() {
		if !downloadCompleted && fMeta.DownloadsRemaining != -1 {
			// Reset in case of failed download
			_, _ = db.SystemDB.Exec("UPDATE files SET DownloadsRemaining = DownloadsRemaining + 1 where ID = ?", decryptedID)
		}
	}()

	f, err := os.OpenFile(filepath.Join(config.DataDir, "files", strconv.FormatInt(decryptedID, 10)), os.O_RDONLY, 0660)
	if err != nil {
		slog.Error("error opening file", "err", err)
		sendWebsocketError(c, "")
		return
	}
	defer func() {
		_ = f.Close()
	}()
	g := bufio.NewReader(f)

	var sentBytes uint64 = 0
	for {
		err = c.SetReadDeadline(time.Now().Add(ConnDeadline))
		if err != nil {
			return
		}
		err = c.SetWriteDeadline(time.Now().Add(ConnDeadline))
		if err != nil {
			return
		}
		_, _, err = c.ReadMessage()
		if err != nil {
			sendWebsocketError(c, "")
			return
		}
		expectedRead := min(fMeta.DownloadSize-sentBytes, uint64(fMeta.MaxChunkSize))
		if expectedRead == 0 {
			_ = c.WriteMessage(websocket.BinaryMessage, []byte{})
			break
		}
		sentBytes += expectedRead
		toSend := make([]byte, expectedRead)
		numRead, err := io.ReadFull(g, toSend)
		if err != nil {
			// EOF should not occur
			slog.Error("error reading file", "err", err)
			sendWebsocketError(c, "")
			return
		}
		err = c.WriteMessage(websocket.BinaryMessage, toSend[:numRead])
		if err != nil {
			sendWebsocketError(c, "Failed sending")
			return
		}
	}
	downloadCompleted = true
	if dbRemainingDownloads == 1 && fMeta.DownloadsRemaining != -1 {
		if err := deleteFile(decryptedID); err != nil {
			slog.Warn("error deleting file", "err", err, "id", decryptedID)
		}
	}
}
