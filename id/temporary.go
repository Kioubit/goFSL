package id

import (
	"errors"
	"strconv"
	"sync"
)

var temporaryIDs = make(map[int]struct{})
var temporaryIDsLock sync.Mutex

const maxIDs = 1000

func GetTemporaryID() (id string, err error) {
	temporaryIDsLock.Lock()
	defer temporaryIDsLock.Unlock()
	for i := 0; i < maxIDs; i++ {
		_, exists := temporaryIDs[i]
		if !exists {
			temporaryIDs[i] = struct{}{}
			return strconv.Itoa(i), nil
		}
	}
	return "", errors.New("max IDs reached")
}

func ReleaseTemporaryID(id string) {
	idInt, err := strconv.Atoi(id)
	if err != nil {
		return
	}
	temporaryIDsLock.Lock()
	defer temporaryIDsLock.Unlock()
	delete(temporaryIDs, idInt)
}
