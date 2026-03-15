package session

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"time"
)

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

type Message struct {
	ID         string     `json:"id"`
	Role       Role       `json:"role"`
	Content    string     `json:"content"`
	CreatedAt  time.Time  `json:"created_at"`
	ToolName   string     `json:"tool_name,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	Status     string     `json:"status,omitempty"`
}

type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

type Session struct {
	ID        string    `json:"id"`
	RepoRoot  string    `json:"repo_root"`
	Workdir   string    `json:"workdir"`
	Model     string    `json:"model"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Messages  []Message `json:"messages"`
}

type MessageOption func(*Message)

func New(repoRoot, workdir, model string) *Session {
	now := time.Now().UTC()
	return &Session{
		ID:        newID("sess"),
		RepoRoot:  repoRoot,
		Workdir:   workdir,
		Model:     model,
		CreatedAt: now,
		UpdatedAt: now,
		Messages:  make([]Message, 0, 8),
	}
}

func (s *Session) AddMessage(role Role, content string, opts ...MessageOption) Message {
	msg := Message{
		ID:        newID("msg"),
		Role:      role,
		Content:   content,
		CreatedAt: time.Now().UTC(),
	}
	for _, opt := range opts {
		opt(&msg)
	}
	s.Messages = append(s.Messages, msg)
	s.UpdatedAt = msg.CreatedAt
	return msg
}

func WithTool(name, callID, status string) MessageOption {
	return func(msg *Message) {
		msg.ToolName = name
		msg.ToolCallID = callID
		msg.Status = status
	}
}

func WithAssistantToolCalls(calls []ToolCall) MessageOption {
	return func(msg *Message) {
		if len(calls) == 0 {
			return
		}
		msg.ToolCalls = append([]ToolCall(nil), calls...)
	}
}

func newID(prefix string) string {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return prefix + "_fallback"
	}
	return prefix + "_" + hex.EncodeToString(buf)
}
