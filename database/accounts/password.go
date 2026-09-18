package accounts

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"strings"

	"golang.org/x/crypto/argon2"
)

// OWASP minimum Argon2id profile: 19 MiB, two iterations, one lane.
const passwordPrefix = "$argon2id$v=19$m=19456,t=2,p=1$"

var passwordChecks = make(chan struct{}, 4)
var dummyPasswordHash = hashPasswd("non-account-dummy-password")

func hashPasswd(password string) string {
	salt := make([]byte, 16)
	// crypto/rand.Read in our Go toolchain never returns an error.
	rand.Read(salt)
	key := argon2.IDKey([]byte(password), salt, 2, 19*1024, 1, 32)
	return passwordPrefix + base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(key)
}

func verifyPassword(encoded, password string) (valid, legacy bool) {
	if strings.HasPrefix(encoded, passwordPrefix) {
		parts := strings.Split(strings.TrimPrefix(encoded, passwordPrefix), "$")
		if len(parts) != 2 {
			return false, false
		}
		salt, err := base64.RawStdEncoding.DecodeString(parts[0])
		if err != nil || len(salt) != 16 {
			return false, false
		}
		want, err := base64.RawStdEncoding.DecodeString(parts[1])
		if err != nil || len(want) != 32 {
			return false, false
		}
		got := argon2.IDKey([]byte(password), salt, 2, 19*1024, 1, 32)
		return subtle.ConstantTimeCompare(got, want) == 1, false
	}
	// Only the exact old encoding is eligible for migration. Reject unknown
	// formats/parameters instead of interpreting attacker-controlled costs.
	want, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(want) != sha256.Size {
		return false, false
	}
	got := sha256.Sum256([]byte(password + "06Wm4Jv1Hkxx"))
	return subtle.ConstantTimeCompare(got[:], want) == 1, true
}
