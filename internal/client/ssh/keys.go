package ssh

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
)

const (
	// DefaultKeyBits is the default number of bits for RSA keys
	DefaultKeyBits = 4096

	// DefaultKeyPermissions is the default file permissions for private keys (0600 - read/write for owner only)
	DefaultKeyPermissions = 0600

	// DefaultPublicKeyPermissions is the default file permissions for public keys (0644 - read for all, write for owner)
	DefaultPublicKeyPermissions = 0644
)

// KeyManager handles SSH key operations
type KeyManager interface {
	// GenerateKeys generates a new SSH key pair
	GenerateKeys(keyPath string, bits int) error

	// LoadPublicKey loads a public key from a file
	LoadPublicKey(keyPath string) (ssh.PublicKey, error)

	// LoadPrivateKey loads a private key from a file
	LoadPrivateKey(keyPath string) (ssh.Signer, error)

	// KeyExists checks if a key pair exists at the given path
	KeyExists(keyPath string) bool
}

// keyManagerImpl implements the KeyManager interface
type keyManagerImpl struct{}

// NewKeyManager creates a new SSH key manager
func NewKeyManager() KeyManager {
	return &keyManagerImpl{}
}

// GenerateKeys generates a new SSH key pair
func (km *keyManagerImpl) GenerateKeys(keyPath string, bits int) error {
	if bits <= 0 {
		bits = DefaultKeyBits
	}

	// Create directory if it doesn't exist
	keyDir := filepath.Dir(keyPath)
	if err := os.MkdirAll(keyDir, 0700); err != nil {
		return errors.Wrapf(err, "failed to create directory %s", keyDir)
	}

	// Generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return errors.Wrap(err, "failed to generate RSA key")
	}

	// Encode private key to PEM
	privateKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}

	// Write private key to file
	privateKeyFile, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, DefaultKeyPermissions)
	if err != nil {
		return errors.Wrapf(err, "failed to create private key file %s", keyPath)
	}
	defer privateKeyFile.Close()

	if err := pem.Encode(privateKeyFile, privateKeyPEM); err != nil {
		return errors.Wrap(err, "failed to write private key")
	}

	// Generate public key
	publicKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return errors.Wrap(err, "failed to generate public key")
	}

	// Write public key to file
	publicKeyPath := keyPath + ".pub"
	publicKeyFile, err := os.OpenFile(publicKeyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, DefaultPublicKeyPermissions)
	if err != nil {
		return errors.Wrapf(err, "failed to create public key file %s", publicKeyPath)
	}
	defer publicKeyFile.Close()

	publicKeyBytes := ssh.MarshalAuthorizedKey(publicKey)
	if _, err := publicKeyFile.Write(publicKeyBytes); err != nil {
		return errors.Wrap(err, "failed to write public key")
	}

	return nil
}

// LoadPublicKey loads a public key from a file
func (km *keyManagerImpl) LoadPublicKey(keyPath string) (ssh.PublicKey, error) {
	// If the path is a private key, append .pub to get the public key path
	if !strings.HasSuffix(keyPath, ".pub") {
		keyPath = keyPath + ".pub"
	}

	// Read public key file
	publicKeyBytes, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read public key file %s", keyPath)
	}

	// Parse public key
	publicKey, _, _, _, err := ssh.ParseAuthorizedKey(publicKeyBytes)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse public key")
	}

	return publicKey, nil
}

// LoadPrivateKey loads a private key from a file
func (km *keyManagerImpl) LoadPrivateKey(keyPath string) (ssh.Signer, error) {
	// Read private key file
	privateKeyBytes, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read private key file %s", keyPath)
	}

	// Parse private key
	signer, err := ssh.ParsePrivateKey(privateKeyBytes)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse private key")
	}

	return signer, nil
}

// KeyExists checks if a key pair exists at the given path
func (km *keyManagerImpl) KeyExists(keyPath string) bool {
	// Check if private key exists
	if _, err := os.Stat(keyPath); err != nil {
		return false
	}

	// Check if public key exists
	publicKeyPath := keyPath + ".pub"
	if _, err := os.Stat(publicKeyPath); err != nil {
		return false
	}

	return true
}

// GetDefaultKeyPath returns the default path for SSH keys
func GetDefaultKeyPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current directory if home directory can't be determined
		return ".ssh/id_rsa"
	}
	return filepath.Join(homeDir, ".dockbridge", "ssh", "id_rsa")
}

// EnsureKeyPairExists ensures that an SSH key pair exists at the given path
// If the key pair doesn't exist, it will be generated
func EnsureKeyPairExists(keyPath string, bits int) error {
	km := NewKeyManager()
	if km.KeyExists(keyPath) {
		return nil
	}

	// Create directory if it doesn't exist
	keyDir := filepath.Dir(keyPath)
	if err := os.MkdirAll(keyDir, 0700); err != nil {
		return errors.Wrapf(err, "failed to create directory %s", keyDir)
	}

	return km.GenerateKeys(keyPath, bits)
}

// GetPublicKeyString returns the public key as a string in the authorized_keys format
func GetPublicKeyString(keyPath string) (string, error) {
	km := NewKeyManager()

	// If the path is a private key, append .pub to get the public key path
	if !strings.HasSuffix(keyPath, ".pub") {
		keyPath = keyPath + ".pub"
	}

	// Read public key file
	publicKeyBytes, err := os.ReadFile(keyPath)
	if err != nil {
		return "", errors.Wrapf(err, "failed to read public key file %s", keyPath)
	}

	return string(publicKeyBytes), nil
}
