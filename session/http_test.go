package session

import (
	nethttp "net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPWithID(t *testing.T) {
	tests := []struct {
		name       string
		header     string
		wantStatus int
		wantNext   bool
	}{
		{name: "present", header: "s_789", wantStatus: nethttp.StatusOK, wantNext: true},
		{name: "absent", header: "", wantStatus: nethttp.StatusUnauthorized, wantNext: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				nextCalled bool
				gotID      string
			)
			next := nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
				nextCalled = true
				gotID, _ = IDFromContext(r.Context())
				w.WriteHeader(nethttp.StatusOK)
			})

			req := httptest.NewRequest(nethttp.MethodGet, "/", nil)
			if tt.header != "" {
				req.Header.Set("session-id", tt.header)
			}
			rec := httptest.NewRecorder()

			HTTPWithID(next).ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if nextCalled != tt.wantNext {
				t.Errorf("next called = %v, want %v", nextCalled, tt.wantNext)
			}
			if tt.wantNext && gotID != tt.header {
				t.Errorf("context id = %q, want %q", gotID, tt.header)
			}
			if !tt.wantNext && rec.Body.Len() == 0 {
				t.Error("expected a problem body on rejection, got empty")
			}
		})
	}
}
