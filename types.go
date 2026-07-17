package mirrorkit

import "encoding/json"

// Wire types for the Mirrors collector. These mirror the canonical shape the
// backend ingests (the Python `mirrorkit` collector, `mirrorkit-js`, and
// `mirrors/ingest/generic.py`): OpenAI / LangChain-style chat messages. The
// backend's ingestor is lenient and re-normalizes server-side, but we send this
// clean, stable, documented shape on the wire.

// Role is an OpenAI-style chat role.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// ToolFunction is the function block of a tool call (OpenAI convention).
type ToolFunction struct {
	Name string `json:"name"`
	// Arguments is a JSON string (OpenAI convention). The server also tolerates
	// an object, but we send the documented string form.
	Arguments string `json:"arguments"`
}

// ToolCall is one tool invocation issued by an assistant message.
type ToolCall struct {
	ID       string       `json:"id"`
	Function ToolFunction `json:"function"`
}

// Message is one OpenAI-style chat message. Content is always present on the
// wire (null when the message only carries tool calls), matching the sibling
// collectors.
type Message struct {
	Role       Role       `json:"role"`
	Content    *string    `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID *string    `json:"tool_call_id,omitempty"`
}

// ToolDefinition is a tool as the agent's own registry declares it (name +
// description + JSON-Schema parameters). Optional per trace; the backend
// treats it as the ground-truth individual description of the tool.
type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema,omitempty"`
}

// Trace is one collected agent run.
type Trace struct {
	ID       string           `json:"id"`
	Messages []Message        `json:"messages"`
	Model    *string          `json:"model,omitempty"`
	Tools    []ToolDefinition `json:"tools,omitempty"`
}

// WithTools attaches the run's declared tool definitions to the trace.
func (t Trace) WithTools(tools ...ToolDefinition) Trace {
	t.Tools = tools
	return t
}

// CollectBatch is the body POSTed to {endpoint}/api/collect.
type CollectBatch struct {
	Project string  `json:"project"`
	Traces  []Trace `json:"traces"`
}

// --- convenience constructors for the common message shapes ---
//
// These are pure data helpers so callers can build a trace without wrangling
// *string pointers — the collector still only takes and sends what you give it.

func strptr(s string) *string { return &s }

// SystemMessage builds a system text message.
func SystemMessage(content string) Message {
	return Message{Role: RoleSystem, Content: strptr(content)}
}

// UserMessage builds a user text message.
func UserMessage(content string) Message {
	return Message{Role: RoleUser, Content: strptr(content)}
}

// AssistantMessage builds an assistant text message.
func AssistantMessage(content string) Message {
	return Message{Role: RoleAssistant, Content: strptr(content)}
}

// AssistantToolCalls builds an assistant message that only issues tool calls
// (content is null on the wire).
func AssistantToolCalls(calls ...ToolCall) Message {
	return Message{Role: RoleAssistant, ToolCalls: calls}
}

// ToolResult builds a tool-result message answering the call with toolCallID.
func ToolResult(toolCallID, content string) Message {
	return Message{Role: RoleTool, Content: strptr(content), ToolCallID: strptr(toolCallID)}
}

// NewToolCall builds a tool call, JSON-encoding args into the OpenAI-style
// arguments string. A string args value is passed through unchanged.
func NewToolCall(id, name string, args any) ToolCall {
	return ToolCall{ID: id, Function: ToolFunction{Name: name, Arguments: jsonString(args)}}
}

// jsonString encodes v as a JSON string, passing an existing string through and
// falling back to "{}" on an encoding error.
func jsonString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}
