package main

import (
	"bufio"
	"github.com/gorilla/websocket"
	"goFSL/config"
	"goFSL/db"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"
)

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	encryptedID := r.URL.Query().Get("id")
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "could not upgrade to websocket", http.StatusBadRequest)
		return
	}
	defer func(c *websocket.Conn) {
		_ = c.Close()
	}(c)
	err = c.SetWriteDeadline(time.Now().Add(30 * time.Second))
	if err != nil {
		http.Error(w, http.StatusText(500), http.StatusInternalServerError)
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
	if fMeta.DownloadsRemaining != -2 {
		err = db.SystemDB.QueryRow("UPDATE files SET DownloadsRemaining = MAX(DownloadsRemaining - 1, -1) WHERE ID = ? RETURNING DownloadsRemaining", decryptedID).Scan(&dbRemainingDownloads)
		if err != nil {
			slog.Error("DB error", "err", err)
			sendWebsocketError(c, "")
			return
		}
	}
	if dbRemainingDownloads < 0 {
		sendWebsocketError(c, "Maximum download count exceeded")
		return
	}
	defer func() {
		if !downloadCompleted && fMeta.DownloadsRemaining != -2 {
			// Reset in case of failed download
			_, _ = db.SystemDB.Exec("UPDATE files SET DownloadsRemaining = DownloadsRemaining + 1 where ID = ?", decryptedID)
		}
	}()

	f, err := os.OpenFile(config.DataDir+"files/"+strconv.FormatInt(decryptedID, 10), os.O_RDONLY, 0660)
	if err != nil {
		slog.Error("error opening file", "err", err)
		sendWebsocketError(c, "")
		return
	}
	defer func(f *os.File) {
		_ = f.Close()
	}(f)
	g := bufio.NewReader(f)

	var sentBytes uint64 = 0
	for {
		_ = c.SetWriteDeadline(time.Now().Add(30 * time.Second))
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
	if dbRemainingDownloads == 0 && fMeta.DownloadsRemaining != -2 {
		deleteFile(decryptedID)
	}
}
