// Package update preserves the version/API used by the pinned Classic agent.
// Install updates explicitly; never replace the running executable remotely.
package update

import "log"

var (
	CurrentVersion = "0.0.1"
	Repo           = "coolman1232004/komari-classic"
)

func DoUpdateWorks() {
	log.Println("Automatic binary updates are disabled in Classic; replace the Docker image or install a reviewed release manually.")
}

func CheckAndUpdate() error {
	DoUpdateWorks()
	return nil
}
