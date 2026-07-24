package http

import (
	"encoding/json"
	nethttp "net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteJSON(rec, nethttp.StatusCreated, map[string]any{"ok": true})

	if rec.Code != nethttp.StatusCreated {
		t.Errorf("status = %d, want %d", rec.Code, nethttp.StatusCreated)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type = %q, want application/json", ct)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	if body["ok"] != true {
		t.Errorf("body[ok] = %v, want true", body["ok"])
	}
}

func TestWriteJSON_NilPayload(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteJSON(rec, nethttp.StatusNoContent, nil)

	if rec.Code != nethttp.StatusNoContent {
		t.Errorf("status = %d, want %d", rec.Code, nethttp.StatusNoContent)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("expected empty body for nil payload, got %q", rec.Body.String())
	}
}

func TestWriteProblem(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteProblem(rec, nethttp.StatusBadRequest, map[string]any{"detail": "boom"})

	if rec.Code != nethttp.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, nethttp.StatusBadRequest)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type = %q, want application/json", ct)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	if body["detail"] != "boom" {
		t.Errorf("body[detail] = %v, want boom", body["detail"])
	}
}
