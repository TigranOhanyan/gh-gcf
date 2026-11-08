package commands

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"charm.land/log/v2"
)

// DevelopmentInstructionFingerprint is the SHA-256 of one role's instruction file.
// An empty file has a fingerprint. A missing file is an error.
func DevelopmentInstructionFingerprint(dir string, development string, role string) (sum string, err error) {
	path := filepath.Join(dir, "developments", development, role+".md")
	sum, err = fingerprintFile(path, role)
	return
}

func fingerprintFile(path string, name string) (sum string, err error) {
	body, err := os.ReadFile(path)
	missing := os.IsNotExist(err)
	if missing {
		err = fmt.Errorf("instruction %s is missing", name)
		log.Error("instruction", "path", path, "err", err)
		return
	}
	if err != nil {
		log.Error("instruction", "path", path, "err", err)
		return
	}
	digest := sha256.Sum256(body)
	sum = hex.EncodeToString(digest[:])
	return
}

// ReadDevelopmentInstruction returns the text of one role's instruction file.
func ReadDevelopmentInstruction(dir string, development string, role string) (text string, err error) {
	path := filepath.Join(dir, "developments", development, role+".md")
	body, err := os.ReadFile(path)
	missing := os.IsNotExist(err)
	if missing {
		err = fmt.Errorf("instruction %s is missing", role)
		log.Error("instruction", "path", path, "err", err)
		return
	}
	if err != nil {
		log.Error("instruction", "path", path, "err", err)
		return
	}
	text = string(body)
	return
}
