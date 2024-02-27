package enigma

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"os"
)

type Enigma struct {
	PublicKeyPath string
	PublicKey     ed25519.PublicKey
}

func (e *Enigma) Init() error {
	publicKeyData, err := os.ReadFile(e.PublicKeyPath)
	if err != nil {
		return fmt.Errorf("enigma: Init: cannot load public key file, err: %v", err)
	}

	e.PublicKey, err = hex.DecodeString(string(publicKeyData))
	if err != nil {
		return fmt.Errorf("enigma: Init: cannot decode public key, err: %v", err)
	}

	return nil
}

func (e *Enigma) Verify(plain, signature []byte) bool {
	s, _ := hex.DecodeString(string(signature))
	return ed25519.Verify(e.PublicKey, plain, s)
}

func New(publicKeyPath string) *Enigma {
	return &Enigma{
		PublicKeyPath: publicKeyPath,
	}
}
