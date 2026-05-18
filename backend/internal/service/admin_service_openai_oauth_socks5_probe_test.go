//go:build unit

package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type probeCreateAccountRepo struct {
	AccountRepository
	created chan *Account
}

func (r *probeCreateAccountRepo) Create(_ context.Context, account *Account) error {
	account.ID = 123
	if r.created != nil {
		cloned := *account
		r.created <- &cloned
	}
	return nil
}

type startupProbeAccountRepo struct {
	AccountRepository
	accounts []Account
	err      error
}

func (r *startupProbeAccountRepo) ListActive(context.Context) ([]Account, error) {
	return r.accounts, r.err
}

func withOpenAIOAuthSocks5ProbeTestConfig(t *testing.T, attempts int, factory func(string) (*http.Client, error)) {
	t.Helper()
	origAttempts := openAIOAuthSocks5ProbeMaxAttempts
	origInterval := openAIOAuthSocks5ProbeRetryInterval
	origFactory := openAIOAuthSocks5ProbeClientFactory
	openAIOAuthSocks5ProbeMaxAttempts = attempts
	openAIOAuthSocks5ProbeRetryInterval = 0
	openAIOAuthSocks5ProbeClientFactory = factory
	t.Cleanup(func() {
		openAIOAuthSocks5ProbeMaxAttempts = origAttempts
		openAIOAuthSocks5ProbeRetryInterval = origInterval
		openAIOAuthSocks5ProbeClientFactory = origFactory
	})
}

func testOpenAIOAuthAccount(protocol string) *Account {
	proxyID := int64(10)
	return &Account{
		ID:       1,
		Name:     "openai-oauth",
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		ProxyID:  &proxyID,
		Proxy: &Proxy{
			ID:       proxyID,
			Protocol: protocol,
			Host:     "proxy.example.com",
			Port:     1080,
			Username: "user",
			Password: "pass",
			Status:   StatusActive,
		},
	}
}

func TestShouldProbeOpenAIOAuthSocks5(t *testing.T) {
	require.True(t, shouldProbeOpenAIOAuthSocks5(testOpenAIOAuthAccount("socks5")))
	require.True(t, shouldProbeOpenAIOAuthSocks5(testOpenAIOAuthAccount("SOCKS5H")))
	require.False(t, shouldProbeOpenAIOAuthSocks5(testOpenAIOAuthAccount("http")))
	require.False(t, shouldProbeOpenAIOAuthSocks5(&Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Proxy: testOpenAIOAuthAccount("socks5").Proxy}))
	require.False(t, shouldProbeOpenAIOAuthSocks5(&Account{Platform: PlatformAnthropic, Type: AccountTypeOAuth, Proxy: testOpenAIOAuthAccount("socks5").Proxy}))
	require.False(t, shouldProbeOpenAIOAuthSocks5(&Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}))
}

func TestProbeOpenAIOAuthSocks5RetriesUntilSuccess(t *testing.T) {
	var calls atomic.Int32
	var gotProxyURL string
	withOpenAIOAuthSocks5ProbeTestConfig(t, 3, func(proxyURL string) (*http.Client, error) {
		gotProxyURL = proxyURL
		return &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			call := calls.Add(1)
			if call < 3 {
				return &http.Response{
					StatusCode: http.StatusBadGateway,
					Body:       io.NopCloser(strings.NewReader("bad gateway")),
				}, nil
			}
			require.Equal(t, openAIOAuthSocks5ProbeTargetURL, req.URL.String())
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("203.0.113.1\n")),
			}, nil
		})}, nil
	})

	probeOpenAIOAuthSocks5(context.Background(), testOpenAIOAuthAccount("socks5"), "unit_test")

	require.Equal(t, int32(3), calls.Load())
	require.Equal(t, "socks5://user:pass@proxy.example.com:1080", gotProxyURL)
}

func TestProbeOpenAIOAuthSocks5StopsAfterMaxAttempts(t *testing.T) {
	var calls atomic.Int32
	withOpenAIOAuthSocks5ProbeTestConfig(t, 3, func(string) (*http.Client, error) {
		return &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			calls.Add(1)
			return nil, errors.New("proxy down")
		})}, nil
	})

	probeOpenAIOAuthSocks5(context.Background(), testOpenAIOAuthAccount("socks5"), "unit_test")

	require.Equal(t, int32(3), calls.Load())
}

func TestCreateAccountTriggersOpenAIOAuthSocks5ProbeWithoutBlocking(t *testing.T) {
	setForcedOpenAIOAuthSocks5ProxyEnv(t)
	called := make(chan string, 1)
	withOpenAIOAuthSocks5ProbeTestConfig(t, 1, func(proxyURL string) (*http.Client, error) {
		called <- proxyURL
		return nil, errors.New("probe failed")
	})
	repo := &probeCreateAccountRepo{created: make(chan *Account, 1)}
	proxyRepo := &forcedProxyRepoStub{}
	svc := &adminServiceImpl{accountRepo: repo, proxyRepo: proxyRepo}

	account, err := svc.CreateAccount(context.Background(), &CreateAccountInput{
		Name:                 "oauth",
		Platform:             PlatformOpenAI,
		Type:                 AccountTypeOAuth,
		Credentials:          map[string]any{"chatgpt_account_id": "acct_123"},
		SkipDefaultGroupBind: true,
	})

	require.NoError(t, err)
	require.NotNil(t, account)
	select {
	case proxyURL := <-called:
		require.Equal(t, "socks5://acct_123:secret@proxy.internal:1080", proxyURL)
	case <-time.After(time.Second):
		t.Fatal("expected probe to be triggered")
	}
}

func TestNewAdminServiceStartupProbeFiltersOpenAIOAuthSocks5Accounts(t *testing.T) {
	called := make(chan string, 1)
	withOpenAIOAuthSocks5ProbeTestConfig(t, 1, func(proxyURL string) (*http.Client, error) {
		called <- proxyURL
		return &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("203.0.113.2"))}, nil
		})}, nil
	})
	repo := &startupProbeAccountRepo{accounts: []Account{
		*testOpenAIOAuthAccount("socks5"),
		{ID: 2, Name: "apikey", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Proxy: testOpenAIOAuthAccount("socks5").Proxy},
		{ID: 3, Name: "http-proxy", Platform: PlatformOpenAI, Type: AccountTypeOAuth, Proxy: testOpenAIOAuthAccount("http").Proxy},
	}}

	_ = NewAdminService(nil, nil, repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	select {
	case proxyURL := <-called:
		require.Equal(t, "socks5://user:pass@proxy.example.com:1080", proxyURL)
	case <-time.After(time.Second):
		t.Fatal("expected startup probe to be triggered")
	}
	select {
	case extra := <-called:
		t.Fatalf("unexpected extra probe: %s", extra)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestNewAdminServiceStartupProbeListActiveFailureDoesNotBlock(t *testing.T) {
	repo := &startupProbeAccountRepo{err: errors.New("db unavailable")}

	svc := NewAdminService(nil, nil, repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	require.NotNil(t, svc)
}
