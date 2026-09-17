package accounts

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"
)

func TestPasswordEncoding(t *testing.T) {
	a, b := hashPasswd("correct"), hashPasswd("correct")
	if a == b || !strings.HasPrefix(a, passwordPrefix) {
		t.Fatal("expected independent Argon2id salts")
	}
	if ok, old := verifyPassword(a, "correct"); !ok || old {
		t.Fatal("new password rejected")
	}
	if ok, _ := verifyPassword(a, "wrong"); ok {
		t.Fatal("wrong password accepted")
	}
	for _, encoded := range []string{"", "$argon2id$v=19$m=999999999,t=2,p=1$bad$bad", passwordPrefix + "bad$bad", passwordPrefix + "bad$bad$extra"} {
		if ok, _ := verifyPassword(encoded, "correct"); ok {
			t.Fatal("malformed hash accepted")
		}
	}
	legacy := sha256.Sum256([]byte("correct06Wm4Jv1Hkxx"))
	encoded := base64.StdEncoding.EncodeToString(legacy[:])
	if ok, old := verifyPassword(encoded, "correct"); !ok || !old {
		t.Fatal("legacy password not recognized")
	}
	if ok, _ := verifyPassword(encoded, "wrong"); ok {
		t.Fatal("wrong legacy password accepted")
	}
}
