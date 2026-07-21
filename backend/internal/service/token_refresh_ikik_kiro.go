package service

func (s *TokenRefreshService) AddKiroTokenRefresher(kiroOAuthService *KiroOAuthService) {
	if s == nil || kiroOAuthService == nil {
		return
	}
	refresher := NewKiroTokenRefresher(kiroOAuthService)
	for _, registration := range s.registrations {
		if registration.platform == PlatformKiro {
			return
		}
	}
	s.registrations = append(s.registrations, tokenRefreshRegistration{
		platform:  PlatformKiro,
		refresher: refresher,
		executor:  refresher,
	})
}
