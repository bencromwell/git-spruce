package cmd //nolint:testpackage // loadConfig is unexported and this tests it directly

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetViper(t *testing.T) {
	t.Helper()

	// viper is process-global; every test starts from a clean state
	// and re-registers the command defaults
	viper.Reset()
	t.Cleanup(viper.Reset)
}

func rootCommandWithConfigFlag(t *testing.T, args []string) *cobra.Command {
	t.Helper()

	rootCmd := NewRootCommand("test", "test")
	rootCmd.SetArgs(args)
	require.NoError(t, rootCmd.ParseFlags(args))

	return rootCmd
}

func TestLoadConfigExplicitFile(t *testing.T) {
	resetViper(t)

	configPath := filepath.Join(t.TempDir(), "custom-config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("merge_base: custom-base\n"), 0o644))

	rootCmd := rootCommandWithConfigFlag(t, []string{"--config", configPath})
	require.NoError(t, loadConfig(rootCmd))
	assert.Equal(t, configPath, viper.ConfigFileUsed())
	assert.Equal(t, "custom-base", viper.GetString("merge_base"))

	// defaults from the command still apply for unset keys
	assert.Equal(t, "origin", viper.GetString("origin"))
}

func TestLoadConfigMissingExplicitFileFails(t *testing.T) {
	resetViper(t)

	configPath := filepath.Join(t.TempDir(), "does-not-exist.yaml")

	rootCmd := rootCommandWithConfigFlag(t, []string{"--config", configPath})
	require.Error(t, loadConfig(rootCmd))
}

func TestLoadConfigInvalidYamlFails(t *testing.T) {
	resetViper(t)

	configPath := filepath.Join(t.TempDir(), "broken.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("merge_base: [unclosed\n"), 0o644))

	rootCmd := rootCommandWithConfigFlag(t, []string{"--config", configPath})
	require.Error(t, loadConfig(rootCmd))
}
