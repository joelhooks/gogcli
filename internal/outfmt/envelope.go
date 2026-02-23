package outfmt

import (
	"context"
)

type NextActionParam struct {
	Description string   `json:"description,omitempty"`
	Value       any      `json:"value,omitempty"`
	Default     any      `json:"default,omitempty"`
	Enum        []string `json:"enum,omitempty"`
	Required    bool     `json:"required,omitempty"`
}

type NextAction struct {
	Command     string                     `json:"command"`
	Description string                     `json:"description"`
	Params      map[string]NextActionParam `json:"params,omitempty"`
}

type ErrorBody struct {
	Message string `json:"message"`
	Code    string `json:"code"`
}

type SuccessEnvelope struct {
	Ok          bool         `json:"ok"`
	Command     string       `json:"command"`
	Result      any          `json:"result"`
	NextActions []NextAction `json:"next_actions"`
}

type ErrorEnvelope struct {
	Ok          bool         `json:"ok"`
	Command     string       `json:"command"`
	Error       ErrorBody    `json:"error"`
	Fix         string       `json:"fix"`
	NextActions []NextAction `json:"next_actions"`
}

type commandCtxKey struct{}

func WithCommand(ctx context.Context, command string) context.Context {
	return context.WithValue(ctx, commandCtxKey{}, command)
}

func CommandFromContext(ctx context.Context) string {
	if v := ctx.Value(commandCtxKey{}); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}

	return ""
}

type nextActionsCtxKey struct{}

func WithNextActions(ctx context.Context, nextActions []NextAction) context.Context {
	cp := make([]NextAction, len(nextActions))
	copy(cp, nextActions)
	return context.WithValue(ctx, nextActionsCtxKey{}, cp)
}

func NextActionsFromContext(ctx context.Context) []NextAction {
	if v := ctx.Value(nextActionsCtxKey{}); v != nil {
		if actions, ok := v.([]NextAction); ok {
			cp := make([]NextAction, len(actions))
			copy(cp, actions)
			return cp
		}
	}

	return nil
}

type envelopeCtxKey struct{}

func WithEnvelope(ctx context.Context, enabled bool) context.Context {
	return context.WithValue(ctx, envelopeCtxKey{}, enabled)
}

func EnvelopeEnabledFromContext(ctx context.Context) bool {
	if v := ctx.Value(envelopeCtxKey{}); v != nil {
		if enabled, ok := v.(bool); ok {
			return enabled
		}
	}

	return false
}
