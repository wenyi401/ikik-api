package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	infraerrors "ikik-api/internal/pkg/errors"
)

const (
	CarpoolPoolStatusRecruiting = "recruiting"
	CarpoolPoolStatusFull       = "full"
	CarpoolPoolStatusClosed     = "closed"

	CarpoolPoolVisibilityPublic     = "public"
	CarpoolPoolVisibilityInviteOnly = "invite_only"

	CarpoolJoinRequestStatusPending   = "pending"
	CarpoolJoinRequestStatusApproved  = "approved"
	CarpoolJoinRequestStatusRejected  = "rejected"
	CarpoolJoinRequestStatusActivated = "activated"

	CarpoolMemberRoleOwner  = "owner"
	CarpoolMemberRoleMember = "member"

	CarpoolMemberStatusActive  = "active"
	CarpoolMemberStatusRemoved = "removed"
)

var (
	ErrCarpoolPoolNotFound           = infraerrors.NotFound("CARPOOL_POOL_NOT_FOUND", "carpool pool not found")
	ErrCarpoolJoinRequestNotFound    = infraerrors.NotFound("CARPOOL_JOIN_REQUEST_NOT_FOUND", "carpool join request not found")
	ErrCarpoolMemberNotFound         = infraerrors.NotFound("CARPOOL_MEMBER_NOT_FOUND", "carpool member not found")
	ErrCarpoolInvalidSeats           = infraerrors.BadRequest("CARPOOL_INVALID_SEATS", "carpool seats must be between 2 and 6")
	ErrCarpoolInvalidDuration        = infraerrors.BadRequest("CARPOOL_INVALID_DURATION", "carpool duration must be between 1 and 365 days")
	ErrCarpoolInvalidPlatform        = infraerrors.BadRequest("CARPOOL_INVALID_PLATFORM", "carpool platform is invalid")
	ErrCarpoolInviteCodeRequired     = infraerrors.BadRequest("CARPOOL_INVITE_CODE_REQUIRED", "carpool invite code is required")
	ErrCarpoolAccountsRequired       = infraerrors.BadRequest("CARPOOL_ACCOUNTS_REQUIRED", "at least one account must be bound to the carpool pool")
	ErrCarpoolOwnerOnly              = infraerrors.Forbidden("CARPOOL_OWNER_ONLY", "only the pool owner can perform this action")
	ErrCarpoolPoolClosed             = infraerrors.Forbidden("CARPOOL_POOL_CLOSED", "carpool pool is closed")
	ErrCarpoolPoolFull               = infraerrors.Conflict("CARPOOL_POOL_FULL", "carpool pool is already full")
	ErrCarpoolAlreadyApplied         = infraerrors.Conflict("CARPOOL_ALREADY_APPLIED", "you already have a pending carpool application")
	ErrCarpoolAlreadyMember          = infraerrors.Conflict("CARPOOL_ALREADY_MEMBER", "you are already a member of this carpool pool")
	ErrCarpoolUserAlreadyInPool      = infraerrors.Conflict("CARPOOL_USER_ALREADY_IN_POOL", "you already have an active carpool pool")
	ErrCarpoolSelfJoinNotAllowed     = infraerrors.BadRequest("CARPOOL_SELF_JOIN_NOT_ALLOWED", "pool owner cannot apply to their own carpool pool")
	ErrCarpoolJoinRequestReviewed    = infraerrors.Conflict("CARPOOL_JOIN_REQUEST_REVIEWED", "carpool join request has already been reviewed")
	ErrCarpoolJoinRequestNotApproved = infraerrors.BadRequest("CARPOOL_JOIN_REQUEST_NOT_APPROVED", "carpool join request must be approved before activating the member")
	ErrCarpoolAccountOwnership       = infraerrors.Forbidden("CARPOOL_ACCOUNT_OWNERSHIP_INVALID", "carpool account must belong to the pool owner")
	ErrCarpoolAccountPlatform        = infraerrors.BadRequest("CARPOOL_ACCOUNT_PLATFORM_INVALID", "carpool account platform does not match the pool platform")
	ErrCarpoolAccountAlreadyBound    = infraerrors.Conflict("CARPOOL_ACCOUNT_ALREADY_BOUND", "carpool account is already bound to another active carpool pool")
	ErrCarpoolSystemProxyUnavailable = infraerrors.BadRequest("CARPOOL_SYSTEM_PROXY_UNAVAILABLE", "no active system proxy is available for this carpool pool")
	ErrCarpoolFiveHourLimitExceeded  = infraerrors.TooManyRequests("CARPOOL_5H_LIMIT_EXCEEDED", "carpool 5-hour usage limit exceeded")
	ErrCarpoolInvalidAllocation      = infraerrors.BadRequest("CARPOOL_INVALID_ALLOCATION", "carpool member allocation must include all active members and total 100%")
)

