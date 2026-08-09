package service

import "strings"

func canUserBindStandardGroup(user *User, group *Group) bool {
	if user == nil || group == nil || group.IsSubscriptionType() || user.IsGroupBlocked(group.ID) {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(group.Scope), GroupScopePublic) && !group.IsExclusive {
		return true
	}
	// Exclusive and non-public standard groups always require an explicit
	// administrator grant, regardless of the group's public visibility scope.
	return user.CanBindGroup(group.ID, true)
}

func isGroupOwnedByUser(group *Group, userID int64) bool {
	return group != nil && group.OwnerUserID != nil && *group.OwnerUserID == userID
}
