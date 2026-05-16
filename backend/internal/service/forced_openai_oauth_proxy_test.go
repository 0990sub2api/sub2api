//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type forcedProxyRepoStub struct {
	created []Proxy
	listed  []Proxy
}

func (s *forcedProxyRepoStub) Create(_ context.Context, proxy *Proxy) error {
	s.created = append(s.created, *proxy)
	proxy.ID = int64(len(s.created))
	return nil
}

func (s *forcedProxyRepoStub) ListWithFilters(_ context.Context, _ pagination.PaginationParams, protocol, _, _ string) ([]Proxy, *pagination.PaginationResult, error) {
	out := make([]Proxy, 0, len(s.listed))
	for i := range s.listed {
		if protocol == "" || s.listed[i].Protocol == protocol {
			out = append(out, s.listed[i])
		}
	}
	return out, &pagination.PaginationResult{Total: int64(len(out))}, nil
}

func (s *forcedProxyRepoStub) Update(_ context.Context, proxy *Proxy) error { return nil }
func (s *forcedProxyRepoStub) GetByID(context.Context, int64) (*Proxy, error) {
	panic("unexpected GetByID call")
}
func (s *forcedProxyRepoStub) ListByIDs(context.Context, []int64) ([]Proxy, error) {
	panic("unexpected ListByIDs call")
}
func (s *forcedProxyRepoStub) Delete(context.Context, int64) error { panic("unexpected Delete call") }
func (s *forcedProxyRepoStub) List(context.Context, pagination.PaginationParams) ([]Proxy, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}
func (s *forcedProxyRepoStub) ListWithFiltersAndAccountCount(context.Context, pagination.PaginationParams, string, string, string) ([]ProxyWithAccountCount, *pagination.PaginationResult, error) {
	panic("unexpected ListWithFiltersAndAccountCount call")
}
func (s *forcedProxyRepoStub) ListActive(context.Context) ([]Proxy, error) {
	panic("unexpected ListActive call")
}
func (s *forcedProxyRepoStub) ListActiveWithAccountCount(context.Context) ([]ProxyWithAccountCount, error) {
	panic("unexpected ListActiveWithAccountCount call")
}
func (s *forcedProxyRepoStub) ExistsByHostPortAuth(context.Context, string, int, string, string) (bool, error) {
	panic("unexpected ExistsByHostPortAuth call")
}
func (s *forcedProxyRepoStub) CountAccountsByProxyID(context.Context, int64) (int64, error) {
	panic("unexpected CountAccountsByProxyID call")
}
func (s *forcedProxyRepoStub) ListAccountSummariesByProxyID(context.Context, int64) ([]ProxyAccountSummary, error) {
	panic("unexpected ListAccountSummariesByProxyID call")
}

func setForcedOpenAIOAuthSocks5ProxyEnv(t *testing.T) {
	t.Setenv("FORCED_OPENAI_OAUTH_SOCKS5_HOST", "proxy.internal")
	t.Setenv("FORCED_OPENAI_OAUTH_SOCKS5_PORT", "1080")
	t.Setenv("FORCED_OPENAI_OAUTH_SOCKS5_PASSWORD", "secret")
}

func TestForcedOpenAIOAuthSocks5ProxyUsesChatGPTAccountIDAsUsername(t *testing.T) {
	setForcedOpenAIOAuthSocks5ProxyEnv(t)
	repo := &forcedProxyRepoStub{}
	svc := &adminServiceImpl{proxyRepo: repo}
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"chatgpt_account_id": "oauth-account-123",
		},
	}

	err := svc.bindForcedOpenAIOAuthSocks5ProxyIfNeeded(context.Background(), account, false)

	require.NoError(t, err)
	require.NotNil(t, account.ProxyID)
	require.Len(t, repo.created, 1)
	require.Equal(t, "socks5", repo.created[0].Protocol)
	require.Equal(t, "oauth-account-123", repo.created[0].Username)
	require.Equal(t, "secret", repo.created[0].Password)
	require.Equal(t, "proxy.internal", repo.created[0].Host)
	require.Equal(t, 1080, repo.created[0].Port)
}

func TestForcedOpenAIOAuthSocks5ProxyRejectsMissingChatGPTAccountID(t *testing.T) {
	setForcedOpenAIOAuthSocks5ProxyEnv(t)
	repo := &forcedProxyRepoStub{}
	svc := &adminServiceImpl{proxyRepo: repo}
	account := &Account{
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{},
	}

	err := svc.bindForcedOpenAIOAuthSocks5ProxyIfNeeded(context.Background(), account, false)

	require.Error(t, err)
	require.Contains(t, err.Error(), "chatgpt_account_id")
	require.Empty(t, repo.created)
}

func TestForcedOpenAIOAuthSocks5ProxyIgnoresNonOpenAIOAuthAccounts(t *testing.T) {
	setForcedOpenAIOAuthSocks5ProxyEnv(t)
	repo := &forcedProxyRepoStub{}
	svc := &adminServiceImpl{proxyRepo: repo}
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"chatgpt_account_id": "oauth-account-123",
		},
	}

	err := svc.bindForcedOpenAIOAuthSocks5ProxyIfNeeded(context.Background(), account, false)

	require.NoError(t, err)
	require.Nil(t, account.ProxyID)
	require.Empty(t, repo.created)
}

func TestShouldHideProxyInfoForForcedOpenAIOAuthAccount(t *testing.T) {
	setForcedOpenAIOAuthSocks5ProxyEnv(t)
	require.True(t, ShouldHideProxyInfoForAccount(&Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}))
	require.False(t, ShouldHideProxyInfoForAccount(&Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}))
}