type CarpoolRepository interface {
	CreatePool(ctx context.Context, input CreateCarpoolPoolInput) (*CarpoolPool, error)
	UpdatePoolGroupAndQuota(ctx context.Context, poolID int64, groupID *int64, totals CarpoolQuotaSnapshot) (*CarpoolPool, error)
	UpdatePoolStatus(ctx context.Context, poolID int64, status string) error
	DeletePool(ctx context.Context, poolID int64) error
	GetPoolByID(ctx context.Context, poolID int64) (*CarpoolPool, error)
	GetPoolByGroupID(ctx context.Context, groupID int64) (*CarpoolPool, error)
	GetPoolByInviteCode(ctx context.Context, inviteCode string) (*CarpoolPool, error)
	ListAdminPools(ctx context.Context, filters AdminCarpoolPoolFilters) ([]AdminCarpoolPoolSummary, int64, error)
	ListOwnedPools(ctx context.Context, ownerUserID int64) ([]CarpoolPoolSummary, error)
	ListJoinedPools(ctx context.Context, userID int64) ([]CarpoolPoolSummary, error)
	ListHallPools(ctx context.Context, userID int64) ([]CarpoolPoolSummary, error)
	ListPoolAccounts(ctx context.Context, poolID int64) ([]CarpoolPoolAccount, error)
	FindActivePoolByAccountID(ctx context.Context, accountID, excludePoolID int64) (*CarpoolPool, error)
	ReplacePoolAccounts(ctx context.Context, poolID int64, accountIDs []int64) error
	ListPoolMembers(ctx context.Context, poolID int64) ([]CarpoolMember, error)
	ListPoolJoinRequests(ctx context.Context, poolID int64) ([]CarpoolJoinRequest, error)
	GetJoinRequestByID(ctx context.Context, requestID int64) (*CarpoolJoinRequest, error)
	GetOpenJoinRequestByPoolAndUser(ctx context.Context, poolID, userID int64) (*CarpoolJoinRequest, error)
	CreateJoinRequest(ctx context.Context, poolID, userID int64, note string) (*CarpoolJoinRequest, error)
	UpdateJoinRequestStatus(ctx context.Context, requestID int64, status, reviewNote string, reviewedAt time.Time) (*CarpoolJoinRequest, error)
	ActivateJoinRequest(ctx context.Context, requestID int64, activatedAt time.Time) error
	UpsertMember(ctx context.Context, input UpsertCarpoolMemberInput) (*CarpoolMember, error)
	UpdateMembersFiveHourLimit(ctx context.Context, poolID int64, limitUSD float64) error
	UpdateMembersQuotaFromSnapshot(ctx context.Context, poolID int64, snapshot CarpoolQuotaSnapshot, defaultShareRatio float64) error
	UpdateMemberAllocations(ctx context.Context, poolID int64, updates []CarpoolMemberAllocationUpdate) error
	ResetPoolMembersFiveHourUsage(ctx context.Context, poolID int64, windowStart *time.Time) ([]int64, error)
	ResetPoolMemberWeeklyUsage(ctx context.Context, poolID int64, windowStart time.Time) ([]int64, error)
	IncrementOwnerMemberFiveHourUsage(ctx context.Context, poolID, ownerUserID int64, costUSD float64, occurredAt time.Time) (*CarpoolMember, error)
	GetMemberByPoolAndUser(ctx context.Context, poolID, userID int64) (*CarpoolMember, error)
	GetMemberByID(ctx context.Context, memberID int64) (*CarpoolMember, error)
	UpdateMemberStatus(ctx context.Context, memberID int64, status string, removedAt time.Time) error
	GetRuntimeMemberLimitByGroupAndUser(ctx context.Context, groupID, userID int64, now time.Time) (*CarpoolRuntimeMemberLimit, error)
	ListPoolApplicantUsageStats(ctx context.Context, poolID int64) (map[int64]CarpoolApplicantUsageStats, error)
	ListPoolMemberUsageStats(ctx context.Context, groupID int64, userIDs []int64) (map[int64]CarpoolMemberUsageStats, error)
	UpdatePoolAccountExternalUsage(ctx context.Context, poolID, accountID int64, update CarpoolPoolAccountExternalUsageUpdate) error
	MarkPoolAccountExternalOverageNotified(ctx context.Context, poolID, accountID int64, notifiedAt time.Time) error
}

