package minio

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

// https://github.com/minio/minio/blob/RELEASE.2025-02-07T23-21-09Z/internal/auth/credentials.go

const (
	// Minimum length for MinIO access key.
	accessKeyMinLen = 3

	// Maximum length for MinIO access key.
	// There is no max length enforcement for access keys
	accessKeyMaxLen = 20

	// Minimum length for MinIO secret key for both server
	secretKeyMinLen = 8

	// Maximum secret key length for MinIO, this
	// is used when autogenerating new credentials.
	// There is no max length enforcement for secret keys
	secretKeyMaxLen = 40

	// Alpha numeric table used for generating access keys.
	alphaNumericTable = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"

	// Total length of the alpha numeric table.
	alphaNumericTableLen = byte(len(alphaNumericTable))

	// reservedChars = "=,"
)

// GenerateAccessKey returns a new access key generated randomly using
// the given io.Reader. If random is nil, crypto/rand.Reader is used.
// If length <= 0, the access key length is chosen automatically.
//
// GenerateAccessKey returns an error if length is too small for a valid
// access key.
func GenerateAccessKey(length int, random io.Reader) ([]byte, error) {
	if random == nil {
		random = rand.Reader
	}
	if length <= 0 {
		length = accessKeyMaxLen
	}
	if length < accessKeyMinLen {
		return nil, errors.New("auth: access key length is too short")
	}

	key := make([]byte, length)
	if _, err := io.ReadFull(random, key); err != nil {
		return nil, err
	}
	for i := range key {
		key[i] = alphaNumericTable[key[i]%alphaNumericTableLen]
	}
	return key, nil
}

// GenerateSecretKey returns a new secret key generated randomly using
// the given io.Reader. If random is nil, crypto/rand.Reader is used.
// If length <= 0, the secret key length is chosen automatically.
//
// GenerateSecretKey returns an error if length is too small for a valid
// secret key.
func GenerateSecretKey(length int, random io.Reader) ([]byte, error) {
	if random == nil {
		random = rand.Reader
	}
	if length <= 0 {
		length = secretKeyMaxLen
	}
	if length < secretKeyMinLen {
		return nil, errors.New("auth: secret key length is too short")
	}

	key := make([]byte, base64.RawStdEncoding.DecodedLen(length))
	if _, err := io.ReadFull(random, key); err != nil {
		return nil, err
	}
	out := make([]byte, length)
	base64.RawStdEncoding.Encode(out, key)
	bytes.ReplaceAll(out, []byte{'/'}, []byte{'+'})
	return out, nil
}
