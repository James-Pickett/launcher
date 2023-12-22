//go:build darwin
// +build darwin

package main

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/kolide/krypto/pkg/challenge"
	"github.com/kolide/krypto/pkg/echelper"
	"github.com/kolide/krypto/pkg/secureenclave"
	"github.com/kolide/launcher/ee/agent/certs"
	"github.com/kolide/launcher/ee/secureenclavesigner"
	"github.com/kolide/launcher/pkg/backoff"
	"github.com/vmihailenco/msgpack/v5"
)

var serverPubKeys = make(map[string]*ecdsa.PublicKey)

func runSecureEnclave(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("not enough arguments, expect create_key <request> or sign <sign_request>")
	}

	if secureenclavesigner.Undertest {
		if secureenclavesigner.TestServerPubKey == "" {
			return fmt.Errorf("test server public key not set")
		}

		k, err := echelper.PublicB64DerToEcdsaKey([]byte(secureenclavesigner.TestServerPubKey))
		if err != nil {
			return fmt.Errorf("parsing test server public key: %w", err)
		}

		serverPubKeys[string(secureenclavesigner.TestServerPubKey)] = k
	}

	for _, keyStr := range []string{certs.K2EccServerCert, certs.ReviewEccServerCert, certs.LocalhostEccServerCert} {
		key, err := echelper.PublicPemToEcdsaKey([]byte(keyStr))
		if err != nil {
			return fmt.Errorf("parsing server public key from pem: %w", err)
		}

		pubB64Der, err := echelper.PublicEcdsaToB64Der(key)
		if err != nil {
			return fmt.Errorf("marshalling server public key to b64 der: %w", err)
		}

		serverPubKeys[string(pubB64Der)] = key
	}

	switch args[0] {
	case "create-key":
		return createSecureEnclaveKey(args[1])

	case "sign":
		return signWithSecureEnclave(args[1])

	default:
		return fmt.Errorf("unknown command %s", args[0])
	}
}

func createSecureEnclaveKey(requestB64 string) error {
	b, err := base64.StdEncoding.DecodeString(requestB64)
	if err != nil {
		return fmt.Errorf("decoding b64 request: %w", err)
	}

	var request secureenclavesigner.Request
	if err := msgpack.Unmarshal(b, &request); err != nil {
		return fmt.Errorf("unmarshaling msgpack request: %w", err)
	}

	if err := verifySecureEnclaveChallenge(request); err != nil {
		return fmt.Errorf("verifying challenge: %w", err)
	}

	secureEnclavePubKey, err := secureenclave.CreateKey()
	if err != nil {
		return fmt.Errorf("creating secure enclave key: %w", err)
	}

	secureEnclavePubDer, err := echelper.PublicEcdsaToB64Der(secureEnclavePubKey)
	if err != nil {
		return fmt.Errorf("marshalling public key to der: %w", err)
	}

	fmt.Println(string(secureEnclavePubDer))
	return nil
}

func signWithSecureEnclave(signRequestB64 string) error {
	b, err := base64.StdEncoding.DecodeString(signRequestB64)
	if err != nil {
		return fmt.Errorf("decoding b64 sign request: %w", err)
	}

	var signRequest secureenclavesigner.SignRequest
	if err := msgpack.Unmarshal(b, &signRequest); err != nil {
		return fmt.Errorf("unmarshaling msgpack sign request: %w", err)
	}

	if err := verifySecureEnclaveChallenge(signRequest.Request); err != nil {
		return fmt.Errorf("verifying challenge: %w", err)
	}

	secureEnclavePubKey, err := echelper.PublicB64DerToEcdsaKey(signRequest.SecureEnclavePubKey)
	if err != nil {
		return fmt.Errorf("marshalling b64 der to public key: %w", err)
	}

	ses, err := secureenclave.New(secureEnclavePubKey)
	if err != nil {
		return fmt.Errorf("creating secure enclave signer: %w", err)
	}
	var sig []byte
	backoff.WaitFor(func() error {
		sig, err = ses.Sign(rand.Reader, signRequest.Digest, crypto.SHA256)
		return err
	}, 250*time.Millisecond, 2*time.Second)

	if err != nil {
		return fmt.Errorf("signing request: %w", err)
	}

	fmt.Print(base64.StdEncoding.EncodeToString(sig))
	return nil
}

func verifySecureEnclaveChallenge(request secureenclavesigner.Request) error {
	c, err := challenge.UnmarshalChallenge(request.Challenge)
	if err != nil {
		return fmt.Errorf("unmarshaling challenge: %w", err)
	}

	serverPubKey, ok := serverPubKeys[string(request.ServerPubKey)]
	if !ok {
		return fmt.Errorf("server public key not found")
	}

	if err := c.Verify(*serverPubKey); err != nil {
		return fmt.Errorf("verifying challenge: %w", err)
	}

	// TODO verify time stamp
	return nil
}
