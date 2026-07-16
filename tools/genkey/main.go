// Generates a fresh key for the companion extension: prints the manifest
// "key" (base64 SPKI public key) and the extension ID Chrome derives from it.
// The private key is intentionally discarded — unpacked extensions only need
// the public key to pin their ID. Re-running produces a NEW id; the manifest
// and internal/companion.ExtensionID must then be updated together.
package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"fmt"
)

func main() {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	spki, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		panic(err)
	}
	sum := sha256.Sum256(spki)
	id := make([]byte, 32)
	for i := 0; i < 16; i++ {
		id[2*i] = 'a' + sum[i]>>4
		id[2*i+1] = 'a' + sum[i]&0xf
	}
	fmt.Printf("key: %s\nid:  %s\n", base64.StdEncoding.EncodeToString(spki), id)
}
