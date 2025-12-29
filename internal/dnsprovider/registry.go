package dnsprovider

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// ProviderRegistration holds all registration data for a provider
type ProviderRegistration struct {
	Name            string
	DisplayName     string
	Factory         func(creds Credentials) (Provider, error)
	PrompterFactory func() CredentialPrompter
	CredsFromMap    func(map[string]string) (Credentials, error)
	SupportsMail    bool
	SupportsPTR     bool
	SupportsNS      bool
}

var (
	registry     = make(map[string]ProviderRegistration)
	registryLock sync.RWMutex
)

// Register adds a provider to the registry.
// This is typically called from init() in each provider file.
func Register(reg ProviderRegistration) {
	registryLock.Lock()
	defer registryLock.Unlock()
	registry[reg.Name] = reg
}

// Get returns provider registration info by name
func Get(name string) (ProviderRegistration, bool) {
	registryLock.RLock()
	defer registryLock.RUnlock()
	info, ok := registry[name]
	return info, ok
}

// List returns all registered provider names sorted alphabetically
func List() []string {
	registryLock.RLock()
	defer registryLock.RUnlock()
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// SupportedProviderNames returns a formatted string of provider names for display
func SupportedProviderNames() string {
	return strings.Join(List(), " | ")
}

// IsValidProvider checks if a provider name is registered
func IsValidProvider(name string) bool {
	_, ok := Get(name)
	return ok
}

// CreateProvider instantiates a provider from credentials
func CreateProvider(creds Credentials) (Provider, error) {
	info, ok := Get(creds.ProviderName())
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s (supported: %s)", creds.ProviderName(), SupportedProviderNames())
	}
	return info.Factory(creds)
}

// CreateProviderFromMap creates a provider from a name and credential map
func CreateProviderFromMap(providerName string, credMap map[string]string) (Provider, error) {
	info, ok := Get(providerName)
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s (supported: %s)", providerName, SupportedProviderNames())
	}
	creds, err := info.CredsFromMap(credMap)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials for %s: %w", providerName, err)
	}
	return info.Factory(creds)
}

// GetPrompter returns the credential prompter for a provider
func GetPrompter(providerName string) (CredentialPrompter, error) {
	info, ok := Get(providerName)
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s (supported: %s)", providerName, SupportedProviderNames())
	}
	return info.PrompterFactory(), nil
}

// GetCredentialsFromMap creates typed credentials from a map for a given provider
func GetCredentialsFromMap(providerName string, credMap map[string]string) (Credentials, error) {
	info, ok := Get(providerName)
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s (supported: %s)", providerName, SupportedProviderNames())
	}
	return info.CredsFromMap(credMap)
}

// ProviderSupportsFeature checks if a provider supports a specific feature
func ProviderSupportsFeature(providerName, feature string) bool {
	info, ok := Get(providerName)
	if !ok {
		return false
	}
	switch feature {
	case "mail":
		return info.SupportsMail
	case "ptr":
		return info.SupportsPTR
	case "ns", "nameserver":
		return info.SupportsNS
	default:
		return false
	}
}
