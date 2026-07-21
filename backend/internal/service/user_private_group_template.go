package service

type UserPrivateGroupTemplate struct {
	DailyLimitUSD   *float64
	WeeklyLimitUSD  *float64
	MonthlyLimitUSD *float64
	RateMultiplier  float64
	RPMLimit        int
	CommissionRate  float64
}
