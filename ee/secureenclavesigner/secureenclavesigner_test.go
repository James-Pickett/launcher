//go:build darwin
// +build darwin

package secureenclavesigner

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"testing"
	"time"

	"github.com/kolide/kit/ulid"
	"github.com/kolide/krypto/pkg/challenge"
	"github.com/kolide/krypto/pkg/echelper"
	"github.com/stretchr/testify/require"
)

const (
	testWrappedEnvVarKey = "SECURE_ENCLAVE_TEST_WRAPPED"
	macOsAppResourceDir  = "./test_app_resources"
)

func WithBinaryPath(p string) Opt {
	return func(ses *secureEnclaveSigner) {
		ses.pathToLauncherBinary = p
	}
}

// #nosec G306 -- Need readable files
func TestSecureEnclaveSigner(t *testing.T) {
	t.Parallel()

	if os.Getenv("CI") != "" {
		t.Skipf("\nskipping because %s env var was not empty, this is being run in a CI environment without access to secure enclave", testWrappedEnvVarKey)
	}

	// set up app bundle
	rootDir := t.TempDir()
	appRoot := filepath.Join(rootDir, "launcher_test.app")

	// make required dirs krypto_test.app/Contents/MacOS and add files
	require.NoError(t, os.MkdirAll(filepath.Join(appRoot, "Contents", "MacOS"), 0777))
	copyFile(t, filepath.Join(macOsAppResourceDir, "Info.plist"), filepath.Join(appRoot, "Contents", "Info.plist"))
	copyFile(t, filepath.Join(macOsAppResourceDir, "embedded.provisionprofile"), filepath.Join(appRoot, "Contents", "embedded.provisionprofile"))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	serverPrivKey, err := echelper.GenerateEcdsaKey()
	require.NoError(t, err)

	serverPubKeyDer, err := echelper.PublicEcdsaToB64Der(serverPrivKey.Public().(*ecdsa.PublicKey))
	require.NoError(t, err)

	// build the executable
	executablePath := filepath.Join(appRoot, "Contents", "MacOS", "launcher_test")
	out, err := exec.CommandContext( //nolint:forbidigo // Only used in test, don't want as standard allowedcmd
		ctx,
		"go",
		"build",
		"-ldflags",
		fmt.Sprintf("-X github.com/kolide/launcher/ee/secureenclavesigner.TestServerPubKey=%s", string(serverPubKeyDer)),
		"-tags",
		"secure_enclave_test",
		"-o",
		executablePath,
		"../../cmd/launcher",
	).CombinedOutput()

	require.NoError(t, ctx.Err())
	require.NoError(t, err, string(out))

	// sign app bundle
	signApp(t, appRoot)

	usr, err := user.Current()
	require.NoError(t, err)

	someData := []byte(ulid.New())
	challenge, _, err := challenge.Generate(serverPrivKey, someData, someData, someData)
	require.NoError(t, err)

	// create brand new signer without existing key
	// ask for public first to trigger key generation
	ses, err := New(usr.Uid, serverPubKeyDer, challenge, WithBinaryPath(executablePath))
	require.NoError(t, err)

	pubKey := ses.Public()
	require.NotNil(t, pubKey)

	dataToSign := ulid.New()

	outerResponse, err := ses.Sign(ulid.New(), []byte(dataToSign))
	require.NoError(t, err)

	require.NoError(t, echelper.VerifySignature(pubKey.(*ecdsa.PublicKey), outerResponse.Msg, outerResponse.Sig))

	// create brand new signer without existing key
	// ask to sign first to trigger key generation
	ses, err = New(usr.Uid, serverPubKeyDer, challenge, WithBinaryPath(executablePath))
	require.NoError(t, err)

	outerResponse, err = ses.Sign(ulid.New(), []byte(dataToSign))
	require.NoError(t, err)

	require.NoError(t, echelper.VerifySignature(ses.Public().(*ecdsa.PublicKey), outerResponse.Msg, outerResponse.Sig))

	// create signer with existing key
	ses, err = New(usr.Uid, serverPubKeyDer, challenge, WithBinaryPath(executablePath), WithExistingKey(pubKey.(*ecdsa.PublicKey)))
	require.NoError(t, err)

	outerResponse, err = ses.Sign(ulid.New(), []byte(dataToSign))
	require.NoError(t, err)

	require.NoError(t, echelper.VerifySignature(pubKey.(*ecdsa.PublicKey), outerResponse.Msg, outerResponse.Sig))

	pubKey = ses.Public()
	require.NotNil(t, pubKey)
}

// #nosec G306 -- Need readable files
func copyFile(t *testing.T, source, destination string) {
	bytes, err := os.ReadFile(source)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(destination, bytes, 0700))
}

// #nosec G204 -- This triggers due to using env var in cmd, making exception for test
func signApp(t *testing.T, appRootDir string) {
	codeSignId := os.Getenv("MACOS_CODESIGN_IDENTITY")
	require.NotEmpty(t, codeSignId, "need MACOS_CODESIGN_IDENTITY env var to sign app, such as [Mac Developer: Jane Doe (ABCD123456)]")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext( //nolint:forbidigo // Only used in test, don't want as standard allowcmd
		ctx,
		"codesign",
		"--deep",
		"--force",
		"--options", "runtime",
		"--entitlements", filepath.Join(macOsAppResourceDir, "entitlements"),
		"--sign", codeSignId,
		"--timestamp",
		appRootDir,
	)

	out, err := cmd.CombinedOutput()
	require.NoError(t, ctx.Err())
	require.NoError(t, err, string(out))
}