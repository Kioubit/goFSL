package main

import (
	cryptoRand "crypto/rand"
	"encoding/base64"
	"fmt"
	"github.com/gorilla/websocket"
	"os"
	"strings"
)

func createDirectory(basePath, path string) error {
	basePath = ensureTrailingSlash(basePath)
	fullPath := basePath + path
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

func ensureTrailingSlash(path string) string {
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	return path
}

func sendWebsocketError(c *websocket.Conn, message string) {
	if message == "" {
		message = "Internal server error"
	}
	_ = c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(4000, message))
}

func getRandomToken() (string, error) {
	randomBytes := make([]byte, 32)
	_, err := cryptoRand.Read(randomBytes)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(randomBytes), nil
}
