package provider

import "ikik-api/internal/payment"

func init() {
	register(payment.TypeEasyPay, func(instanceID string, config map[string]string) (payment.Provider, error) {
		return NewEasyPay(instanceID, config)
	})
	register(payment.TypeAlipay, func(instanceID string, config map[string]string) (payment.Provider, error) {
		return NewAlipay(instanceID, config)
	})
	register(payment.TypeWxpay, func(instanceID string, config map[string]string) (payment.Provider, error) {
		return NewWxpay(instanceID, config)
	})
	register(payment.TypeStripe, func(instanceID string, config map[string]string) (payment.Provider, error) {
		return NewStripe(instanceID, config)
	})
	register(payment.TypeAirwallex, func(instanceID string, config map[string]string) (payment.Provider, error) {
		return NewAirwallex(instanceID, config)
	})
}
