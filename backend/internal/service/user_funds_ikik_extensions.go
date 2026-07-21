package service

func HasUsageBillingFunds(user *User) bool {
	return user != nil && (user.Balance > 0 || CanUsePointsForUsage(user))
}
