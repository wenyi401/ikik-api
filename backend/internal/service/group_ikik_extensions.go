package service

import (
	"fmt"
	"strings"
	"time"
)

func CarpoolUserGroupName(userID int64, platform string) string {
	return fmt.Sprintf("carpool-u%d-%s", userID, strings.ToLower(strings.TrimSpace(platform)))
}
func (g *Group) EffectiveKiroAutoStickyEnabled() bool {
	return g != nil && g.Platform == PlatformKiro && g.KiroAutoStickyEnabled
}
func (g *Group) EffectiveKiroCacheEmulationEnabled() bool {
	return g != nil && g.Platform == PlatformKiro && g.KiroCacheEmulationEnabled && g.EffectiveKiroCacheEmulationRatio() > 0
}

func (g *Group) EffectiveKiroCacheEmulationRatio() float64 {
	if g == nil || g.Platform != PlatformKiro || !g.KiroCacheEmulationEnabled {
		return 0
	}
	return normalizeKiroCacheEmulationRatio(g.KiroCacheEmulationRatio)
}

func (g *Group) EffectiveKiroEndpointMode() string {
	if g == nil || g.Platform != PlatformKiro {
		return KiroEndpointModeQ
	}
	switch g.KiroEndpointMode {
	case KiroEndpointModeKRS:
		return KiroEndpointModeKRS
	default:
		return KiroEndpointModeQ
	}
}
func (g *Group) EffectiveKiroStickySessionTTL() time.Duration {
	seconds := g.EffectiveKiroStickySessionTTLSeconds()
	if seconds <= 0 {
		seconds = defaultKiroStickySessionTTLSeconds
	}
	return time.Duration(seconds) * time.Second
}
func (g *Group) EffectiveKiroStickySessionTTLSeconds() int {
	if g == nil || g.Platform != PlatformKiro {
		return defaultKiroStickySessionTTLSeconds
	}
	if g.KiroStickySessionTTLSeconds <= 0 {
		return defaultKiroStickySessionTTLSeconds
	}
	return g.KiroStickySessionTTLSeconds
}

func IsSupportedUserCarpoolGroupPlatform(platform string) bool {
	normalized := strings.ToLower(strings.TrimSpace(platform))
	for _, supported := range SupportedUserCarpoolGroupPlatforms() {
		if normalized == supported {
			return true
		}
	}
	return false
}
func IsSupportedUserPrivateGroupPlatform(platform string) bool {
	normalized := strings.ToLower(strings.TrimSpace(platform))
	for _, supported := range SupportedUserPrivateGroupPlatforms() {
		if normalized == supported {
			return true
		}
	}
	return false
}
func (g *Group) IsUserCarpoolScope() bool {
	if g == nil {
		return false
	}
	return NormalizeGroupScope(g.Scope) == GroupScopeUserCarpool
}
func (g *Group) IsUserPrivateScope() bool {
	if g == nil {
		return false
	}
	return NormalizeGroupScope(g.Scope) == GroupScopeUserPrivate
}

const (
	KiroEndpointModeKRS = "krs"
)
const (
	KiroEndpointModeQ = "q"
)

func (g *Group) KiroKRSEnabled() bool {
	return g.EffectiveKiroEndpointMode() == KiroEndpointModeKRS
}

func NormalizeGroupRuntimeFields(g *Group) {
	normalizeKiroCacheEmulationFields(g)
}
func NormalizeGroupScope(scope string) string {
	switch strings.ToLower(strings.TrimSpace(scope)) {
	case GroupScopeUserPrivate:
		return GroupScopeUserPrivate
	case GroupScopeUserCarpool:
		return GroupScopeUserCarpool
	default:
		return GroupScopePublic
	}
}

func PrivateGroupName(userID int64, platform string) string {
	return fmt.Sprintf("private-u%d-%s", userID, strings.ToLower(strings.TrimSpace(platform)))
}

func SupportedUserCarpoolGroupPlatforms() []string {
	return []string{PlatformAnthropic, PlatformOpenAI, PlatformGemini, PlatformAntigravity, PlatformOpenCodeGo}
}
func SupportedUserPrivateGroupPlatforms() []string {
	return []string{PlatformAnthropic, PlatformOpenAI, PlatformGemini, PlatformAntigravity, PlatformGrok, PlatformKiro, PlatformCustom, PlatformOpenCodeGo}
}

const defaultKiroStickySessionTTLSeconds = 3600

func normalizeKiroCacheEmulationFields(g *Group) {
	if g == nil {
		return
	}
	if g.Platform != PlatformKiro {
		g.KiroAutoStickyEnabled = false
		g.KiroStickySessionTTLSeconds = defaultKiroStickySessionTTLSeconds
		g.KiroCacheEmulationEnabled = false
		g.KiroCacheEmulationRatio = 0
		g.KiroEndpointMode = ""
		return
	}
	if g.KiroStickySessionTTLSeconds <= 0 {
		g.KiroStickySessionTTLSeconds = defaultKiroStickySessionTTLSeconds
	}
	if g.KiroCacheEmulationRatio == 0 {
		g.KiroCacheEmulationRatio = 1
	}
	g.KiroCacheEmulationRatio = normalizeKiroCacheEmulationRatio(g.KiroCacheEmulationRatio)
	normalizeKiroEndpointModeField(g)
}
func normalizeKiroCacheEmulationRatio(ratio float64) float64 {
	switch {
	case ratio < 0:
		return 0
	case ratio > 1:
		return 1
	case ratio == 0:
		return 1
	default:
		return ratio
	}
}

func normalizeKiroEndpointModeField(g *Group) {
	if g == nil {
		return
	}
	if g.Platform != PlatformKiro {
		g.KiroEndpointMode = ""
		return
	}
	switch g.KiroEndpointMode {
	case KiroEndpointModeKRS:
	default:
		g.KiroEndpointMode = KiroEndpointModeQ
	}
}
