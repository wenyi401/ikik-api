package service

import (
	"time"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID             int64
	Email          string
	Username       string
	Notes          string
	AvatarURL      string
	AvatarSource   string
	AvatarMIME     string
	AvatarByteSize int
	AvatarSHA256   string
	PasswordHash   string
	Role           string
	Balance        float64
	// ikik 扩展：分账余额、积分与成长/分享相关字段。
	RechargeBalance                  float64
	InviteIncomeBalance              float64
	ShareIncomeBalance               float64
	PointsBalance                    float64
	PreferPointsBilling              bool
	FrozenBalance                    float64
	Concurrency                      int
	Status                           string
	DeveloperAPIEnabled              bool
	OpenAIExperimentalPromptUnlocked bool
	OnboardingMode                   string
	ShareCardText                    string
	ShareCardTextColor               string
	AllowedGroups                    []int64
	BlockedGroups                    []int64
	// RestrictPublicGroups narrows the public groups this user may bind to the
	// ones listed in AllowedGroups. False keeps the default, where every public
	// group is bindable.
	RestrictPublicGroups bool
	RiskGroupBlocks      []UserRiskGroupBlock
	TokenVersion         int64 // Incremented on password change to invalidate existing tokens
	// TokenVersionResolved indicates TokenVersion already contains the fingerprint-derived
	// value expected in JWT claims and refresh-token state.
	TokenVersionResolved bool
	SignupSource         string
	LastLoginAt          *time.Time
	LastActiveAt         *time.Time
	LastUsedAt           *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
	DeletedAt            *time.Time // 非 nil 表示用户已软删除

	// GroupRates 用户专属分组倍率配置
	// map[groupID]rateMultiplier
	GroupRates map[int64]float64

	// TOTP 双因素认证字段
	TotpSecretEncrypted *string    // AES-256-GCM 加密的 TOTP 密钥
	TotpEnabled         bool       // 是否启用 TOTP
	TotpEnabledAt       *time.Time // TOTP 启用时间

	// 余额不足通知
	BalanceNotifyEnabled       bool
	BalanceNotifyThresholdType string // "fixed" (default) | "percentage"
	BalanceNotifyThreshold     *float64
	BalanceNotifyExtraEmails   []NotifyEmailEntry
	TotalRecharged             float64
	TotalInviteIncome          float64
	TotalShareIncome           float64

	// RPMLimit 用户级每分钟请求数上限（0 = 不限制）。仅在所用分组未设置 rpm_limit
	// 且该 (用户, 分组) 无 rpm_override 时作为全局兜底生效，计数键 rpm:u:{userID}:{min}。
	RPMLimit int

	// UserGroupRPMOverride 来自 auth cache snapshot 的 (user, group) RPM 覆盖值。
	// nil = 该 API Key 对应的 (user, group) 无 override；非 nil 时 checkRPM 直接使用，
	// 避免每请求查 DB。字段不持久化到数据库。
	UserGroupRPMOverride *int

	APIKeys       []APIKey
	Subscriptions []UserSubscription
}

type UserRiskGroupBlock struct {
	GroupID      int64      `json:"group_id"`
	BlockedUntil *time.Time `json:"blocked_until,omitempty"`
	Permanent    bool       `json:"permanent"`
}

func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin
}

func (u *User) IsActive() bool {
	return u.Status == StatusActive
}

// CanBindGroup checks whether a user can bind to a given group.
// Explicitly blocked groups cannot bind. Public groups are available by default;
// restricted users and exclusive groups require an explicit AllowedGroups grant.
// Public groups are available by default; restricted users and exclusive groups
// require an explicit AllowedGroups grant. Explicit blocks always win.
func (u *User) CanBindGroup(groupID int64, isExclusive bool) bool {
	if u.IsGroupBlocked(groupID) {
		return false
	}
	if !isExclusive && !u.RestrictPublicGroups {
		return true
	}
	// 专属分组，以及受限用户的公开分组：需要在 AllowedGroups 中
	for _, id := range u.AllowedGroups {
		if id == groupID {
			return true
		}
	}
	return false
}

func (u *User) IsGroupBlocked(groupID int64) bool {
	return u.isGroupBlockedAt(groupID, time.Now())
}

func (u *User) isGroupBlockedAt(groupID int64, now time.Time) bool {
	if u == nil || groupID <= 0 {
		return false
	}
	for _, id := range u.BlockedGroups {
		if id == groupID {
			return true
		}
	}
	for _, block := range u.RiskGroupBlocks {
		if block.GroupID != groupID {
			continue
		}
		if block.Permanent || (block.BlockedUntil != nil && now.Before(*block.BlockedUntil)) {
			return true
		}
	}
	return false
}

func (u *User) SetPassword(password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hash)
	return nil
}

func (u *User) CheckPassword(password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) == nil
}
