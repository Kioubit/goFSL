package id

import (
	"crypto/aes"
	cryptoRand "crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"goFSL/db"
)

var idKey = make([]byte, 32)

func InitializeIDKey() error {
	var idKeyString string
	err := db.SystemDB.QueryRow("SELECT Value FROM KV WHERE Key = 'idKey'").Scan(&idKeyString)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if _, err := cryptoRand.Read(idKey); err != nil {
			return err
		}
		_, err = db.SystemDB.Exec("INSERT INTO KV(Key, Value) VALUES ('idKey', ?)", base64.StdEncoding.EncodeToString(idKey))
		if err != nil {
			return err
		}
	} else {
		idKey, err = base64.StdEncoding.DecodeString(idKeyString)
		if err != nil {
			return err
		}
	}
	return nil
}

func EncryptID(id int64) string {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, uint64(id))
	block := make([]byte, 16)
	copy(block[:8], buf)
	result := aesModeECBEncryptRaw(block, idKey)
	return hex.EncodeToString(result)
}

func DecryptID(id string) (uint64, error) {
	encrypted, err := hex.DecodeString(id)
	if err != nil {
		return 0, err
	}
	result, err := aesModeECBDecryptRaw(encrypted, idKey)
	if err != nil {
		return 0, err
	}
	if len(result) < 8 {
		return 0, errors.New("invalid id")
	}
	// Basic forgery protection
	cmp := make([]byte, 8)
	if subtle.ConstantTimeCompare(result[8:], cmp) == 0 {
		return 0, errors.New("invalid ID")
	}

	intResult := binary.LittleEndian.Uint64(result[:8])

	return intResult, nil
}

func aesModeECBEncryptRaw(data, key []byte) []byte {
	c, _ := aes.NewCipher(key)
	encrypted := make([]byte, len(data))
	blockSize := aes.BlockSize
	for bs, be := 0, blockSize; bs < len(data); bs, be = bs+blockSize, be+blockSize {
		c.Encrypt(encrypted[bs:be], data[bs:be])
	}
	return encrypted
}

func aesModeECBDecryptRaw(data, key []byte) (decrypted []byte, err error) {
	defer func() {
		if recover() != nil {
			err = errors.New("decryption error")
		}
	}()
	c, _ := aes.NewCipher(key)
	decrypted = make([]byte, len(data))
	blockSize := aes.BlockSize
	for bs, be := 0, blockSize; bs < len(data); bs, be = bs+blockSize, be+blockSize {
		c.Decrypt(decrypted[bs:be], data[bs:be])
	}
	return decrypted, nil
}
