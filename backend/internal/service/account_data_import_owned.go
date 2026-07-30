package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	infraerrors "ikik-api/internal/pkg/errors"
)

const MaxOwnedAccountDataImportItems = MaxAccountCredentialImportItems

var ErrOwnedAccountDataImportInvalid = infraerrors.BadRequest(
	"OWNED_ACCOUNT_DATA_IMPORT_INVALID",
	"invalid user account import data",
)

type OwnedAccountDataImportDefaults struct {
	Concurrency        *int  `json:"concurrency"`
	Priority           *int  `json:"priority"`
	AutoPauseOnExpired *bool `json:"auto_pause_on_expired"`
}

type OwnedAccountDataImportOptions struct {
	GroupIDs        []int64
	AccountDefaults *OwnedAccountDataImportDefaults
}

type OwnedAccountDataImportError struct {
	Kind     string `json:"kind"`
	Name     string `json:"name,omitempty"`
	ProxyKey string `json:"proxy_key,omitempty"`
	Message  string `json:"message"`
}

type OwnedAccountDataImportResult struct {
	ProxyCreated   int                           `json:"proxy_created"`
	ProxyReused    int                           `json:"proxy_reused"`
	ProxyFailed    int                           `json:"proxy_failed"`
	AccountCreated int                           `json:"account_created"`
	AccountFailed  int                           `json:"account_failed"`
	Errors         []OwnedAccountDataImportError `json:"errors,omitempty"`
}

type ownedDataProxyStatusUpdate struct {
	ID       int64
	Name     string
	ProxyKey string
	Status   string
}

func BuildAccountDataProxyKey(protocol, host string, port int, username, password string) string {
	return fmt.Sprintf("%s|%s|%d|%s|%s",
		strings.ToLower(strings.TrimSpace(protocol)),
		strings.TrimSpace(host),
		port,
		strings.TrimSpace(username),
		strings.TrimSpace(password),
	)
}

func (s *AccountService) ImportOwnedData(
	ctx context.Context,
	ownerUserID int64,
	payload AccountDataPayload,
	opts OwnedAccountDataImportOptions,
) (OwnedAccountDataImportResult, error) {
	result := OwnedAccountDataImportResult{}
	if ownerUserID <= 0 {
		return result, ErrUserNotFound
	}
	if err := validateOwnedAccountDataPayload(payload); err != nil {
		return result, err
	}
	if len(opts.GroupIDs) > 0 {
		return result, ErrOwnedAccountDataImportInvalid.WithMetadata(map[string]string{"field": "group_ids"})
	}

	needsProxyLookup := len(payload.Proxies) > 0
	if !needsProxyLookup {
		for i := range payload.Accounts {
			if dataAccountProxyKey(payload.Accounts[i]) != "" {
				needsProxyLookup = true
				break
			}
		}
	}
	proxyKeyToID, inactiveProxyIDs, proxyStatusUpdates, err := s.importOwnedDataProxies(ctx, ownerUserID, payload.Proxies, needsProxyLookup, &result)
	if err != nil {
		return result, err
	}
	for i := range payload.Accounts {
		item := payload.Accounts[i]
		proxyID, proxyErr := resolveOwnedDataAccountProxyID(item, proxyKeyToID)
		if proxyErr != nil {
			result.AccountFailed++
			result.Errors = append(result.Errors, OwnedAccountDataImportError{
				Kind:     "account",
				Name:     item.Name,
				ProxyKey: dataAccountProxyKey(item),
				Message:  proxyErr.Error(),
			})
			continue
		}

		concurrency := item.Concurrency
		priority := item.Priority
		autoPauseOnExpired := item.AutoPauseOnExpired
		if opts.AccountDefaults != nil {
			if opts.AccountDefaults.Concurrency != nil {
				concurrency = *opts.AccountDefaults.Concurrency
			}
			if opts.AccountDefaults.Priority != nil {
				priority = *opts.AccountDefaults.Priority
			}
			if opts.AccountDefaults.AutoPauseOnExpired != nil {
				autoPauseOnExpired = opts.AccountDefaults.AutoPauseOnExpired
			}
		}

		var expiresAt *time.Time
		if item.ExpiresAt != nil && *item.ExpiresAt > 0 {
			t := time.Unix(*item.ExpiresAt, 0).UTC()
			expiresAt = &t
		}
		schedulable := true
		if proxyID != nil {
			_, inactive := inactiveProxyIDs[*proxyID]
			schedulable = !inactive
		}
		_, createErr := s.importOwnedDataAccount(ctx, ownerUserID, CreateAccountRequest{
			Name:               strings.TrimSpace(item.Name),
			Notes:              item.Notes,
			Platform:           strings.ToLower(strings.TrimSpace(item.Platform)),
			AccountLevel:       AccountLevelUnknown,
			Type:               strings.ToLower(strings.TrimSpace(item.Type)),
			Credentials:        item.Credentials,
			Extra:              item.Extra,
			ShareMode:          AccountShareModePrivate,
			ProxyID:            proxyID,
			Concurrency:        concurrency,
			Priority:           priority,
			GroupIDs:           nil,
			ExpiresAt:          expiresAt,
			AutoPauseOnExpired: autoPauseOnExpired,
		}, schedulable)
		if createErr != nil {
			result.AccountFailed++
			result.Errors = append(result.Errors, OwnedAccountDataImportError{
				Kind:    "account",
				Name:    item.Name,
				Message: createErr.Error(),
			})
			continue
		}
		result.AccountCreated++
	}
	s.applyOwnedDataProxyStatuses(ctx, ownerUserID, proxyStatusUpdates, &result)
	return result, nil
}

