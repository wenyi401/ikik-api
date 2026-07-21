package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	infraerrors "ikik-api/internal/pkg/errors"
)

const AccountListProxyUnassigned int64 = -1

func (s *AccountService) CreateOwnedProxy(ctx context.Context, ownerUserID int64, req CreateProxyRequest) (*Proxy, error) {
	if ownerUserID <= 0 {
		return nil, ErrUserNotFound
	}
	repo, err := s.userPrivateProxyRepo()
	if err != nil {
		return nil, err
	}
	count, err := repo.CountByOwnerUserID(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	if count >= UserPrivateProxyLimit {
		return nil, ErrUserPrivateProxyLimitExceeded.WithMetadata(map[string]string{
			"limit": fmt.Sprintf("%d", UserPrivateProxyLimit),
		})
	}
	proxy, err := normalizeUserPrivateProxyCreate(req)
	if err != nil {
		return nil, err
	}
	proxy.OwnerUserID = &ownerUserID
	if err := repo.Create(ctx, proxy); err != nil {
		return nil, fmt.Errorf("create user private proxy: %w", err)
	}
	return proxy, nil
}

func (s *AccountService) DeleteOwnedProxy(ctx context.Context, ownerUserID, proxyID int64) error {
	if ownerUserID <= 0 {
		return ErrUserNotFound
	}
	repo, err := s.userPrivateProxyRepo()
	if err != nil {
		return err
	}
	if _, err := repo.GetOwnedByID(ctx, ownerUserID, proxyID); err != nil {
		return err
	}
	count, err := repo.CountOwnedAccountsByProxyID(ctx, ownerUserID, proxyID)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrProxyInUse
	}
	if err := repo.Delete(ctx, proxyID); err != nil {
		return fmt.Errorf("delete user private proxy: %w", err)
	}
	return nil
}

var (
	ErrUserPrivateProxyInvalid = infraerrors.BadRequest("USER_PRIVATE_PROXY_INVALID", "invalid private proxy configuration")
)
var (
	ErrUserPrivateProxyLimitExceeded = infraerrors.BadRequest("USER_PRIVATE_PROXY_LIMIT_EXCEEDED", "user private proxy limit exceeded")
)

func (s *AccountService) SetProxyProber(prober ProxyExitInfoProber) {
	if s == nil {
		return
	}
	s.proxyProber = prober
}
func (s *AccountService) SetProxyRepository(repo ProxyRepository) {
	if s == nil {
		return
	}
	s.proxyRepo = repo
}

func (s *AccountService) TestOwnedProxy(ctx context.Context, ownerUserID, proxyID int64) (*ProxyTestResult, error) {
	if ownerUserID <= 0 {
		return nil, ErrUserNotFound
	}
	repo, err := s.userPrivateProxyRepo()
	if err != nil {
		return nil, err
	}
	proxy, err := repo.GetOwnedByID(ctx, ownerUserID, proxyID)
	if err != nil {
		return nil, err
	}
	return runPrivateProxyConnectivityTest(ctx, proxy, s.proxyProber), nil
}
func (s *AccountService) UpdateOwnedProxy(ctx context.Context, ownerUserID, proxyID int64, req UpdateProxyRequest) (*Proxy, error) {
	if ownerUserID <= 0 {
		return nil, ErrUserNotFound
	}
	repo, err := s.userPrivateProxyRepo()
	if err != nil {
		return nil, err
	}
	proxy, err := repo.GetOwnedByID(ctx, ownerUserID, proxyID)
	if err != nil {
		return nil, err
	}
	if req.Name != nil {
		proxy.Name = strings.TrimSpace(*req.Name)
	}
	if req.Protocol != nil {
		proxy.Protocol = strings.ToLower(strings.TrimSpace(*req.Protocol))
	}
	if req.Host != nil {
		proxy.Host = strings.TrimSpace(*req.Host)
	}
	if req.Port != nil {
		proxy.Port = *req.Port
	}
	if req.Username != nil {
		proxy.Username = strings.TrimSpace(*req.Username)
	}
	if req.Password != nil {
		proxy.Password = strings.TrimSpace(*req.Password)
	}
	if req.Status != nil {
		proxy.Status = strings.ToLower(strings.TrimSpace(*req.Status))
	}
	normalized, err := normalizeUserPrivateProxyCreate(CreateProxyRequest{
		Name:     proxy.Name,
		Protocol: proxy.Protocol,
		Host:     proxy.Host,
		Port:     proxy.Port,
		Username: proxy.Username,
		Password: proxy.Password,
	})
	if err != nil {
		return nil, err
	}
	proxy.Name = normalized.Name
	proxy.Protocol = normalized.Protocol
	proxy.Host = normalized.Host
	proxy.Port = normalized.Port
	proxy.Username = normalized.Username
	proxy.Password = normalized.Password
	switch proxy.Status {
	case "", StatusActive:
		proxy.Status = StatusActive
	case "inactive", StatusDisabled:
		proxy.Status = "inactive"
	default:
		return nil, ErrUserPrivateProxyInvalid
	}
	proxy.OwnerUserID = &ownerUserID
	if err := repo.Update(ctx, proxy); err != nil {
		return nil, fmt.Errorf("update user private proxy: %w", err)
	}
	return proxy, nil
}

