package commands

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"charm.land/log/v2"
)

// InstructionFingerprint is the SHA-256 of the member's instruction file.
// An empty file has a fingerprint. A missing file is an error.
func InstructionFingerprint(dir string, name string) (sum string, err error) {
	path := filepath.Join(dir, "members", name+".md")
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

// ReadInstruction returns the text of the member's instruction file.
func ReadInstruction(dir string, name string) (text string, err error) {
	path := filepath.Join(dir, "members", name+".md")
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
	text = string(body)
	return
}
