package provider

import (
	"fmt"

	"ikik-api/internal/payment"
)

// CreateProvider creates a Provider from a provider key, instance ID and decrypted config.
// Constructor errors (including *infraerrors.ApplicationError from wxpay) are returned
// as-is, never wrapped: the "_validate_" path relies on the structured error for
// frontend i18n (see service/payment_config_providers.go).
func CreateProvider(providerKey string, instanceID string, config map[string]string) (payment.Provider, error) {
	fn, ok := constructors[providerKey]
	if !ok {
		return nil, fmt.Errorf("unknown provider key: %s", providerKey)
	}
	return fn(instanceID, config)
}
