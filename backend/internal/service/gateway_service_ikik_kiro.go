package service

func (s *GatewayService) SetKiroCooldownStore(store KiroCooldownStore) {
	if s != nil {
		s.kiroCooldownStore = store
	}
}
func (s *GatewayService) SetKiroTokenProvider(provider *KiroTokenProvider) {
	if s != nil {
		s.kiroTokenProvider = provider
	}
}

func isKiroGroup(group *Group) bool {
	return group != nil && group.Platform == PlatformKiro
}
