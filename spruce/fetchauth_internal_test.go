package spruce

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
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

// writeDefaultTestKey drops a valid ed25519 private key at
// $HOME/.ssh/id_ed25519, the first file the fallback tries.
func writeDefaultTestKey(t *testing.T, home string) {
	t.Helper()

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	der, err := x509.MarshalPKCS8PrivateKey(priv)
	require.NoError(t, err)

	keyDir := filepath.Join(home, ".ssh")
	require.NoError(t, os.MkdirAll(keyDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(keyDir, "id_ed25519"),
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600))
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

func TestFetchAuthNilWithoutAgentOrKeys(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	t.Setenv("HOME", t.TempDir())

	gs := sshRemoteSpruce(t, "ssh://git@github.com/bencromwell/git-spruce.git")
	assert.Nil(t, gs.fetchAuth())
}
