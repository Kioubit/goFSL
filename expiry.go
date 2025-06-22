package main

import (
	"database/sql"
	"errors"
	"goFSL/db"
	"log/slog"
	"time"
)

func DeleteAllExpiredFiles() {
	rows, err := db.SystemDB.Query("SELECT ID FROM files where Expiry <  ?", time.Now().Unix())
	if err != nil {
		slog.Warn("Failed to delete all expired files", "err", err)
		return
	}
	toDelete := make([]int64, 0)
	for rows.Next() {
		var ID int64
		err = rows.Scan(&ID)
		if err != nil {
			slog.Warn("Failed to delete expired files", "err", err)
			return
		}
		toDelete = append(toDelete, ID)
	}
	if err = rows.Err(); err != nil {
		slog.Warn("Failed to delete all expired files", "err", err)
		return
	}
	for _, ID := range toDelete {
		deleteFile(ID)
	}
}

var NextExpiryChannel = make(chan int64, 2)

func ExpiryObserver() {
	var trackedExpiry = getNextExpiryTime()
	for {
		if trackedExpiry == 0 {
			trackedExpiry = <-NextExpiryChannel
		}
		delay := time.NewTimer(time.Until(time.Unix(trackedExpiry, 0)))
		select {
		case receivedTime := <-NextExpiryChannel:
			if trackedExpiry > receivedTime {
				trackedExpiry = receivedTime
			}
		case <-delay.C:
			DeleteAllExpiredFiles()
			trackedExpiry = getNextExpiryTime()
		}
		delay.Stop()
	}
}

func getNextExpiryTime() int64 {
	var nextExpiryTime int64
	err := db.SystemDB.QueryRow("SELECT Expiry FROM files ORDER BY Expiry LIMIT 1").Scan(&nextExpiryTime)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			slog.Warn("unable to obtain next expiry time", "err", err)
		}
		return 0
	}
	return nextExpiryTime
}
