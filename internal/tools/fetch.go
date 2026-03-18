// This tool is implemented through self-coding!!!

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type fetchArgs struct {
	URL         string            `json:"url"`
	Method      string            `json:"method,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Body        string            `json:"body,omitempty"`
	Timeout     int               `json:"timeout,omitempty"`
	Description string            `json:"description"`
}

func FetchTool() Spec {
	return Spec{
		Name:        "fetch",
		Description: "Make HTTP/HTTPS requests to fetch remote data. Supports GET, POST, PUT, DELETE methods.",
		Parameters: objectSchema(map[string]any{
			"url":         map[string]any{"type": "string", "description": "URL to fetch"},
			"method":      map[string]any{"type": "string", "description": "HTTP method (GET, POST, PUT, DELETE); defaults to GET", "enum": []string{"GET", "POST", "PUT", "DELETE", "HEAD", "PATCH"}},
			"headers":     map[string]any{"type": "object", "description": "Optional HTTP headers", "additionalProperties": map[string]any{"type": "string"}},
			"body":        map[string]any{"type": "string", "description": "Optional request body for POST/PUT/PATCH"},
			"timeout":     map[string]any{"type": "integer", "description": "Timeout in milliseconds; defaults to 30000"},
			"description": map[string]any{"type": "string", "description": "Short explanation of what the request does"},
		}, "url", "description"),
		ParallelSafe: true, // HTTP requests are generally parallel safe
		Execute:      executeFetch,
	}
}

func executeFetch(ctx context.Context, callCtx CallContext, raw json.RawMessage) (Result, error) {
	var args fetchArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return Result{}, fmt.Errorf("decode fetch args: %w", err)
	}

	if strings.TrimSpace(args.URL) == "" {
		return Result{}, fmt.Errorf("URL is required")
	}
	if strings.TrimSpace(args.Description) == "" {
		return Result{}, fmt.Errorf("description is required")
	}

	// Validate URL scheme (only allow HTTP/HTTPS for security)
	if !strings.HasPrefix(args.URL, "http://") && !strings.HasPrefix(args.URL, "https://") {
		return Result{}, fmt.Errorf("URL must start with http:// or https://")
	}

	// Set default method
	method := "GET"
	if args.Method != "" {
		method = strings.ToUpper(args.Method)
	}

	// Set default timeout
	timeout := 30 * time.Second
	if args.Timeout > 0 {
		timeout = time.Duration(args.Timeout) * time.Millisecond
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: timeout,
	}

	// Create request
	var bodyReader io.Reader
	if args.Body != "" {
		bodyReader = strings.NewReader(args.Body)
	}

	req, err := http.NewRequestWithContext(ctx, method, args.URL, bodyReader)
	if err != nil {
		return Result{}, fmt.Errorf("create request: %w", err)
	}

	// Set headers
	if args.Headers != nil {
		for key, value := range args.Headers {
			req.Header.Set(key, value)
		}
	}

	// Set default User-Agent if not provided
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "minioc/1.0")
	}

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return Result{}, fmt.Errorf("read response: %w", err)
	}

	// Prepare response summary
	responseInfo := fmt.Sprintf("Status: %d %s\n", resp.StatusCode, http.StatusText(resp.StatusCode))
	responseInfo += fmt.Sprintf("Content-Type: %s\n", resp.Header.Get("Content-Type"))
	responseInfo += fmt.Sprintf("Content-Length: %d\n\n", len(bodyBytes))

	// Truncate body if too large (e.g., > 100KB)
	bodyStr := string(bodyBytes)
	if len(bodyStr) > 100*1024 {
		bodyStr = bodyStr[:100*1024] + "\n\n[Response truncated to 100KB]"
	}

	output := responseInfo + bodyStr

	return Result{
		Title:  fmt.Sprintf("Fetch %s %s", method, args.URL),
		Output: output,
	}, nil
}
