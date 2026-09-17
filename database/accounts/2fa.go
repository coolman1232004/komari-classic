package accounts

import (
	"fmt"
	"image"
	"sync"
	"time"

	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/pquerna/otp/totp"
)

var (
	TwoFactorIssuer = "Komari Monitor"
)

func Generate2Fa() (string, image.Image, error) {
	otp, err := totp.Generate(totp.GenerateOpts{
		Issuer:      TwoFactorIssuer,
		AccountName: "komari",
	})
	if err != nil {
		return "", nil, err
	}
	img, err := otp.Image(250, 250)
	if err != nil {
		return "", nil, err
	}
	return otp.Secret(), img, nil
}

func Enable2Fa(uuid, secret string) error {
	db := dbcore.GetDBInstance()
	result := db.Model(&models.User{}).Where("uuid = ? AND (two_factor = ? OR two_factor IS NULL)", uuid, "").Update("two_factor", secret)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("2FA already enabled or user missing")
	}
	return nil
}

var twoFAMutex sync.Mutex
var twoFAAttempts = make(map[string]struct {
	count  int
	expiry time.Time
})

func Verify2Fa(uuid, code string) (bool, error) {
	twoFAMutex.Lock()
	now := time.Now()
	for id, entry := range twoFAAttempts {
		if !now.Before(entry.expiry) {
			delete(twoFAAttempts, id)
		}
	}
	entry := twoFAAttempts[uuid]
	if entry.count >= 10 || (entry.count == 0 && len(twoFAAttempts) >= 4096) {
		twoFAMutex.Unlock()
		return false, fmt.Errorf("too many 2FA attempts; wait five minutes")
	}
	if entry.count == 0 {
		entry.expiry = now.Add(5 * time.Minute)
	}
	entry.count++
	twoFAAttempts[uuid] = entry
	twoFAMutex.Unlock()

	db := dbcore.GetDBInstance()
	var user models.User
	err := db.Where("uuid = ?", uuid).First(&user).Error
	if err != nil {
		return false, err
	}

	if user.TwoFactor == "" {
		return false, nil // 用户未启用2FA
	}

	valid := totp.Validate(code, user.TwoFactor)
	if !valid {
		return false, nil
	}

	return true, nil
}

func Disable2Fa(uuid string) error {
	db := dbcore.GetDBInstance()
	return db.Model(&models.User{}).Where("uuid = ?", uuid).Update("two_factor", "").Error
}
