package provider

import (
	"fmt"
	"strings"

	"minioc/internal/config"
)

func ResolveAPIKey(providerName string, providerConfig config.Provider) (string, error) {
	authType := strings.TrimSpace(providerConfig.AuthType)
	if authType == "" {
		authType = "api_key"
	}
	if authType != "api_key" {
		return "", fmt.Errorf("provider %q auth type %q is not supported", providerName, providerConfig.AuthType)
	}
	return providerConfig.ResolveAPIKey()
}
