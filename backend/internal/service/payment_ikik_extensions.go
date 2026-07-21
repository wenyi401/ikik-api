package service

func (s *PaymentService) SetShopFulfillment(fulfillment ShopPaymentFulfillment) {
	if s == nil {
		return
	}
	s.shopFulfillment = fulfillment
}
