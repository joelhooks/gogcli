package cmd

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

func TestExecute_DriveDelete_DefaultAndPermanent(t *testing.T) {
	t.Run("default_trash", func(t *testing.T) {
		origNew := newDriveService
		t.Cleanup(func() { newDriveService = origNew })

		var patchCount int
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.Contains(r.URL.Path, "/files/id1") || (r.Method != http.MethodPatch && r.Method != http.MethodPut) {
				http.NotFound(w, r)
				return
			}
			patchCount++
			if got := r.URL.Query().Get("supportsAllDrives"); got != "true" {
				t.Fatalf("expected supportsAllDrives=true, got: %q (raw=%q)", got, r.URL.RawQuery)
			}
			body, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(body), "\"trashed\":true") {
				t.Fatalf("expected trashed=true body, got: %q", string(body))
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":      "id1",
				"trashed": true,
				"kind":    "drive#file",
			})
		}))
		defer srv.Close()

		svc, err := drive.NewService(context.Background(),
			option.WithoutAuthentication(),
			option.WithHTTPClient(srv.Client()),
			option.WithEndpoint(srv.URL+"/"),
		)
		if err != nil {
			t.Fatalf("NewService: %v", err)
		}
		newDriveService = func(context.Context, string) (*drive.Service, error) { return svc, nil }

		out := captureStdout(t, func() {
			_ = captureStderr(t, func() {
				if execErr := Execute([]string{"--force", "--account", "a@b.com", "drive", "delete", "id1"}); execErr != nil {
					t.Fatalf("Execute: %v", execErr)
				}
			})
		})
		if !strings.Contains(out, "trashed\ttrue") || !strings.Contains(out, "deleted\tfalse") {
			t.Fatalf("unexpected text output: %q", out)
		}

		jsonOut := captureStdout(t, func() {
			_ = captureStderr(t, func() {
				if execErr := Execute([]string{"--json", "--force", "--account", "a@b.com", "drive", "delete", "id1"}); execErr != nil {
					t.Fatalf("Execute: %v", execErr)
				}
			})
		})
		var parsed struct {
			Trashed bool   `json:"trashed"`
			Deleted bool   `json:"deleted"`
			ID      string `json:"id"`
		}
		if err := json.Unmarshal([]byte(jsonOut), &parsed); err != nil {
			t.Fatalf("json parse: %v\nout=%q", err, jsonOut)
		}
		if !parsed.Trashed || parsed.Deleted || parsed.ID != "id1" {
			t.Fatalf("unexpected json output: %#v", parsed)
		}

		if patchCount != 2 {
			t.Fatalf("expected 2 PATCH calls, got %d", patchCount)
		}
	})

	t.Run("permanent_delete", func(t *testing.T) {
		origNew := newDriveService
		t.Cleanup(func() { newDriveService = origNew })

		var deleteCount int
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.Contains(r.URL.Path, "/files/id1") || r.Method != http.MethodDelete {
				http.NotFound(w, r)
				return
			}
			deleteCount++
			if got := r.URL.Query().Get("supportsAllDrives"); got != "true" {
				t.Fatalf("expected supportsAllDrives=true, got: %q (raw=%q)", got, r.URL.RawQuery)
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		svc, err := drive.NewService(context.Background(),
			option.WithoutAuthentication(),
			option.WithHTTPClient(srv.Client()),
			option.WithEndpoint(srv.URL+"/"),
		)
		if err != nil {
			t.Fatalf("NewService: %v", err)
		}
		newDriveService = func(context.Context, string) (*drive.Service, error) { return svc, nil }

		out := captureStdout(t, func() {
			_ = captureStderr(t, func() {
				if execErr := Execute([]string{"--force", "--account", "a@b.com", "drive", "delete", "id1", "--permanent"}); execErr != nil {
					t.Fatalf("Execute: %v", execErr)
				}
			})
		})
		if !strings.Contains(out, "trashed\tfalse") || !strings.Contains(out, "deleted\ttrue") {
			t.Fatalf("unexpected text output: %q", out)
		}

		jsonOut := captureStdout(t, func() {
			_ = captureStderr(t, func() {
				if execErr := Execute([]string{"--json", "--force", "--account", "a@b.com", "drive", "delete", "id1", "--permanent"}); execErr != nil {
					t.Fatalf("Execute: %v", execErr)
				}
			})
		})
		var parsed struct {
			Trashed bool   `json:"trashed"`
			Deleted bool   `json:"deleted"`
			ID      string `json:"id"`
		}
		if err := json.Unmarshal([]byte(jsonOut), &parsed); err != nil {
			t.Fatalf("json parse: %v\nout=%q", err, jsonOut)
		}
		if parsed.Trashed || !parsed.Deleted || parsed.ID != "id1" {
			t.Fatalf("unexpected json output: %#v", parsed)
		}

		if deleteCount != 2 {
			t.Fatalf("expected 2 DELETE calls, got %d", deleteCount)
		}
	})
}
