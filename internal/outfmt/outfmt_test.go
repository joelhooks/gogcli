package outfmt

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestFromFlags(t *testing.T) {
	if _, err := FromFlags(true, true); err == nil {
		t.Fatalf("expected error when combining --json and --plain")
	}

	got, err := FromFlags(true, false)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	if !got.JSON || got.Plain {
		t.Fatalf("unexpected mode: %#v", got)
	}
}

func TestContextMode(t *testing.T) {
	ctx := context.Background()

	if IsJSON(ctx) || IsPlain(ctx) {
		t.Fatalf("expected default text")
	}
	ctx = WithMode(ctx, Mode{JSON: true})

	if !IsJSON(ctx) || IsPlain(ctx) {
		t.Fatalf("expected json-only")
	}
}

func TestWriteJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteJSON(context.Background(), &buf, map[string]any{"ok": true}); err != nil {
		t.Fatalf("err: %v", err)
	}

	if buf.Len() == 0 {
		t.Fatalf("expected output")
	}
}

func TestWriteJSON_ResultsOnlyAndSelect(t *testing.T) {
	ctx := WithJSONTransform(context.Background(), JSONTransform{
		ResultsOnly: true,
		Select:      []string{"id"},
	})

	var buf bytes.Buffer
	if err := WriteJSON(ctx, &buf, map[string]any{
		"files": []map[string]any{
			{"id": "1", "name": "one"},
			{"id": "2", "name": "two"},
		},
		"nextPageToken": "tok",
	}); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	var got []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v (out=%q)", err, buf.String())
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 items, got %d", len(got))
	}

	if got[0]["id"] != "1" || got[1]["id"] != "2" {
		t.Fatalf("unexpected ids: %#v", got)
	}

	if _, ok := got[0]["name"]; ok {
		t.Fatalf("expected name to be stripped, got %#v", got[0])
	}
}

func TestFromEnvAndParseError(t *testing.T) {
	t.Setenv("GOG_JSON", "yes")
	t.Setenv("GOG_PLAIN", "0")
	mode := FromEnv()

	if !mode.JSON || mode.Plain {
		t.Fatalf("unexpected env mode: %#v", mode)
	}

	if err := (&ParseError{msg: "boom"}).Error(); err != "boom" {
		t.Fatalf("unexpected parse error: %q", err)
	}
}

func TestFromContext_WrongType(t *testing.T) {
	ctx := context.WithValue(context.Background(), ctxKey{}, "nope")
	if got := FromContext(ctx); got != (Mode{}) {
		t.Fatalf("expected zero mode, got %#v", got)
	}
}

func TestWriteJSON_WithEnvelope(t *testing.T) {
	ctx := context.Background()
	ctx = WithEnvelope(ctx, true)
	ctx = WithCommand(ctx, "gog version")
	ctx = WithNextActions(ctx, []NextAction{
		{Command: "gog schema version", Description: "Inspect command schema"},
	})

	var buf bytes.Buffer
	if err := WriteJSON(ctx, &buf, map[string]any{"version": "1.2.3"}); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	raw := strings.TrimSpace(buf.String())
	if !strings.Contains(raw, `"ok": true`) || !strings.Contains(raw, `"command": "gog version"`) {
		t.Fatalf("expected envelope output, got: %s", raw)
	}
	if !strings.Contains(raw, `"next_actions"`) {
		t.Fatalf("expected next_actions in envelope, got: %s", raw)
	}
}

func TestWriteJSON_DoesNotDoubleWrapEnvelope(t *testing.T) {
	ctx := WithEnvelope(context.Background(), true)
	var buf bytes.Buffer

	input := map[string]any{
		"ok":      false,
		"command": "gog x",
		"error": map[string]any{
			"message": "boom",
			"code":    "COMMAND_FAILED",
		},
		"fix":          "do x",
		"next_actions": []map[string]any{},
	}

	if err := WriteJSON(ctx, &buf, input); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	var out map[string]any
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out["ok"] != false {
		t.Fatalf("expected error envelope passthrough, got %#v", out)
	}
	if out["command"] != "gog x" {
		t.Fatalf("expected command passthrough, got %#v", out["command"])
	}
}
