//go:build !darwin

package crypto

import (
	"errors"
	"fmt"
	"os"
)

type fallbackKeyring struct{}

func newPlatformKeyring() Keyring {
	return &fallbackKeyring{}
}

// GetKey retrieves the encryption key from TIMESINK_DB_KEY environment variable
func (k *fallbackKeyring) GetKey() (string, error) {
	key := os.Getenv("TIMESINK_DB_KEY")
	if key == "" {
		return "", errors.New("TIMESINK_DB_KEY environment variable not set")
	}

	return key, nil
}

// SetKey returns an error suggesting to set the environment variable
func (k *fallbackKeyring) SetKey(password string) error {
	if password == "" {
		return errors.New("password cannot be empty")
	}

	return fmt.Errorf("keyring not available on this platform: please set TIMESINK_DB_KEY environment variable to '%s'", password)
}

// DeleteKey returns an error suggesting to unset the environment variable
func (k *fallbackKeyring) DeleteKey() error {
	return errors.New("keyring not available on this platform: please unset TIMESINK_DB_KEY environment variable manually")
}

// IsAvailable checks if the TIMESINK_DB_KEY environment variable is set
func (k *fallbackKeyring) IsAvailable() bool {
	return os.Getenv("TIMESINK_DB_KEY") != ""
}
