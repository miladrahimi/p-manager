package enigma

import (
	"crypto/ed25519"
	"encoding/hex"
	"github.com/cockroachdb/errors"
	"os"
)

type Enigma struct {
	publicKeyPath string
	publicKey     ed25519.PublicKey
}

func (e *Enigma) Init() error {
	publicKeyData, err := os.ReadFile(e.publicKeyPath)
	if err != nil {
		return errors.Wrap(err, "cannot read public key file")
	}
	e.publicKey, err = hex.DecodeString(string(publicKeyData))
	if err != nil {
		return errors.Wrap(err, "cannot decode public key")
	}
	return nil
}

func (e *Enigma) Verify(plain, signature string) bool {
	s, err := hex.DecodeString(signature)
	if err != nil {
		return false
	}
	return ed25519.Verify(e.publicKey, []byte(plain), s)
}

func New(publicKeyPath string) *Enigma {
	return &Enigma{
		publicKeyPath: publicKeyPath,
	}
}
