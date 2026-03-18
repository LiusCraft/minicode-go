package provider

import (
	"fmt"
	"os"
	"strings"

	"minioc/internal/config"
)

func ResolveAPIKey(providerName string, providerConfig config.Provider) (string, error) {
	auth := providerConfig.EffectiveAuth()
	authType := strings.TrimSpace(auth.Type)
	if authType == "" {
		authType = "api_key"
	}
	if authType != "api_key" {
		return "", fmt.Errorf("provider %q auth type %q is not supported", providerName, auth.Type)
	}

	if auth.APIKey.Env != "" {
		value := strings.TrimSpace(os.Getenv(auth.APIKey.Env))
		if value == "" {
			return "", fmt.Errorf("provider %q api_key env %q is empty", providerName, auth.APIKey.Env)
		}
		return value, nil
	}

	if auth.APIKey.Value != "" {
		return auth.APIKey.Value, nil
	}

	return "", fmt.Errorf("provider %q is missing api_key", providerName)
}
