package main

import (
	cryptoRand "crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gorilla/websocket"
)

func createDirectory(basePath, path string) error {
	fullPath := filepath.Join(basePath, path)
	stat, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			err = os.Mkdir(fullPath, 0770)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	} else if !stat.IsDir() {
		return fmt.Errorf("%s already exists and is not a directory", fullPath)
	}
	return nil
}

func clearTempDirectory(tempDirectoryPath string) error {
	entries, err := os.ReadDir(tempDirectoryPath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		fullPath := filepath.Join(tempDirectoryPath, entry.Name())
		err = os.RemoveAll(fullPath)
		if err != nil {
			return err
		}
	}
	return nil
}

func sendWebsocketError(c *websocket.Conn, message string) {
	if message == "" {
		message = "Internal server error"
	}
	_ = c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(4000, message))
}

func getRandomToken() string {
	randomBytes := make([]byte, 32)
	_, err := cryptoRand.Read(randomBytes)
	if err != nil {
		panic(err)
	}
	return base64.StdEncoding.EncodeToString(randomBytes)
}
