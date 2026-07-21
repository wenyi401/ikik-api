package provider

import (
	"fmt"

	"ikik-api/internal/payment"
)

// CreateProvider creates a Provider from a provider key, instance ID and decrypted config.
func CreateProvider(providerKey string, instanceID string, config map[string]string) (payment.Provider, error) {
	constructor, ok := constructors[providerKey]
	if !ok {
		return nil, fmt.Errorf("unknown provider key: %s", providerKey)
	}
	return constructor(instanceID, config)
}
