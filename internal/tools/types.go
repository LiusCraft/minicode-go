package tools

import (
	"context"
	"encoding/json"

	"minioc/internal/safety"
)

type Result struct {
	Title  string
	Output string
}

type CallContext struct {
	RepoRoot    string
	Workdir     string
	Permissions *safety.PermissionManager
}

type Spec struct {
	Name        string
	Description string
	Parameters  map[string]any
	Execute     func(ctx context.Context, callCtx CallContext, arguments json.RawMessage) (Result, error)
}
