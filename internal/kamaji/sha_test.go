package kamaji

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

//cat sha_test.crt | openssl x509 -pubkey | openssl pkey -pubin -outform DER | openssl dgst -sha256 | sed 's/^.* //'

func TestSha256Crt(t *testing.T) {
	certPath := "sha_test.crt"

	// Step 1: Read the certificate
	data, err := os.ReadFile(certPath)
	if err != nil {
		log.Fatalf("Failed to read certificate: %v", err)
	}

	// Step 2: Decode PEM
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "CERTIFICATE" {
		log.Fatalf("Failed to decode PEM block containing certificate")
	}
	blockBase64 := base64.StdEncoding.EncodeToString(block.Bytes)
	fmt.Println("Base64 Encoded Certificate:")
	fmt.Println(blockBase64)

	// Step 3: Parse X.509 certificate
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		log.Fatalf("Failed to parse certificate: %v", err)
	}

	// Step 4: Marshal public key in DER (PKIX) format
	pubKeyDER, err := x509.MarshalPKIXPublicKey(cert.PublicKey)
	if err != nil {
		log.Fatalf("Failed to marshal public key: %v", err)
	}

	// Step 5: Print DER public key as base64
	pubKeyBase64 := base64.StdEncoding.EncodeToString(pubKeyDER)
	fmt.Println("DER Public Key (base64):")
	fmt.Println(pubKeyBase64)
	require.Equal(t, "MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA45hkyyVDhtRgcCjtC9Fgh/9i7ojjEm4Bf9uU72m2viM6AsUF6toVJNGOhXk2PaoP2mZQ2CZ4HyaO9fLuX/QXWH8nKlaInVzlQby+JFZ5K5VRHyAuCGwbHt6ueVkAGfFUD9Z17Ui7So6cSEfn6+AGM7IabKFSN73DnUmOguwzT9I5ONIyygNQHwn0Y97LoMOD/jicFloWeszZSr8NzfisSptvdYrSZ/IVqvrbi18jOzSHdOpAB4TCVF3e50Nb5VzgCpDO1S1Y3JuwpNNmiHFOTh/SajK59liDBeCHEX/pdG5U7tUmctO1PjiuKJaSVqAx2wLO1Hw6BVDoY3oeWdaaGQIDAQAB", pubKeyBase64)

	// Step 6: SHA-256 hash of the DER
	hash := sha256.Sum256(pubKeyDER)
	fmt.Println("\nSHA-256 Fingerprint:")
	encHash := hex.EncodeToString(hash[:])
	fmt.Println(encHash)

	require.Equal(t, "b1e40b13c33172005655bf2cf8aed10ec7c2125eb0aeffc8e24fc465269b0ae6", encHash)
}
