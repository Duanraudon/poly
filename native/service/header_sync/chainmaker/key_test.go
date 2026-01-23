/*
* @Author Duanraudon
* @Description
* @FileName key_test.go
* @ProductName GoLand
* @Date 2026/1/16 09:22
 */

package chainmaker

import (
	"encoding/hex"
	"fmt"
	"log"
	"testing"
)

func TestKey(t *testing.T) {
	pri := `-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIIjRSRhOeGQkFXwOE4L+a5tgZ8G0V/HIRYZFq/Me0DdtoAoGCCqGSM49
AwEHoUQDQgAEbcgTRqhs89i25kgrF8yiGhL9RPO0re1EX/JRq+09qm9a3r/Zo4kZ
A/9SLlAbPzTHJQhFxNWKGWtVw8HFdSkmlA==
-----END EC PRIVATE KEY-----
`
	pub := `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEbcgTRqhs89i25kgrF8yiGhL9RPO0
re1EX/JRq+09qm9a3r/Zo4kZA/9SLlAbPzTHJQhFxNWKGWtVw8HFdSkmlA==
-----END PUBLIC KEY-----
`
	SIGN_DATA := []byte("hello world")
	signature, err := SignData([]byte(pri), SIGN_DATA)
	if err != nil {
		log.Fatal(err)
	}

	sigToString := hex.EncodeToString(signature)
	decodeString, err := hex.DecodeString(sigToString)
	if err != nil {
		log.Fatal(err)
	}

	VERIFY, err := VerifySignature(SIGN_DATA, decodeString, []byte(pub))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(VERIFY)
}
