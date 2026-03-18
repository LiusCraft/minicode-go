package models

import (
	"fmt"
	"strings"
)

type Ref struct {
	Raw      string
	Provider string
	Model    string
}

func ParseRef(raw string) (Ref, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Ref{}, fmt.Errorf("model reference is empty")
	}

	parts := strings.Split(raw, "/")
	if len(parts) != 2 {
		return Ref{}, fmt.Errorf("model reference %q must use provider/model", raw)
	}

	provider := strings.TrimSpace(parts[0])
	model := strings.TrimSpace(parts[1])
	if provider == "" || model == "" {
		return Ref{}, fmt.Errorf("model reference %q must use provider/model", raw)
	}

	return Ref{
		Raw:      raw,
		Provider: provider,
		Model:    model,
	}, nil
}