const UserPrivateProxyLimit = 3

func (s *AccountService) ValidateOwnedOAuthProxyID(ctx context.Context, ownerUserID int64, proxyID *int64) (*int64, error) {
	id, err := s.ValidateOwnedProxyID(ctx, ownerUserID, proxyID)
	if errors.Is(err, ErrProxyNotFound) {
		return nil, nil
	}
	return id, err
}
func (s *AccountService) ValidateOwnedProxyID(ctx context.Context, ownerUserID int64, proxyID *int64) (*int64, error) {
	if proxyID == nil {
		return nil, nil
	}
	if *proxyID <= 0 {
		return nil, nil
	}
	repo, err := s.userPrivateProxyRepo()
	if err != nil {
		return nil, err
	}
	proxy, err := repo.GetOwnedByID(ctx, ownerUserID, *proxyID)
	if err != nil {
		return nil, err
	}
	if proxy.Status != StatusActive {
		return nil, ErrUserPrivateProxyInvalid
	}
	id := proxy.ID
	return &id, nil
}
func normalizeUserPrivateProxyCreate(req CreateProxyRequest) (*Proxy, error) {
	name := strings.TrimSpace(req.Name)
	protocol := strings.ToLower(strings.TrimSpace(req.Protocol))
	host := strings.TrimSpace(req.Host)
	username := strings.TrimSpace(req.Username)
	password := strings.TrimSpace(req.Password)
	port := req.Port
	if port == 0 {
		port = 443
	}
	if name == "" || host == "" || port <= 0 || port > 65535 {
		return nil, ErrUserPrivateProxyInvalid
	}
	if strings.Contains(host, "://") || strings.ContainsAny(host, "/?#") {
		return nil, ErrUserPrivateProxyInvalid
	}
	switch protocol {
	case "http", "https", "socks5", "socks5h":
	default:
		return nil, ErrUserPrivateProxyInvalid
	}
	return &Proxy{
		Name:     name,
		Protocol: protocol,
		Host:     host,
		Port:     port,
		Username: username,
		Password: password,
		Status:   StatusActive,
	}, nil
}
func (s *AccountService) userPrivateProxyRepo() (userPrivateProxyRepository, error) {
	if s == nil || s.proxyRepo == nil {
		return nil, ErrOwnedAccountGroupValidationUnavailable
	}
	repo, ok := s.proxyRepo.(userPrivateProxyRepository)
	if !ok {
		return nil, ErrOwnedAccountGroupValidationUnavailable
	}
	return repo, nil
}

type userPrivateProxyRepository interface {
	Create(ctx context.Context, proxy *Proxy) error
	GetOwnedByID(ctx context.Context, ownerUserID, id int64) (*Proxy, error)
	ListOwnedByUserID(ctx context.Context, ownerUserID int64) ([]ProxyWithAccountCount, error)
	CountByOwnerUserID(ctx context.Context, ownerUserID int64) (int64, error)
	CountOwnedAccountsByProxyID(ctx context.Context, ownerUserID, proxyID int64) (int64, error)
	Update(ctx context.Context, proxy *Proxy) error
	Delete(ctx context.Context, id int64) error
}
