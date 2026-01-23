/*
* @Author Duanraudon
* @Description
* @FileName keyUtils.go
* @ProductName GoLand
* @Date 2026/1/16 09:53
 */

package chainmaker

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/big"
)

// SignData signs the data using the provided private key
func SignData(privateKeyBytes []byte, data []byte) ([]byte, error) {
	// Parse the private key from PEM format
	privateKeyPem, _ := pem.Decode(privateKeyBytes)
	if privateKeyPem == nil {
		return nil, fmt.Errorf("failed to decode PEM block containing private key")
	}

	privateKey, err := x509.ParseECPrivateKey(privateKeyPem.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %v", err)
	}

	// Hash the data using SHA256
	hash := sha256.Sum256(data)

	// Sign the hash
	r, s, err := ecdsa.Sign(rand.Reader, privateKey, hash[:])
	if err != nil {
		return nil, fmt.Errorf("failed to sign data: %v", err)
	}

	// Serialize the signature as r||s (concatenated)
	sig := make([]byte, 64) // Assuming curve P256, which gives 32-byte r and s
	copy(sig[:32], r.Bytes())
	copy(sig[32:], s.Bytes())

	return sig, nil
}

// VerifySignature verifies the signature against the data and public key
func VerifySignature(data []byte, signature []byte, publicKeyBytes []byte) (bool, error) {
	if len(signature) != 64 {
		return false, fmt.Errorf("invalid signature length: expected 64 bytes, got %d", len(signature))
	}

	// Parse the public key from PEM format
	publicKeyPem, _ := pem.Decode(publicKeyBytes)
	if publicKeyPem == nil {
		return false, fmt.Errorf("failed to decode PEM block containing public key")
	}

	publicKey, err := x509.ParsePKIXPublicKey(publicKeyPem.Bytes)
	if err != nil {
		return false, fmt.Errorf("failed to parse public key: %v", err)
	}

	ecdsaPublicKey, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return false, fmt.Errorf("not an ECDSA public key")
	}

	// Split the signature into r and s components
	r := new(big.Int).SetBytes(signature[:32])
	s := new(big.Int).SetBytes(signature[32:])

	// Hash the data (same as during signing)
	hash := sha256.Sum256(data)

	// Verify the signature
	isValid := ecdsa.Verify(ecdsaPublicKey, hash[:], r, s)
	return isValid, nil
}
