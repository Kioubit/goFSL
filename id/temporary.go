package id

import (
	"errors"
	"strconv"
	"sync"
)

var temporaryIDs = make(map[int]struct{})
var temporaryIDsLock sync.Mutex

const maxIDs = 1000

func GetTemporaryID() (id string, release func(), err error) {
	release = func() {}
	temporaryIDsLock.Lock()
	defer temporaryIDsLock.Unlock()
	for i := 0; i < maxIDs; i++ {
		_, exists := temporaryIDs[i]
		if !exists {
			temporaryIDs[i] = struct{}{}
			return strconv.Itoa(i), func() {
				releaseTemporaryID(i)
			}, nil
		}
	}
	return "", release, errors.New("max IDs reached")
}

func releaseTemporaryID(id int) {
	temporaryIDsLock.Lock()
	defer temporaryIDsLock.Unlock()
	delete(temporaryIDs, id)
}
