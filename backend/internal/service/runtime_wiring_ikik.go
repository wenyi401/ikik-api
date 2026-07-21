package service

// IkikRuntimeWiring is a Wire-visible side effect that connects optional ikik
// extensions to upstream services without widening upstream constructors.
type IkikRuntimeWiring struct{}

func ProvideIkikRuntimeWiring(
	gatewayService *GatewayService,
	openAIGatewayService *OpenAIGatewayService,
	accountUsageService *AccountUsageService,
	carpoolRepo CarpoolRepository,
	kiroTokenProvider *KiroTokenProvider,
	kiroCooldownStore KiroCooldownStore,
) *IkikRuntimeWiring {
	gatewayService.SetCarpoolRepository(carpoolRepo)
	gatewayService.SetKiroTokenProvider(kiroTokenProvider)
	gatewayService.SetKiroCooldownStore(kiroCooldownStore)
	openAIGatewayService.SetCarpoolRepository(carpoolRepo)
	openAIGatewayService.SetKiroTokenProvider(kiroTokenProvider)
	accountUsageService.SetKiroTokenProvider(kiroTokenProvider)
	accountUsageService.SetKiroCooldownStore(kiroCooldownStore)
	return &IkikRuntimeWiring{}
}
