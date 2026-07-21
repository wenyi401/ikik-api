package service

func (s *OpenAIGatewayService) SetCarpoolRepository(repo CarpoolRepository) {
	if s != nil {
		s.carpoolRepo = repo
	}
}
