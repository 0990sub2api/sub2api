package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/httpclient"
)

const openAIOAuthSocks5ProbeTargetURL = "https://api64.ipify.org"

var (
	openAIOAuthSocks5ProbeMaxAttempts   = 20
	openAIOAuthSocks5ProbeRetryInterval = 5 * time.Second
	openAIOAuthSocks5ProbeClientFactory = defaultOpenAIOAuthSocks5ProbeClient
)

func defaultOpenAIOAuthSocks5ProbeClient(proxyURL string) (*http.Client, error) {
	return httpclient.GetClient(httpclient.Options{
		ProxyURL:              proxyURL,
		Timeout:               10 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		MaxIdleConnsPerHost:   2,
		MaxConnsPerHost:       2,
		ValidateResolvedIP:    false,
		AllowPrivateHosts:     false,
		InsecureSkipVerify:    false,
		MaxIdleConns:          10,
	})
}

func shouldProbeOpenAIOAuthSocks5(account *Account) bool {
	if account == nil || !account.IsOpenAIOAuth() || account.Proxy == nil {
		return false
	}
	protocol := strings.ToLower(strings.TrimSpace(account.Proxy.Protocol))
	return protocol == "socks5" || protocol == "socks5h"
}

func (s *adminServiceImpl) probeOpenAIOAuthSocks5OnStartup() {
	if s == nil || s.accountRepo == nil {
		return
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("openai_oauth_socks5_probe.startup_panic", "recover", r)
			}
		}()

		accounts, err := s.accountRepo.ListActive(context.Background())
		if err != nil {
			slog.Warn("openai_oauth_socks5_probe.startup_list_failed", "error", err)
			return
		}
		for i := range accounts {
			account := accounts[i]
			s.probeOpenAIOAuthSocks5Async(&account, "service_startup")
		}
	}()
}

func (s *adminServiceImpl) probeOpenAIOAuthSocks5Async(account *Account, reason string) {
	if !shouldProbeOpenAIOAuthSocks5(account) {
		return
	}
	probeAccount := cloneOpenAIOAuthSocks5ProbeAccount(account)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("openai_oauth_socks5_probe.panic",
					"account_id", probeAccount.ID,
					"account_name", probeAccount.Name,
					"reason", reason,
					"recover", r,
				)
			}
		}()
		probeOpenAIOAuthSocks5(context.Background(), probeAccount, reason)
	}()
}

func cloneOpenAIOAuthSocks5ProbeAccount(account *Account) *Account {
	if account == nil {
		return nil
	}
	cloned := *account
	if account.Proxy != nil {
		proxy := *account.Proxy
		cloned.Proxy = &proxy
	}
	return &cloned
}

func probeOpenAIOAuthSocks5(ctx context.Context, account *Account, reason string) {
	if !shouldProbeOpenAIOAuthSocks5(account) {
		return
	}
	attempts := openAIOAuthSocks5ProbeMaxAttempts
	if attempts <= 0 {
		attempts = 1
	}
	interval := openAIOAuthSocks5ProbeRetryInterval
	if interval < 0 {
		interval = 0
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		start := time.Now()
		err := probeOpenAIOAuthSocks5Once(ctx, account)
		latencyMs := time.Since(start).Milliseconds()
		if err == nil {
			slog.Info("openai_oauth_socks5_probe.succeeded",
				openAIOAuthSocks5ProbeLogFields(account, reason, attempt, attempts, latencyMs, nil)...,
			)
			return
		}
		lastErr = err
		slog.Warn("openai_oauth_socks5_probe.failed",
			openAIOAuthSocks5ProbeLogFields(account, reason, attempt, attempts, latencyMs, err)...,
		)

		if attempt == attempts {
			break
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			slog.Warn("openai_oauth_socks5_probe.cancelled",
				openAIOAuthSocks5ProbeLogFields(account, reason, attempt, attempts, latencyMs, ctx.Err())...,
			)
			return
		case <-timer.C:
		}
	}

	slog.Warn("openai_oauth_socks5_probe.exhausted",
		openAIOAuthSocks5ProbeLogFields(account, reason, attempts, attempts, 0, lastErr)...,
	)
}

func probeOpenAIOAuthSocks5Once(ctx context.Context, account *Account) error {
	if !shouldProbeOpenAIOAuthSocks5(account) {
		return nil
	}
	client, err := openAIOAuthSocks5ProbeClientFactory(account.Proxy.URL())
	if err != nil {
		return fmt.Errorf("create proxy client: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, openAIOAuthSocks5ProbeTargetURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if strings.TrimSpace(string(body)) == "" {
		return fmt.Errorf("empty response")
	}
	return nil
}

func openAIOAuthSocks5ProbeLogFields(account *Account, reason string, attempt, maxAttempts int, latencyMs int64, err error) []any {
	fields := []any{
		"account_id", account.ID,
		"account_name", account.Name,
		"attempt", attempt,
		"max_attempts", maxAttempts,
		"reason", reason,
		"target_url", openAIOAuthSocks5ProbeTargetURL,
		"latency_ms", latencyMs,
	}
	if account.Proxy != nil {
		fields = append(fields,
			"proxy_id", account.Proxy.ID,
			"proxy_host", account.Proxy.Host,
			"proxy_port", account.Proxy.Port,
		)
	}
	if err != nil {
		fields = append(fields, "error", err)
	}
	return fields
}
