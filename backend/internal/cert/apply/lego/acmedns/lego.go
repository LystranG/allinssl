// Package acmedns implements a DNS provider for solving ACME DNS-01 challenges
// using the ACME-DNS service (https://github.com/joohoi/acme-dns).
// Account credentials are persisted to the caller-supplied storage callback
// so they survive across certificate renewal runs.
package acmedns

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-acme/lego/v4/challenge"
	"github.com/go-acme/lego/v4/challenge/dns01"
)

const DefaultAPIBase = "https://auth.acme-dns.io"

// Account holds the per-domain credentials issued by the ACME-DNS server.
type Account struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	Subdomain  string `json:"subdomain"`
	FullDomain string `json:"fulldomain"`
}

// Config holds the configuration for the DNSProvider.
type Config struct {
	// APIBase is the base URL of the ACME-DNS server.
	// Defaults to https://auth.acme-dns.io.
	APIBase string

	// Accounts is a map of bare domain name → Account.
	// It is populated automatically when a domain is first seen.
	Accounts map[string]Account

	// PropagationTimeout and PollingInterval control how long the lego
	// client waits for DNS propagation after Present returns.
	PropagationTimeout time.Duration
	PollingInterval    time.Duration

	// UpdateAccounts, when non-nil, is called whenever Accounts is
	// modified so the caller can persist the updated map.
	UpdateAccounts func(accounts map[string]Account) error
}

// DNSProvider implements challenge.ProviderTimeout for ACME-DNS.
type DNSProvider struct {
	config *Config
	client *http.Client
}

var _ challenge.ProviderTimeout = (*DNSProvider)(nil)

// NewDefaultConfig returns a Config pointing at the public ACME-DNS server.
func NewDefaultConfig() *Config {
	return &Config{
		APIBase:            DefaultAPIBase,
		Accounts:           make(map[string]Account),
		PropagationTimeout: dns01.DefaultPropagationTimeout,
		PollingInterval:    dns01.DefaultPollingInterval,
	}
}

// NewDNSProviderConfig creates a DNSProvider from cfg.
func NewDNSProviderConfig(cfg *Config) (*DNSProvider, error) {
	if cfg == nil {
		return nil, fmt.Errorf("acme-dns: 配置不能为空")
	}
	if cfg.APIBase == "" {
		cfg.APIBase = DefaultAPIBase
	}
	if cfg.Accounts == nil {
		cfg.Accounts = make(map[string]Account)
	}
	return &DNSProvider{
		config: cfg,
		client: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// Timeout implements challenge.ProviderTimeout.
func (d *DNSProvider) Timeout() (time.Duration, time.Duration) {
	return d.config.PropagationTimeout, d.config.PollingInterval
}

// Present creates/updates the TXT record for the DNS-01 challenge.
//
// If the domain has no stored account it is registered with the ACME-DNS
// server and an error is returned asking the user to create a CNAME record.
// On the next invocation (after the CNAME is in place) the stored account
// is used to update the TXT record and the challenge proceeds normally.
func (d *DNSProvider) Present(domain, _, keyAuth string) error {
	info := dns01.GetChallengeInfo(domain, keyAuth)

	account, ok := d.config.Accounts[domain]
	if !ok {
		// First time for this domain – register a new account.
		newAccount, err := d.register()
		if err != nil {
			return fmt.Errorf("acme-dns: 注册账号失败: %w", err)
		}
		d.config.Accounts[domain] = newAccount
		account = newAccount

		// Persist accounts so the next run does not need to re-register.
		if d.config.UpdateAccounts != nil {
			if saveErr := d.config.UpdateAccounts(d.config.Accounts); saveErr != nil {
				// Saving failed. Return an error that still gives the user the
				// CNAME instructions and the raw account credentials so they can
				// manually add them on retry.
				return fmt.Errorf(
					"acme-dns: 域名 %q 已注册新的 ACME-DNS 账号，但账号信息保存失败: %v\n"+
						"账号信息（请妥善保存）: username=%s, subdomain=%s, fulldomain=%s\n"+
						"请在 DNS 中为该域名添加以下 CNAME 记录，完成后重新运行：\n"+
						"  %s  CNAME  %s.",
					domain, saveErr,
					account.Username, account.Subdomain, account.FullDomain,
					info.EffectiveFQDN, account.FullDomain,
				)
			}
		}

		// Halt this attempt and tell the user what CNAME to add.
		return fmt.Errorf(
			"acme-dns: 域名 %q 已注册新的 ACME-DNS 账号。\n"+
				"请在 DNS 中为该域名添加以下 CNAME 记录，完成后重新运行：\n"+
				"  %s  CNAME  %s.",
			domain, info.EffectiveFQDN, account.FullDomain,
		)
	}

	return d.updateTXT(account, info.Value)
}

// CleanUp implements challenge.Provider.
// ACME-DNS does not support record deletion so this is a no-op.
func (d *DNSProvider) CleanUp(_, _, _ string) error {
	return nil
}

// register calls POST /register on the ACME-DNS server and returns the new account.
func (d *DNSProvider) register() (Account, error) {
	apiURL := d.config.APIBase + "/register"
	req, err := http.NewRequest(http.MethodPost, apiURL, bytes.NewBufferString("{}"))
	if err != nil {
		return Account{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		return Account{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return Account{}, fmt.Errorf("注册失败，HTTP 状态码: %d", resp.StatusCode)
	}

	var account Account
	if err := json.NewDecoder(resp.Body).Decode(&account); err != nil {
		return Account{}, fmt.Errorf("解析注册响应失败: %w", err)
	}
	return account, nil
}

// updateTXT calls POST /update on the ACME-DNS server with the new TXT value.
func (d *DNSProvider) updateTXT(account Account, value string) error {
	apiURL := d.config.APIBase + "/update"
	body, err := json.Marshal(map[string]string{
		"subdomain": account.Subdomain,
		"txt":       value,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-User", account.Username)
	req.Header.Set("X-Api-Key", account.Password)

	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("更新 TXT 记录失败，HTTP 状态码: %d", resp.StatusCode)
	}
	return nil
}
