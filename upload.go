package main

import (
	"encoding/base64"
	"encoding/json"
	"github.com/gorilla/websocket"
	"goFSL/config"
	"goFSL/db"
	"goFSL/id"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"
)

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	var filesDir = config.DataDir + "files/"
	var tempPath = filesDir + "temp/"

	requestedExpiry := r.URL.Query().Get("expiry")
	requestedMaxDownloads := r.URL.Query().Get("max_downloads")
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "could not upgrade to websocket", http.StatusBadRequest)
		return
	}
	defer func(c *websocket.Conn) {
		_ = c.Close()
	}(c)

	_ = c.SetReadDeadline(time.Now().Add(30 * time.Second))
	fMeta := fileMetaData{}

	expiry, err := strconv.ParseInt(requestedExpiry, 10, 64)
	if err != nil {
		sendWebsocketError(c, "Invalid expiry value")
		return
	}
	if expiry < time.Now().Unix() || expiry > time.Now().Unix()+60*60*24*30+120 {
		sendWebsocketError(c, "Expiry time is not valid")
		return
	}
	fMeta.Expiry = expiry

	maxDownloads, err := strconv.Atoi(requestedMaxDownloads)
	if err != nil {
		sendWebsocketError(c, "Invalid max downloads value")
		return
	}
	if maxDownloads < 1 {
		if maxDownloads != -2 {
			sendWebsocketError(c, "Max downloads value must be at least 1")
			return
		}
	}
	fMeta.DownloadsRemaining = maxDownloads

	transferCompleted := false

	tempID, err := id.GetTemporaryID()
	if err != nil {
		sendWebsocketError(c, "Too many simultaneous uploads")
		return
	}
	defer func() {
		if !transferCompleted {
			err := os.Remove(tempPath + tempID)
			if err != nil {
				slog.Warn("Failed to remove incomplete data file", "err", err)
			}
		}
		id.ReleaseTemporaryID(tempID)
	}()

	mt, metaDataMessage, err := c.ReadMessage()
	if err != nil {
		http.Error(w, http.StatusText(500), http.StatusInternalServerError)
		return
	}
	if mt != websocket.BinaryMessage {
		sendWebsocketError(c, "Invalid message received")
		return
	}

	if len(metaDataMessage) > 1000 {
		sendWebsocketError(c, "File metadata too large")
		return
	}

	fMeta.UserMetaData = base64.StdEncoding.EncodeToString(metaDataMessage)

	outputFile, err := os.OpenFile(tempPath+tempID, os.O_WRONLY|os.O_CREATE, 0660)
	if err != nil {
		slog.Error("error opening file", "err", err)
		sendWebsocketError(c, "")
		return
	}

	var totalSize uint64 = 0
	for {
		mt, message, err := c.ReadMessage()
		if err != nil {
			slog.Debug("ws read error", "err", err)
			break
		}
		err = c.SetReadDeadline(time.Now().Add(30 * time.Second))
		if err != nil {
			return
		}

		if mt == websocket.BinaryMessage {
			if fMeta.MaxChunkSize == 0 {
				fMeta.MaxChunkSize = len(message)
			}
			if len(message) > 10000000 {
				_ = outputFile.Close()
				sendWebsocketError(c, "Chunk size too large")
				return
			}

			totalSize += uint64(len(message))
			if totalSize > config.GlobalConfig.MaxFileSizeMb*1000000 && config.GlobalConfig.MaxFileSizeMb != 0 {
				_ = outputFile.Close()
				sendWebsocketError(c, "Maximum file size exceeded")
				return
			}

			_, err = outputFile.Write(message)
			if err != nil {
				_ = outputFile.Close()
				slog.Error("file write error", "err", err)
				sendWebsocketError(c, "")
				return
			}

			if err = c.WriteMessage(websocket.TextMessage, []byte("OK")); err != nil {
				sendWebsocketError(c, "")
				return
			}
		} else if mt == websocket.TextMessage {
			if string(message) == "COMPLETED" {
				transferCompleted = true
				break
			}
		}
	}
	fMeta.DownloadSize = totalSize
	_ = outputFile.Close()
	if !transferCompleted {
		return
	}

	deletionToken, err := getRandomToken()
	if err != nil {
		slog.Error("Failed to get random token", "err", err)
		sendWebsocketError(c, "")
		return
	}

	var permanentID int64
	err = db.SystemDB.QueryRow("INSERT INTO files(Expiry, DownloadsRemaining, MaxChunkSize, DownloadSize, UserMetaData, DeletionToken) VALUES (?,?, ?, ?, ?, ?) RETURNING ID",
		fMeta.Expiry, fMeta.DownloadsRemaining, fMeta.MaxChunkSize, fMeta.DownloadSize, fMeta.UserMetaData, deletionToken).Scan(&permanentID)
	if err != nil {
		slog.Error("DB error", "err", err)
		sendWebsocketError(c, "")
		return
	}
	err = os.Rename(tempPath+tempID, filesDir+strconv.FormatInt(permanentID, 10))
	if err != nil {
		slog.Error("Error moving file", "err", err)
		sendWebsocketError(c, "")
		return
	}

	NextExpiryChannel <- expiry
	js, _ := json.Marshal(struct {
		ID            string
		DeletionToken string
	}{
		id.EncryptID(permanentID),
		deletionToken,
	})
	_ = c.WriteMessage(websocket.TextMessage, js)
}
