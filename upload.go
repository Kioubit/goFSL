package main

import (
	"encoding/base64"
	"encoding/json"
	"goFSL/config"
	"goFSL/db"
	"goFSL/id"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
)

const (
	MB                = 1024 * 1024
	MaxMetadataSize   = 10 * 1024 // 10KB
	MaxChunkSize      = 10 * MB   // 10MB
	ConnDeadline      = 10 * time.Second
	MaxExpiryDuration = 30 * 24 * time.Hour
)

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	var filesDir = filepath.Join(config.DataDir, "/files/")
	var tempDirPath = filepath.Join(filesDir, "/temp/")

	requestedExpiry := r.URL.Query().Get("expiry")
	requestedMaxDownloads := r.URL.Query().Get("max_downloads")

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
	fMeta := fileMetaData{}

	expiry, err := strconv.ParseInt(requestedExpiry, 10, 64)
	if err != nil {
		sendWebsocketError(c, "Invalid expiry value")
		return
	}
	if expiry < time.Now().Unix() || expiry > time.Now().Unix()+int64(MaxExpiryDuration.Seconds()) {
		sendWebsocketError(c, "Expiry time is not valid")
		return
	}
	fMeta.Expiry = expiry

	maxDownloads, err := strconv.Atoi(requestedMaxDownloads)
	if err != nil {
		sendWebsocketError(c, "Invalid max downloads value")
		return
	}
	if maxDownloads < 1 && maxDownloads != -1 {
		sendWebsocketError(c, "Max downloads value must be at least 1")
		return
	}
	fMeta.DownloadsRemaining = maxDownloads

	transferCompleted := false

	tempID, releaseID, err := id.GetTemporaryID()
	if err != nil {
		sendWebsocketError(c, "Too many simultaneous uploads")
		return
	}
	defer releaseID()

	tempFilePath := filepath.Join(tempDirPath, tempID)
	outputFile, err := os.OpenFile(tempFilePath, os.O_WRONLY|os.O_CREATE, 0660)
	if err != nil {
		slog.Error("error opening file", "err", err)
		sendWebsocketError(c, "")
		return
	}
	defer func() {
		_ = outputFile.Close()
		if !transferCompleted {
			err = os.Remove(tempFilePath)
			if err != nil {
				slog.Warn("Failed to remove incomplete data file", "err", err)
			}
		}
	}()

	var totalSize uint64 = 0
	for {
		err = c.SetWriteDeadline(time.Now().Add(ConnDeadline))
		if err != nil {
			return
		}
		err = c.SetReadDeadline(time.Now().Add(ConnDeadline))
		if err != nil {
			return
		}

		mt, message, err := c.ReadMessage()
		if err != nil {
			slog.Debug("ws read error", "err", err)
			break
		}
		if mt == websocket.BinaryMessage {
			if fMeta.MaxChunkSize == 0 {
				fMeta.MaxChunkSize = len(message)
				if fMeta.MaxChunkSize > MaxChunkSize {
					sendWebsocketError(c, "Chunk exceeds maximum allowed size")
					return
				}
			}
			if len(message) > fMeta.MaxChunkSize {
				sendWebsocketError(c, "Chunk size inconsistent")
				return
			}

			totalSize += uint64(len(message))
			if totalSize > config.GlobalConfig.MaxFileSizeMb*1000000 && config.GlobalConfig.MaxFileSizeMb != 0 {
				sendWebsocketError(c, "Maximum file size exceeded")
				return
			}

			_, err = outputFile.Write(message)
			if err != nil {
				slog.Error("file write error", "err", err)
				sendWebsocketError(c, "")
				return
			}

		} else if mt == websocket.TextMessage {
			expectedByteCount, err := strconv.ParseUint(string(message), 10, 64)
			if err != nil {
				sendWebsocketError(c, "Invalid message format")
				return
			}
			if totalSize == expectedByteCount {
				transferCompleted = true
				break
			}
			sendWebsocketError(c, "Transfer failed")
			return
		}
	}

	// Read encrypted metadata
	mt, metaDataMessage, err := c.ReadMessage()
	if err != nil {
		sendWebsocketError(c, "")
		return
	}
	if mt != websocket.BinaryMessage {
		sendWebsocketError(c, "Invalid message received")
		return
	}

	if len(metaDataMessage) > MaxMetadataSize {
		sendWebsocketError(c, "File metadata too large")
		return
	}

	fMeta.UserMetaData = base64.StdEncoding.EncodeToString(metaDataMessage)

	fMeta.DownloadSize = totalSize
	err = outputFile.Close()
	if err != nil {
		slog.Error("error closing file", "err", err)
		return
	}

	if !transferCompleted {
		return
	}

	deletionToken := getRandomToken()

	var permanentID int64
	err = db.SystemDB.QueryRow("INSERT INTO files(Expiry, DownloadsRemaining, MaxChunkSize, DownloadSize, UserMetaData, DeletionToken) VALUES (?,?, ?, ?, ?, ?) RETURNING ID",
		fMeta.Expiry, fMeta.DownloadsRemaining, fMeta.MaxChunkSize, fMeta.DownloadSize, fMeta.UserMetaData, deletionToken).Scan(&permanentID)
	if err != nil {
		slog.Error("DB error", "err", err)
		sendWebsocketError(c, "")
		return
	}
	err = os.Rename(tempFilePath, filepath.Join(filesDir, strconv.FormatInt(permanentID, 10)))
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
