package auth

import (
	"crypto/md5"
	"encoding/hex"
)

func MD5Hash(plain string) string {
	sum := md5.Sum([]byte(plain))
	return hex.EncodeToString(sum[:])
}
