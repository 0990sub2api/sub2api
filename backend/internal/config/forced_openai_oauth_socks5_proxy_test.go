package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestForcedOpenAIOAuthSocks5ProxyFromEnv(t *testing.T) {
	t.Setenv("FORCED_OPENAI_OAUTH_SOCKS5_HOST", "proxy.internal")
	t.Setenv("FORCED_OPENAI_OAUTH_SOCKS5_PORT", "1080")
	t.Setenv("FORCED_OPENAI_OAUTH_SOCKS5_PASSWORD", "secret")

	cfg, err := ForcedOpenAIOAuthSocks5ProxyFromEnv()

	require.NoError(t, err)
	require.Equal(t, "proxy.internal", cfg.Host)
	require.Equal(t, 1080, cfg.Port)
	require.Equal(t, "secret", cfg.Password)
}

func TestForcedOpenAIOAuthSocks5ProxyFromEnvRejectsInvalidPort(t *testing.T) {
	t.Setenv("FORCED_OPENAI_OAUTH_SOCKS5_PORT", "bad")

	_, err := ForcedOpenAIOAuthSocks5ProxyFromEnv()

	require.Error(t, err)
	require.Contains(t, err.Error(), "FORCED_OPENAI_OAUTH_SOCKS5_PORT")
}