type CreateCarpoolPoolInput struct {
	OwnerUserID          int64
	InviteCode           string
	Name                 string
	Platform             string
	Visibility           string
	TargetSeats          int
	DurationDays         int
	SeatPrice            float64
	ExtraFee             float64
	ExtraFeeDescription  string
	SystemProxyEnabled   bool
	RiskControlEnabled   bool
	Notes                string
	InitialQuotaSnapshot CarpoolQuotaSnapshot
}

type UpsertCarpoolMemberInput struct {
	PoolID             int64
	UserID             int64
	SubscriptionID     *int64
	Role               string
	Status             string
	PaidConfirmedAt    *time.Time
	QuotaShareRatio    float64
	FiveHourLimitUSD   float64
	WeeklyLimitUSD     float64
	ResetFiveHourUsage bool
}

type CarpoolMemberAllocationUpdate struct {
	MemberID         int64
	QuotaShareRatio  float64
	FiveHourLimitUSD float64
	WeeklyLimitUSD   float64
}

type CarpoolPool struct {
	ID                        int64
	OwnerUserID               int64
	GroupID                   *int64
	InviteCode                string
	Name                      string
	Platform                  string
	Status                    string
	Visibility                string
	TargetSeats               int
	DurationDays              int
	SeatPrice                 float64
	ExtraFee                  float64
	ExtraFeeDescription       string
	SystemProxyEnabled        bool
	RiskControlEnabled        bool
	Notes                     string
	TotalFiveHourLimitUSD     float64
	TotalWeeklyLimitUSD       float64
	PerMemberFiveHourLimitUSD float64
	PerMemberWeeklyLimitUSD   float64
	QuotaSnapshotAt           *time.Time
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
}

type CarpoolPoolAccount struct {
	ID                        int64
	PoolID                    int64
	AccountID                 int64
	Name                      string
	Platform                  string
	Type                      string
	AccountLevel              string
	Status                    string
	FiveHourLimitUSD          float64
	WeeklyLimitUSD            float64
	ExternalFiveHourUsedUSD   float64
	ExternalWeeklyUsedUSD     float64
	ExternalFiveHourResetAt   *time.Time
	ExternalWeeklyResetAt     *time.Time
	ExternalCheckedAt         *time.Time
	ExternalOverageNotifiedAt *time.Time
	CreatedAt                 time.Time
}

type CarpoolPoolAccountExternalUsageUpdate struct {
	ExternalFiveHourUsedUSD float64
	ExternalWeeklyUsedUSD   float64
	ExternalFiveHourResetAt *time.Time
	ExternalWeeklyResetAt   *time.Time
	CheckedAt               time.Time
}

type CarpoolApplicantUsageStats struct {
	TotalRequests   int64
	TotalTokens     int64
	Last7dRequests  int64
	Last7dTokens    int64
	Last30dRequests int64
	Last30dTokens   int64
}

type CarpoolMemberUsageStats struct {
	TotalTokens  int64
	TotalCostUSD float64
}

