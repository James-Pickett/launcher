//go:build darwin
// +build darwin

package secureenclavesigner

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"encoding/base64"
	"fmt"
	"os"
	"os/user"
	"strings"
	"time"

	"github.com/kolide/krypto/pkg/echelper"
	"github.com/kolide/launcher/ee/allowedcmd"
	"github.com/vmihailenco/msgpack/v5"
)

const (
	CreateKeyCmd = "create-key"
	SignCmd      = "sign"
)

type Opt func(*secureEnclaveSigner)

// WithExistingKey allows you to pass the public portion of an existing
// secure enclave key to use for signing
func WithExistingKey(publicKey *ecdsa.PublicKey) Opt {
	return func(ses *secureEnclaveSigner) {
		ses.pubKey = publicKey
	}
}

type secureEnclaveSigner struct {
	// uid is the uid of the user to run the secure enclave commands as
	uid string
	// username is the username of the user to run the secure enclave commands as
	username             string
	serverPubKeyB64Der   []byte
	challenge            []byte
	pubKey               *ecdsa.PublicKey
	pathToLauncherBinary string
}

func New(signingUid string, serverPubKeyB64Der []byte, challenge []byte, opts ...Opt) (*secureEnclaveSigner, error) {
	ses := &secureEnclaveSigner{
		serverPubKeyB64Der: serverPubKeyB64Der,
		challenge:          challenge,
	}

	for _, opt := range opts {
		opt(ses)
	}

	// look up user by uid
	u, err := user.LookupId(signingUid)
	if err != nil {
		return nil, fmt.Errorf("looking up user by uid: %w", err)
	}

	ses.uid = u.Uid
	ses.username = u.Username

	if ses.pathToLauncherBinary == "" {
		p, err := os.Executable()
		if err != nil {
			return nil, fmt.Errorf("getting path to launcher binary: %w", err)
		}

		ses.pathToLauncherBinary = p
	}

	return ses, nil
}

// Public returns the public key of the secure enclave signer
// it creates a new public key using secure enclave if a public key
// is not set
func (ses *secureEnclaveSigner) Public() crypto.PublicKey {
	if ses.pubKey != nil {
		return ses.pubKey
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := ses.createKey(ctx); err != nil {
		return nil
	}

	return ses.pubKey
}

// Sign signs the digest using the secure enclave
// If a public key is not set, it will create a new key
func (ses *secureEnclaveSigner) Sign(baseNonce string, data []byte) (*SignResponseOuter, error) {
	// create the key if we don't have it
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if ses.pubKey == nil {
		if err := ses.createKey(ctx); err != nil {
			return nil, fmt.Errorf("creating key: %w", err)
		}
	}

	pubKeyBytes, err := echelper.PublicEcdsaToB64Der(ses.pubKey)
	if err != nil {
		return nil, fmt.Errorf("marshalling public key to der: %w", err)
	}

	signRequest := SignRequest{
		SecureEnclaveRequest: SecureEnclaveRequest{
			Challenge:    ses.challenge,
			ServerPubKey: ses.serverPubKeyB64Der,
		},
		BaseNonce:           baseNonce,
		Data:                data,
		SecureEnclavePubKey: pubKeyBytes,
	}

	signRequestMsgPack, err := msgpack.Marshal(signRequest)
	if err != nil {
		return nil, fmt.Errorf("marshalling sign request to msgpack: %w", err)
	}

	cmd, err := allowedcmd.Launchctl(
		ctx,
		"asuser",
		ses.uid,
		"sudo",
		"--preserve-env",
		"-u",
		ses.username,
		ses.pathToLauncherBinary,
		"secure-enclave",
		SignCmd,
		base64.StdEncoding.EncodeToString(signRequestMsgPack),
	)

	if err != nil {
		return nil, fmt.Errorf("creating command to sign: %w", err)
	}

	// skip updates since we have full path of binary
	cmd.Env = append(cmd.Environ(), fmt.Sprintf("%s=%s", "LAUNCHER_SKIP_UPDATES", "true"))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("executing launcher binary to sign: %w: %s", err, string(out))
	}

	responseRaw := []byte(lastLine(out))

	responseBytes, err := base64.StdEncoding.DecodeString(string(responseRaw))
	if err != nil {
		return nil, fmt.Errorf("decoding response from base64: %w", err)
	}

	var outerResponse SignResponseOuter
	if err := msgpack.Unmarshal(responseBytes, &outerResponse); err != nil {
		return nil, fmt.Errorf("unmarshalling response to msgpack: %w", err)
	}

	return &outerResponse, nil
}

func (ses *secureEnclaveSigner) createKey(ctx context.Context) error {
	request := CreateKeyRequest{
		SecureEnclaveRequest: SecureEnclaveRequest{
			Challenge:    ses.challenge,
			ServerPubKey: ses.serverPubKeyB64Der,
		},
	}

	requestMsgPack, err := msgpack.Marshal(request)
	if err != nil {
		return fmt.Errorf("marshalling request to msgpack: %w", err)
	}

	cmd, err := allowedcmd.Launchctl(
		ctx,
		"asuser",
		ses.uid,
		"sudo",
		"--preserve-env",
		"-u",
		ses.username,
		ses.pathToLauncherBinary,
		"secure-enclave",
		CreateKeyCmd,
		base64.StdEncoding.EncodeToString(requestMsgPack),
	)

	if err != nil {
		return fmt.Errorf("creating command to create key: %w", err)
	}

	// skip updates since we have full path of binary
	cmd.Env = append(cmd.Environ(), fmt.Sprintf("%s=%s", "LAUNCHER_SKIP_UPDATES", "true"))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("executing launcher binary to create key: %w: %s", err, string(out))
	}

	pubKey, err := echelper.PublicB64DerToEcdsaKey([]byte(lastLine(out)))
	if err != nil {
		return fmt.Errorf("marshalling public key to der: %w", err)
	}

	ses.pubKey = pubKey
	return nil
}

// lastLine returns the last line of the out.
// This is needed because laucher sets up a logger by default.
// The last line of the output is the public key or signature.
func lastLine(out []byte) string {
	outStr := string(out)

	// get last line of outstr
	lastLine := ""
	for _, line := range strings.Split(outStr, "\n") {
		if line != "" {
			lastLine = line
		}
	}

	return lastLine
}