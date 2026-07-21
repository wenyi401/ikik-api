package service

import (
	"context"

	"log/slog"

	"strconv"
	"strings"

	"time"
)

func (s *GatewayService) BindStickySessionForGroup(ctx context.Context, groupID *int64, sessionHash string, accountID int64, group *Group) error {
	return s.BindStickySessionWithTTL(ctx, groupID, sessionHash, accountID, stickySessionTTLForGroup(group))
}

func (s *GatewayService) BindStickySessionWithTTL(ctx context.Context, groupID *int64, sessionHash string, accountID int64, ttl time.Duration) error {
	if sessionHash == "" || accountID <= 0 || s.cache == nil {
		return nil
	}
	if ttl <= 0 {
		ttl = stickySessionTTL
	}
	return s.cache.SetSessionAccountID(ctx, derefGroupID(groupID), sessionHash, accountID, ttl)
}
func (s *GatewayService) hashStickySessionHint(parsed *ParsedRequest, sessionID, source string) (string, bool) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return "", false
	}

	var sb strings.Builder
	if parsed != nil && parsed.SessionContext != nil {
		_, _ = sb.WriteString(strconv.FormatInt(parsed.SessionContext.APIKeyID, 10))
		_, _ = sb.WriteString("|")
	}
	_, _ = sb.WriteString(sessionID)
	hash := s.hashContent(sb.String())
	slog.Info("sticky.hash_source",
		"source", source,
		"hash", hash,
	)
	return hash, true
}

func stickySessionTTLForGroup(group *Group) time.Duration {
	if group != nil && group.Platform == PlatformKiro {
		return group.EffectiveKiroStickySessionTTL()
	}
	return stickySessionTTL
}