type CarpoolJoinRequest struct {
	ID          int64
	PoolID      int64
	UserID      int64
	Status      string
	Note        string
	ReviewNote  string
	ReviewedAt  *time.Time
	ActivatedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type CarpoolMember struct {
	ID                  int64
	PoolID              int64
	UserID              int64
	SubscriptionID      *int64
	Role                string
	Status              string
	PaidConfirmedAt     *time.Time
	QuotaShareRatio     float64
	FiveHourLimitUSD    float64
	FiveHourUsedUSD     float64
	WeeklyLimitUSD      float64
	FiveHourWindowStart *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type CarpoolRuntimeMemberLimit struct {
	PoolID              int64
	MemberID            int64
	FiveHourLimitUSD    float64
	FiveHourUsedUSD     float64
	FiveHourWindowStart *time.Time
	WeeklyLimitUSD      float64
	WeeklyUsageUSD      float64
}

type CarpoolQuotaSnapshot struct {
	TotalFiveHourLimitUSD     float64
	TotalWeeklyLimitUSD       float64
	PerMemberFiveHourLimitUSD float64
	PerMemberWeeklyLimitUSD   float64
	SnapshotAt                *time.Time
}

type CarpoolPoolSummary struct {
	Pool                 CarpoolPool
	GroupName            string
	ActiveMembers        int
	PendingApplications  int
	BoundAccountCount    int
	IsOwner              bool
	CurrentUserStatus    string
	CurrentUserRequestID *int64
}

type AdminCarpoolPoolFilters struct {
	Page        int
	PageSize    int
	Search      string
	Platform    string
	Status      string
	OwnerUserID int64
}

type AdminCarpoolPoolSummary struct {
	CarpoolPoolSummary
	OwnerEmail    string
	OwnerUsername string
}

type AdminCarpoolPoolListResult struct {
	Items []AdminCarpoolPoolSummary
	Total int64
}

type CarpoolPoolDetail struct {
	Pool             CarpoolPool
	Group            *Group
	Summaries        CarpoolPoolSummary
	Accounts         []CarpoolPoolAccount
	PoolUsageWindows []CarpoolUsageWindow
	Members          []CarpoolMemberProfile
	JoinRequests     []CarpoolJoinRequestProfile
}

type CarpoolUsageWindow struct {
	Window          string
	UsedPoints      float64
	LimitPoints     float64
	RemainingPoints float64
	Utilization     float64
	ResetAt         *time.Time
}

type CarpoolUsageOverview struct {
	Pool    CarpoolPool
	Member  CarpoolMemberProfile
	Windows []CarpoolUsageWindow
}

type CarpoolMemberProfile struct {
	Member         CarpoolMember
	MaskedEmail    string
	Username       string
	WeeklyLimitUSD float64
	WeeklyUsageUSD float64
	WeeklyResetAt  *time.Time
	UsageWindows   []CarpoolUsageWindow
	TotalTokens    int64
	TotalCostUSD   float64
}

type CarpoolJoinRequestProfile struct {
	Request     CarpoolJoinRequest
	MaskedEmail string
	Username    string
	Usage       CarpoolApplicantUsageStats
}

type CreateCarpoolPoolRequest struct {
	Name                string
	Platform            string
	Visibility          string
	TargetSeats         int
	DurationDays        int
	SeatPrice           float64
	ExtraFee            float64
	ExtraFeeDescription string
	SystemProxyEnabled  bool
	RiskControlEnabled  bool
	Notes               string
}

type BindCarpoolAccountsRequest struct {
	AccountIDs []int64
}

type ApplyCarpoolPoolRequest struct {
	Note string
}

type ReviewCarpoolJoinRequest struct {
	ReviewNote string
}

type UpdateCarpoolMemberAllocationsRequest struct {
	Allocations []CarpoolMemberAllocationInput
}

type CarpoolMemberAllocationInput struct {
	MemberID        int64
	QuotaShareRatio float64
}

func NormalizeCarpoolPoolVisibility(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case CarpoolPoolVisibilityInviteOnly, "invite", "invite-only", "private":
		return CarpoolPoolVisibilityInviteOnly
	default:
		return CarpoolPoolVisibilityPublic
	}
}

func NormalizeCarpoolPlatform(platform string) string {
	return strings.ToLower(strings.TrimSpace(platform))
}

func IsSupportedCarpoolPlatform(platform string) bool {
	switch NormalizeCarpoolPlatform(platform) {
	case PlatformOpenAI, PlatformAnthropic, PlatformGemini, PlatformAntigravity:
		return true
	default:
		return false
	}
}

func MaskCarpoolEmail(email string) string {
	email = strings.TrimSpace(email)
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		if len(email) <= 3 {
			return email
		}
		return email[:3] + "***"
	}
	local := parts[0]
	if len(local) <= 3 {
		return local + "***@" + parts[1]
	}
	return local[:3] + "***@" + parts[1]
}

func NormalizeCarpoolPoolStatus(status string, activeMembers, targetSeats int) string {
	if strings.EqualFold(strings.TrimSpace(status), CarpoolPoolStatusClosed) {
		return CarpoolPoolStatusClosed
	}
	if activeMembers >= targetSeats && targetSeats > 0 {
		return CarpoolPoolStatusFull
	}
	return CarpoolPoolStatusRecruiting
}

func CarpoolGroupName(poolID int64, name string) string {
	slug := strings.TrimSpace(name)
	if slug == "" {
		slug = "pool"
	}
	return fmt.Sprintf("Carpool %d · %s", poolID, slug)
}
