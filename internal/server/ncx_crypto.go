package server

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// Navicat 12+ connection exports use AES-128-CBC with zero padding.
const (
	ncxKey = "libcckeylibcckey"
	ncxIV  = "libcciv libcciv "
)

func decryptNCXPassword(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	data, err := hex.DecodeString(value)
	if err != nil || len(data) == 0 || len(data)%aes.BlockSize != 0 {
		return "", errors.New("password is not a valid AES-CBC hex value")
	}
	block, err := aes.NewCipher([]byte(ncxKey))
	if err != nil {
		return "", err
	}
	out := make([]byte, len(data))
	cipher.NewCBCDecrypter(block, []byte(ncxIV)).CryptBlocks(out, data)
	out = []byte(strings.TrimRight(string(out), "\x00"))
	if !utf8.Valid(out) {
		return "", errors.New("decrypted password is not UTF-8")
	}
	return string(out), nil
}

func encryptNCXPassword(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	data := []byte(value)
	if pad := (aes.BlockSize - len(data)%aes.BlockSize) % aes.BlockSize; pad > 0 {
		data = append(data, make([]byte, pad)...)
	}
	block, err := aes.NewCipher([]byte(ncxKey))
	if err != nil {
		return "", err
	}
	out := make([]byte, len(data))
	cipher.NewCBCEncrypter(block, []byte(ncxIV)).CryptBlocks(out, data)
	return hex.EncodeToString(out), nil
}

func ncxEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "\"", "&quot;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}

func ncxAttr(name, value string) string {
	return fmt.Sprintf(` %s="%s"`, name, ncxEscape(value))
}
