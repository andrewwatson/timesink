package crypto

// Keyring provides secure key storage abstraction
type Keyring interface {
	GetKey() (string, error)
	SetKey(password string) error
	DeleteKey() error
	IsAvailable() bool
}

const (
	ServiceName = "timesink"
	KeyName     = "db-encryption-key"
)

// NewKeyring returns the best available keyring implementation
func NewKeyring() Keyring {
	return newPlatformKeyring()
}
