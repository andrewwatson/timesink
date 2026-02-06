//go:build darwin

package crypto

import (
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"
)

type darwinKeyring struct{}

func newPlatformKeyring() Keyring {
	return &darwinKeyring{}
}

// GetKey retrieves the encryption key from macOS Keychain
func (k *darwinKeyring) GetKey() (string, error) {
	key, err := keyring.Get(ServiceName, KeyName)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return "", fmt.Errorf("encryption key not found in keychain: %w", err)
		}
		return "", fmt.Errorf("failed to retrieve key from keychain: %w", err)
	}

	if key == "" {
		return "", errors.New("encryption key is empty")
	}

	return key, nil
}

// SetKey stores the encryption key in macOS Keychain
func (k *darwinKeyring) SetKey(password string) error {
	if password == "" {
		return errors.New("password cannot be empty")
	}

	err := keyring.Set(ServiceName, KeyName, password)
	if err != nil {
		return fmt.Errorf("failed to store key in keychain: %w", err)
	}

	return nil
}

// DeleteKey removes the encryption key from macOS Keychain
func (k *darwinKeyring) DeleteKey() error {
	err := keyring.Delete(ServiceName, KeyName)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return fmt.Errorf("encryption key not found in keychain: %w", err)
		}
		return fmt.Errorf("failed to delete key from keychain: %w", err)
	}

	return nil
}

// IsAvailable checks if the macOS Keychain is accessible
func (k *darwinKeyring) IsAvailable() bool {
	// Test keychain availability by attempting a dummy operation
	// We use a test key that we immediately delete
	testKey := "__timesink_availability_test__"
	err := keyring.Set(ServiceName, testKey, "test")
	if err != nil {
		return false
	}

	// Clean up test key
	_ = keyring.Delete(ServiceName, testKey)
	return true
}
