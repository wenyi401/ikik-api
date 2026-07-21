package service

func CanUsePointsForUsage(user *User) bool {
	return user != nil && user.PreferPointsBilling && user.PointsBalance > 0
}
