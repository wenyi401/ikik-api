package service

func (s *OpenAIOAuthService) PrivacyClientFactory() PrivacyClientFactory {
	if s == nil {
		return nil
	}
	return s.privacyClientFactory
}
