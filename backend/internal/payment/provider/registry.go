package provider

import (
	"fmt"

	"ikik-api/internal/payment"
)

// ConstructorFunc builds a payment.Provider from an instance ID and decrypted config.
type ConstructorFunc func(instanceID string, config map[string]string) (payment.Provider, error)

// constructors is the package-private provider constructor registry.
// It is populated exclusively by each provider file's init() via register()
// and is read-only afterwards, so concurrent CreateProvider calls
// (e.g. RefreshProviders' clear-and-reload loop) are safe and side-effect free.
var constructors = map[string]ConstructorFunc{}

// register adds a provider constructor under the given key.
// A duplicate key panics: that is an init-time instrumentation error
// which must surface as early as possible.
func register(key string, fn ConstructorFunc) {
	if _, exists := constructors[key]; exists {
		panic(fmt.Sprintf("payment/provider: register: provider key %q is already registered (duplicate registration)", key))
	}
	constructors[key] = fn
}
