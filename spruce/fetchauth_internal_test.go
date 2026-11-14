package spruce

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sshRemoteSpruce(t *testing.T, url string) *GitSpruce {
	t.Helper()

	repo, err := git.PlainInit(t.TempDir(), false)
	require.NoError(t, err)

	_, err = repo.CreateRemote(&config.RemoteConfig{Name: "origin", URLs: []string{url}})
	require.NoError(t, err)

	return &GitSpruce{Origin: "origin", Repo: repo}
}

// writeTestKey drops a private key at $HOME/.ssh/<name>.
func writeTestKey(t *testing.T, home, name, blockType string, der []byte) {
	t.Helper()

	keyDir := filepath.Join(home, ".ssh")
	require.NoError(t, os.MkdirAll(keyDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(keyDir, name),
		pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der}), 0o600))
}

// writeDefaultTestKey drops a valid ed25519 private key at
// $HOME/.ssh/id_ed25519.
func writeDefaultTestKey(t *testing.T, home string) {
	t.Helper()

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	der, err := x509.MarshalPKCS8PrivateKey(priv)
	require.NoError(t, err)

	writeTestKey(t, home, "id_ed25519", "PRIVATE KEY", der)
}

func TestFetchAuthSkipsNonSSHRemote(t *testing.T) {
	gs := sshRemoteSpruce(t, "https://github.com/bencromwell/git-spruce.git")
	assert.Nil(t, gs.fetchAuth())
}

func TestFetchAuthFallsBackToDefaultKeyWithoutAgent(t *testing.T) {
	// an empty SSH_AUTH_SOCK makes the agent unreachable, as in cron,
	// systemd units, and CI runners
	t.Setenv("SSH_AUTH_SOCK", "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeDefaultTestKey(t, home)

	gs := sshRemoteSpruce(t, "git@github.com:bencromwell/git-spruce.git")
	assert.NotNil(t, gs.fetchAuth())
}

func TestFetchAuthOffersEveryDefaultKeyWithoutAgent(t *testing.T) {
	// the first key file (id_rsa) may parse yet be rejected by the remote,
	// so the fallback must offer every readable key, not stop at the first
	t.Setenv("SSH_AUTH_SOCK", "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeDefaultTestKey(t, home)

	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	writeTestKey(t, home, "id_rsa", "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(rsaKey))

	gs := sshRemoteSpruce(t, "git@github.com:bencromwell/git-spruce.git")
	auth := gs.fetchAuth()
	require.NotNil(t, auth)

	callback, ok := auth.(*gitssh.PublicKeysCallback)
	require.True(t, ok, "fallback should offer all keys via a callback, got %T", auth)
	signers, err := callback.Callback()
	require.NoError(t, err)
	assert.Len(t, signers, 2)
}

func TestFetchAuthNilWithoutAgentOrKeys(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	t.Setenv("HOME", t.TempDir())

	gs := sshRemoteSpruce(t, "ssh://git@github.com/bencromwell/git-spruce.git")
	assert.Nil(t, gs.fetchAuth())
}

func TestFetchAuthUsesScpLikeURLUser(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeDefaultTestKey(t, home)

	// scp-like syntax with a non-git user is still an ssh remote
	gs := sshRemoteSpruce(t, "deploy@gitlab.example.com:org/repo.git")
	auth := gs.fetchAuth()
	require.NotNil(t, auth)

	callback, ok := auth.(*gitssh.PublicKeysCallback)
	require.True(t, ok, "got %T", auth)
	assert.Equal(t, "deploy", callback.User)
}

func TestFetchAuthUsesSSHURLUser(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeDefaultTestKey(t, home)

	gs := sshRemoteSpruce(t, "ssh://deploy@github.com/bencromwell/git-spruce.git")
	auth := gs.fetchAuth()
	require.NotNil(t, auth)

	callback, ok := auth.(*gitssh.PublicKeysCallback)
	require.True(t, ok, "got %T", auth)
	assert.Equal(t, "deploy", callback.User)
}

func TestFetchAuthDefaultsToGitUser(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeDefaultTestKey(t, home)

	gs := sshRemoteSpruce(t, "ssh://github.com/bencromwell/git-spruce.git")
	auth := gs.fetchAuth()
	require.NotNil(t, auth)

	callback, ok := auth.(*gitssh.PublicKeysCallback)
	require.True(t, ok, "got %T", auth)
	assert.Equal(t, "git", callback.User)
}
