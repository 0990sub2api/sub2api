package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const forcedOpenAIOAuthSocks5ProxyProtocol = "socks5"

func IsForcedOpenAIOAuthSocks5ProxyEnabled() bool {
	return config.IsForcedOpenAIOAuthSocks5ProxyEnabledFromEnv()
}

func ShouldHideProxyInfoForAccount(account *Account) bool {
	return account != nil && account.IsOpenAIOAuth() && IsForcedOpenAIOAuthSocks5ProxyEnabled()
}

func (s *adminServiceImpl) bindForcedOpenAIOAuthSocks5ProxyIfNeeded(ctx context.Context, account *Account, persist bool) error {
	proxyID, proxy, err := s.resolveForcedOpenAIOAuthSocks5Proxy(ctx, account)
	if err != nil || proxyID == nil {
		return err
	}
	account.ProxyID = proxyID
	account.Proxy = proxy
	if !persist {
		return nil
	}
	return s.accountRepo.Update(ctx, account)
}

func (s *adminServiceImpl) resolveForcedOpenAIOAuthSocks5Proxy(ctx context.Context, account *Account) (*int64, *Proxy, error) {
	if account == nil || !account.IsOpenAIOAuth() {
		return nil, nil, nil
	}
	cfg, err := config.ForcedOpenAIOAuthSocks5ProxyFromEnv()
	if err != nil {
		return nil, nil, err
	}
	if !cfg.Enabled {
		return nil, nil, nil
	}
	if err := validateForcedOpenAIOAuthSocks5ProxyAccount(cfg, account); err != nil {
		return nil, nil, err
	}
	accountUniqueID := strings.TrimSpace(account.GetChatGPTAccountID())

	proxy, err := s.findForcedOpenAIOAuthSocks5Proxy(ctx, cfg, accountUniqueID)
	if err != nil {
		return nil, nil, err
	}
	if proxy == nil {
		proxy = &Proxy{
			Name:     forcedOpenAIOAuthSocks5ProxyName(accountUniqueID),
			Protocol: forcedOpenAIOAuthSocks5ProxyProtocol,
			Host:     strings.TrimSpace(cfg.Host),
			Port:     cfg.Port,
			Username: accountUniqueID,
			Password: strings.TrimSpace(cfg.Password),
			Status:   StatusActive,
		}
		if err := s.proxyRepo.Create(ctx, proxy); err != nil {
			return nil, nil, err
		}
		return &proxy.ID, proxy, nil
	}

	changed := false
	if proxy.Name != forcedOpenAIOAuthSocks5ProxyName(accountUniqueID) {
		proxy.Name = forcedOpenAIOAuthSocks5ProxyName(accountUniqueID)
		changed = true
	}
	if proxy.Status != StatusActive {
		proxy.Status = StatusActive
		changed = true
	}
	if changed {
		if err := s.proxyRepo.Update(ctx, proxy); err != nil {
			return nil, nil, err
		}
	}
	return &proxy.ID, proxy, nil
}

func validateForcedOpenAIOAuthSocks5ProxyAccount(cfg config.ForcedOpenAIOAuthSocks5ProxyConfig, account *Account) error {
	if account == nil || !account.IsOpenAIOAuth() || !cfg.Enabled {
		return nil
	}
	if strings.TrimSpace(account.GetChatGPTAccountID()) == "" {
		return infraerrors.BadRequest(
			"OPENAI_OAUTH_ACCOUNT_ID_REQUIRED",
			"credentials.chatgpt_account_id is required when forced OpenAI OAuth SOCKS5 proxy is enabled",
		)
	}
	if strings.TrimSpace(cfg.Host) == "" || cfg.Port <= 0 || cfg.Port > 65535 || strings.TrimSpace(cfg.Password) == "" {
		return infraerrors.BadRequest(
			"FORCED_OPENAI_OAUTH_SOCKS5_PROXY_INVALID",
			"forced OpenAI OAuth SOCKS5 proxy environment variables are incomplete",
		)
	}
	return nil
}

func validateBulkForcedOpenAIOAuthSocks5ProxyAccounts(input *BulkUpdateAccountsInput, accounts []*Account) error {
	cfg, err := config.ForcedOpenAIOAuthSocks5ProxyFromEnv()
	if err != nil {
		return err
	}
	if !cfg.Enabled {
		return nil
	}
	for _, account := range accounts {
		if account == nil || !account.IsOpenAIOAuth() {
			continue
		}
		projected := *account
		projected.Credentials = mergeCredentialMaps(account.Credentials, input.Credentials)
		if err := validateForcedOpenAIOAuthSocks5ProxyAccount(cfg, &projected); err != nil {
			return err
		}
	}
	return nil
}

func (s *adminServiceImpl) findForcedOpenAIOAuthSocks5Proxy(ctx context.Context, cfg config.ForcedOpenAIOAuthSocks5ProxyConfig, username string) (*Proxy, error) {
	host := strings.TrimSpace(cfg.Host)
	password := strings.TrimSpace(cfg.Password)
	const pageSize = 500
	for page := 1; ; page++ {
		proxies, result, err := s.proxyRepo.ListWithFilters(ctx, pagination.PaginationParams{
			Page:      page,
			PageSize:  pageSize,
			SortBy:    "id",
			SortOrder: "desc",
		}, forcedOpenAIOAuthSocks5ProxyProtocol, "", "")
		if err != nil {
			return nil, err
		}
		for i := range proxies {
			p := proxies[i]
			if p.Protocol == forcedOpenAIOAuthSocks5ProxyProtocol &&
				p.Host == host &&
				p.Port == cfg.Port &&
				p.Username == username &&
				p.Password == password {
				return &p, nil
			}
		}
		if result == nil || int64(page*pageSize) >= result.Total || len(proxies) == 0 {
			return nil, nil
		}
	}
}

func forcedOpenAIOAuthSocks5ProxyName(accountUniqueID string) string {
	name := fmt.Sprintf("forced-openai-oauth-%s", accountUniqueID)
	if len(name) > 100 {
		return name[:100]
	}
	return name
}

func mergeCredentialMaps(base, overlay map[string]any) map[string]any {
	if len(overlay) == 0 {
		return base
	}
	merged := make(map[string]any, len(base)+len(overlay))
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range overlay {
		merged[k] = v
	}
	return merged
}
