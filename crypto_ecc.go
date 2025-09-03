package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"fmt"

	"golang.org/x/crypto/nacl/box"
)

// ECCKeyPair holds both X25519 (encryption) and Ed25519 (signing) keys
type ECCKeyPair struct {
	EncryptionPrivate []byte // X25519 private key (32 bytes)
	EncryptionPublic  []byte // X25519 public key (32 bytes)
	SigningPrivate    ed25519.PrivateKey // Ed25519 private key (64 bytes)
	SigningPublic     ed25519.PublicKey  // Ed25519 public key (32 bytes)
}

// PublicKeyBundle is what gets transmitted - both public keys
type PublicKeyBundle struct {
	EncryptionKey []byte // X25519 public key (32 bytes)
	SigningKey    []byte // Ed25519 public key (32 bytes)
}

// GenerateECCKeyPair generates both X25519 and Ed25519 keypairs
func GenerateECCKeyPair() (*ECCKeyPair, error) {
	// Generate X25519 keypair for encryption using NaCl box
	encPub, encPriv, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	
	// Generate Ed25519 keypair for signing
	sigPub, sigPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	
	return &ECCKeyPair{
		EncryptionPrivate: encPriv[:],
		EncryptionPublic:  encPub[:],
		SigningPrivate:    sigPriv,
		SigningPublic:     sigPub,
	}, nil
}

// EncodePublicKeys encodes both public keys for DNS transmission
func EncodePublicKeys(encPub, sigPub []byte) string {
	// Concatenate both 32-byte public keys (64 bytes total)
	bundle := append(encPub, sigPub...)
	// Base32 encode without padding (64 bytes → ~103 chars)
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(bundle)
}

// DecodeServerPublicKeys decodes both server public keys from single base32 string
func DecodeServerPublicKeys(encoded string) (encPub, sigPub []byte, err error) {
	bundle, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(encoded)
	if err != nil {
		return nil, nil, err
	}
	
	if len(bundle) != 64 {
		return nil, nil, fmt.Errorf("invalid public key bundle size: %d", len(bundle))
	}
	
	return bundle[:32], bundle[32:], nil
}

// NaClEncrypt encrypts data using NaCl box (X25519 + XSalsa20-Poly1305)
func NaClEncrypt(plaintext []byte, senderPriv []byte, recipientPub []byte) ([]byte, error) {
	// Generate nonce (24 bytes for NaCl)
	nonce := make([]byte, 24)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	
	// Check key sizes
	if len(senderPriv) != 32 || len(recipientPub) != 32 {
		return nil, errors.New("invalid key size")
	}
	
	// Copy keys to arrays
	var senderPrivArray, recipientPubArray [32]byte
	copy(senderPrivArray[:], senderPriv)
	copy(recipientPubArray[:], recipientPub)
	
	// Encrypt using NaCl box
	var nonceArray [24]byte
	copy(nonceArray[:], nonce)
	
	// NaCl box adds 16-byte authenticator
	ciphertext := make([]byte, len(plaintext)+16)
	encrypted := box.Seal(ciphertext[:0], plaintext, &nonceArray, &recipientPubArray, &senderPrivArray)
	
	// Return: nonce[24] || ciphertext[...]
	result := make([]byte, 24+len(encrypted))
	copy(result[0:24], nonce)
	copy(result[24:], encrypted)
	
	return result, nil
}

// NaClDecrypt decrypts data encrypted with NaClEncrypt
func NaClDecrypt(ciphertext []byte, recipientPriv []byte, senderPub []byte) ([]byte, error) {
	if len(ciphertext) < 24 {
		return nil, errors.New("ciphertext too short")
	}
	
	// Extract nonce and encrypted data
	nonce := ciphertext[0:24]
	encrypted := ciphertext[24:]
	
	// Check key sizes
	if len(recipientPriv) != 32 || len(senderPub) != 32 {
		return nil, errors.New("invalid key size")
	}
	
	// Copy to arrays
	var recipientPrivArray, senderPubArray [32]byte
	var nonceArray [24]byte
	copy(recipientPrivArray[:], recipientPriv)
	copy(senderPubArray[:], senderPub)
	copy(nonceArray[:], nonce)
	
	// Decrypt
	plaintext, ok := box.Open(nil, encrypted, &nonceArray, &senderPubArray, &recipientPrivArray)
	if !ok {
		return nil, errors.New("decryption failed")
	}
	
	return plaintext, nil
}

// Ed25519Sign signs data with Ed25519
func Ed25519Sign(data []byte, privKey ed25519.PrivateKey) []byte {
	return ed25519.Sign(privKey, data)
}

// Ed25519Verify verifies Ed25519 signature
func Ed25519Verify(data, signature []byte, pubKey ed25519.PublicKey) bool {
	return ed25519.Verify(pubKey, data, signature)
}

// PackSignedEncrypted combines signature and encrypted data
func PackSignedEncrypted(signature, encrypted []byte) []byte {
	// signature[64] || encrypted[...]
	return append(signature, encrypted...)
}

// UnpackSignedEncrypted splits signature and encrypted data
func UnpackSignedEncrypted(data []byte) (signature, encrypted []byte, err error) {
	if len(data) < 64 {
		return nil, nil, errors.New("data too short for signature")
	}
	return data[:64], data[64:], nil
}

// Base32EncodeNoPad encodes to base32 without padding
func Base32EncodeNoPad(data []byte) string {
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(data)
}

// Base32DecodeNoPad decodes from base32 without padding
func Base32DecodeNoPad(s string) ([]byte, error) {
	return base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(s)
}

// Base64Encode encodes to standard base64
func Base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// Base64Decode decodes from standard base64
func Base64Decode(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

// DeriveXORKey derives a deterministic XOR key from shared secret and context
func DeriveXORKey(sharedSecret []byte, context string, length int) []byte {
	// Use HKDF or simple hash chain for key derivation
	// For now, simple approach: hash(secret || context || counter)
	key := make([]byte, length)
	
	for i := 0; i < length; i += 32 {
		// Create input for this block
		input := append(sharedSecret, []byte(context)...)
		input = append(input, byte(i/32))
		
		// Hash it
		hash := sha256.Sum256(input)
		
		// Copy to output
		copyLen := 32
		if i+32 > length {
			copyLen = length - i
		}
		copy(key[i:], hash[:copyLen])
	}
	
	return key
}

// XOREncrypt encrypts data with XOR - zero overhead
func XOREncrypt(plaintext, key []byte) []byte {
	if len(key) < len(plaintext) {
		panic("XOR key too short")
	}
	
	ciphertext := make([]byte, len(plaintext))
	for i := range plaintext {
		ciphertext[i] = plaintext[i] ^ key[i]
	}
	return ciphertext
}

// XORDecrypt decrypts data with XOR - same as encrypt
func XORDecrypt(ciphertext, key []byte) []byte {
	return XOREncrypt(ciphertext, key) // XOR is symmetric
}

// DeriveSharedSecret performs ECDH to get shared secret
func DeriveSharedSecret(privateKey, publicKey []byte) ([]byte, error) {
	if len(privateKey) != 32 || len(publicKey) != 32 {
		return nil, errors.New("keys must be 32 bytes")
	}
	
	// Use nacl box.Precompute which does X25519 ECDH
	var sharedKey [32]byte
	var privKey [32]byte
	var pubKey [32]byte
	copy(privKey[:], privateKey)
	copy(pubKey[:], publicKey)
	
	box.Precompute(&sharedKey, &pubKey, &privKey)
	return sharedKey[:], nil
}