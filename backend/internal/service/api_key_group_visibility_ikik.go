package service

import "strings"

func canUserBindStandardGroup(user *User, group *Group) bool {
	if user == nil || group == nil || group.IsSubscriptionType() {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(group.Scope), GroupScopePublic) {
		return true
	}
	return user.CanBindGroup(group.ID, group.IsExclusive)
}

func isGroupOwnedByUser(group *Group, userID int64) bool {
	return group != nil && group.OwnerUserID != nil && *group.OwnerUserID == userID
}
