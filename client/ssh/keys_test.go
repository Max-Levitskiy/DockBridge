package ssh

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKeyManager_GenerateKeys(t *testing.T) {
	// Create a temporary directory for test keys
	tempDir, err := os.MkdirTemp("", "ssh-keys-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	keyPath := filepath.Join(tempDir, "id_rsa")

	// Create key manager
	km := NewKeyManager()

	// Generate keys
	err = km.GenerateKeys(keyPath, DefaultKeyBits)
	require.NoError(t, err)

	// Verify that both private and public keys were created
	assert.FileExists(t, keyPath)
	assert.FileExists(t, keyPath+".pub")

	// Check file permissions
	privateKeyInfo, err := os.Stat(keyPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(DefaultKeyPermissions), privateKeyInfo.Mode().Perm())

	publicKeyInfo, err := os.Stat(keyPath + ".pub")
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(DefaultPublicKeyPermissions), publicKeyInfo.Mode().Perm())
}

func TestKeyManager_LoadKeys(t *testing.T) {
	// Create a temporary directory for test keys
	tempDir, err := os.MkdirTemp("", "ssh-keys-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	keyPath := filepath.Join(tempDir, "id_rsa")

	// Create key manager
	km := NewKeyManager()

	// Generate keys
	err = km.GenerateKeys(keyPath, DefaultKeyBits)
	require.NoError(t, err)

	// Test loading private key
	privateKey, err := km.LoadPrivateKey(keyPath)
	require.NoError(t, err)
	assert.NotNil(t, privateKey)

	// Test loading public key
	publicKey, err := km.LoadPublicKey(keyPath)
	require.NoError(t, err)
	assert.NotNil(t, publicKey)

	// Test loading public key with explicit .pub suffix
	publicKey2, err := km.LoadPublicKey(keyPath + ".pub")
	require.NoError(t, err)
	assert.NotNil(t, publicKey2)

	// Verify that both loaded public keys are the same
	assert.Equal(t, publicKey.Type(), publicKey2.Type())
	assert.Equal(t, publicKey.Marshal(), publicKey2.Marshal())
}

func TestKeyManager_KeyExists(t *testing.T) {
	// Create a temporary directory for test keys
	tempDir, err := os.MkdirTemp("", "ssh-keys-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	keyPath := filepath.Join(tempDir, "id_rsa")
	nonExistentKeyPath := filepath.Join(tempDir, "non_existent")

	// Create key manager
	km := NewKeyManager()

	// Check non-existent key
	assert.False(t, km.KeyExists(nonExistentKeyPath))

	// Generate keys
	err = km.GenerateKeys(keyPath, DefaultKeyBits)
	require.NoError(t, err)

	// Check existing key
	assert.True(t, km.KeyExists(keyPath))

	// Remove public key and check again
	err = os.Remove(keyPath + ".pub")
	require.NoError(t, err)
	assert.False(t, km.KeyExists(keyPath))
}