func (s *AccountService) importOwnedDataAccount(
	ctx context.Context,
	ownerUserID int64,
	req CreateAccountRequest,
	schedulable bool,
) (*Account, error) {
	return s.createOwnedWithOptions(ctx, ownerUserID, req, ownedAccountCreateOptions{
		validateProxyID: s.validateOwnedDataImportProxyID,
		schedulable:     schedulable,
	})
}

func (s *AccountService) validateOwnedDataImportProxyID(
	ctx context.Context,
	ownerUserID int64,
	proxyID *int64,
) (*int64, error) {
	if proxyID == nil || *proxyID <= 0 {
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
	switch strings.ToLower(strings.TrimSpace(proxy.Status)) {
	case StatusActive, "inactive", StatusDisabled:
		id := proxy.ID
		return &id, nil
	default:
		return nil, ErrUserPrivateProxyInvalid
	}
}

func validateOwnedAccountDataPayload(payload AccountDataPayload) error {
	if payload.Type != "" && payload.Type != "sub2api-data" && payload.Type != "sub2api-bundle" {
		return ErrOwnedAccountDataImportInvalid.WithMetadata(map[string]string{"field": "type"})
	}
	if payload.Version != 0 && payload.Version != 1 {
		return ErrOwnedAccountDataImportInvalid.WithMetadata(map[string]string{"field": "version"})
	}
	if payload.Proxies == nil {
		return ErrOwnedAccountDataImportInvalid.WithMetadata(map[string]string{"field": "proxies"})
	}
	if payload.Accounts == nil {
		return ErrOwnedAccountDataImportInvalid.WithMetadata(map[string]string{"field": "accounts"})
	}
	if len(payload.Proxies) > MaxOwnedAccountDataImportItems || len(payload.Accounts) > MaxOwnedAccountDataImportItems {
		return ErrOwnedAccountDataImportInvalid.WithMetadata(map[string]string{
			"field": "items",
			"limit": fmt.Sprintf("%d", MaxOwnedAccountDataImportItems),
		})
	}
	return nil
}

func (s *AccountService) importOwnedDataProxies(
	ctx context.Context,
	ownerUserID int64,
	items []AccountDataProxy,
	needsProxyLookup bool,
	result *OwnedAccountDataImportResult,
) (map[string]int64, map[int64]struct{}, []ownedDataProxyStatusUpdate, error) {
	if !needsProxyLookup {
		return map[string]int64{}, map[int64]struct{}{}, nil, nil
	}

	existing, err := s.ListOwnedProxies(ctx, ownerUserID)
	if err != nil {
		return nil, nil, nil, err
	}
	canonicalToID := make(map[string]int64, len(existing)+len(items))
	proxyKeyToID := make(map[string]int64, len(existing)+len(items)*2)
	inactiveProxyIDs := make(map[int64]struct{}, len(existing)+len(items))
	statusUpdates := make([]ownedDataProxyStatusUpdate, 0, len(items))
	for i := range existing {
		key := BuildAccountDataProxyKey(existing[i].Protocol, existing[i].Host, existing[i].Port, existing[i].Username, existing[i].Password)
		canonicalToID[key] = existing[i].ID
		proxyKeyToID[key] = existing[i].ID
		if isOwnedDataProxyInactive(existing[i].Status) {
			inactiveProxyIDs[existing[i].ID] = struct{}{}
		}
	}

	for i := range items {
		item := items[i]
		inputKey := strings.TrimSpace(item.ProxyKey)
		desiredStatus, statusErr := normalizeOwnedDataImportedProxyStatus(item.Status)
		if statusErr != nil {
			result.ProxyFailed++
			result.Errors = append(result.Errors, OwnedAccountDataImportError{
				Kind:     "proxy",
				Name:     item.Name,
				ProxyKey: inputKey,
				Message:  statusErr.Error(),
			})
			continue
		}
		normalized, normalizeErr := normalizeUserPrivateProxyCreate(CreateProxyRequest{
			Name:     defaultOwnedDataProxyName(item.Name),
			Protocol: item.Protocol,
			Host:     item.Host,
			Port:     item.Port,
			Username: item.Username,
			Password: item.Password,
		})
		if normalizeErr != nil {
			result.ProxyFailed++
			result.Errors = append(result.Errors, OwnedAccountDataImportError{
				Kind:     "proxy",
				Name:     item.Name,
				ProxyKey: inputKey,
				Message:  normalizeErr.Error(),
			})
			continue
		}
		canonicalKey := BuildAccountDataProxyKey(normalized.Protocol, normalized.Host, normalized.Port, normalized.Username, normalized.Password)
		proxyID, reused := canonicalToID[canonicalKey]
		if inputKey != "" {
			if previousID, exists := proxyKeyToID[inputKey]; exists && (!reused || previousID != proxyID) {
				result.ProxyFailed++
				result.Errors = append(result.Errors, OwnedAccountDataImportError{
					Kind:     "proxy",
					Name:     item.Name,
					ProxyKey: inputKey,
					Message:  "proxy_key maps to multiple private proxies",
				})
				continue
			}
		}
		if !reused {
			created, createErr := s.CreateOwnedProxy(ctx, ownerUserID, CreateProxyRequest{
				Name:     normalized.Name,
				Protocol: normalized.Protocol,
				Host:     normalized.Host,
				Port:     normalized.Port,
				Username: normalized.Username,
				Password: normalized.Password,
			})
			if createErr != nil {
				result.ProxyFailed++
				result.Errors = append(result.Errors, OwnedAccountDataImportError{
					Kind:     "proxy",
					Name:     item.Name,
					ProxyKey: inputKey,
					Message:  createErr.Error(),
				})
				continue
			}
			proxyID = created.ID
			canonicalToID[canonicalKey] = proxyID
			result.ProxyCreated++
			if desiredStatus == "inactive" {
				inactiveProxyIDs[proxyID] = struct{}{}
				statusUpdates = append(statusUpdates, ownedDataProxyStatusUpdate{
					ID:       proxyID,
					Name:     item.Name,
					ProxyKey: inputKey,
					Status:   desiredStatus,
				})
			}
		} else {
			result.ProxyReused++
		}
		proxyKeyToID[canonicalKey] = proxyID
		if inputKey != "" {
			proxyKeyToID[inputKey] = proxyID
		}
	}
	return proxyKeyToID, inactiveProxyIDs, statusUpdates, nil
}

func (s *AccountService) applyOwnedDataProxyStatuses(
	ctx context.Context,
	ownerUserID int64,
	updates []ownedDataProxyStatusUpdate,
	result *OwnedAccountDataImportResult,
) {
	for i := range updates {
		status := updates[i].Status
		if _, err := s.UpdateOwnedProxy(ctx, ownerUserID, updates[i].ID, UpdateProxyRequest{Status: &status}); err != nil {
			result.ProxyFailed++
			result.Errors = append(result.Errors, OwnedAccountDataImportError{
				Kind:     "proxy",
				Name:     updates[i].Name,
				ProxyKey: updates[i].ProxyKey,
				Message:  "restore private proxy status: " + err.Error(),
			})
		}
	}
}

func normalizeOwnedDataImportedProxyStatus(status string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", StatusActive, "expired":
		return StatusActive, nil
	case "inactive", StatusDisabled:
		return "inactive", nil
	default:
		return "", ErrUserPrivateProxyInvalid
	}
}

func isOwnedDataProxyInactive(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "inactive", StatusDisabled:
		return true
	default:
		return false
	}
}

func defaultOwnedDataProxyName(name string) string {
	if strings.TrimSpace(name) == "" {
		return "imported-proxy"
	}
	return name
}

func resolveOwnedDataAccountProxyID(item AccountDataAccount, proxyKeyToID map[string]int64) (*int64, error) {
	key := dataAccountProxyKey(item)
	if key == "" {
		return nil, nil
	}
	id, ok := proxyKeyToID[key]
	if !ok || id <= 0 {
		return nil, fmt.Errorf("private proxy_key not found")
	}
	return &id, nil
}

func dataAccountProxyKey(item AccountDataAccount) string {
	if item.ProxyKey == nil {
		return ""
	}
	return strings.TrimSpace(*item.ProxyKey)
}
