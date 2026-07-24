package time

import (
	nethttp "net/http"
	"net/http/httptest"
	"testing"

	_ "time/tzdata"
)

func TestHTTPWithZone(t *testing.T) {
	tests := []struct {
		name       string
		header     string
		setHeader  bool
		wantStatus int
		wantNext   bool
	}{
		{name: "valid", header: "Asia/Jakarta", setHeader: true, wantStatus: nethttp.StatusOK, wantNext: true},
		{name: "absent", setHeader: false, wantStatus: nethttp.StatusBadRequest, wantNext: false},
		{name: "invalid", header: "Mars/Olympus", setHeader: true, wantStatus: nethttp.StatusBadRequest, wantNext: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				nextCalled bool
				gotZone    string
			)
			next := nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
				nextCalled = true
				gotZone, _ = ZoneFromContext(r.Context())
				w.WriteHeader(nethttp.StatusOK)
			})

			req := httptest.NewRequest(nethttp.MethodGet, "/", nil)
			if tt.setHeader {
				req.Header.Set("time-zone", tt.header)
			}
			rec := httptest.NewRecorder()

			HTTPWithZone(next).ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if nextCalled != tt.wantNext {
				t.Errorf("next called = %v, want %v", nextCalled, tt.wantNext)
			}
			if tt.wantNext && gotZone != tt.header {
				t.Errorf("context zone = %q, want %q", gotZone, tt.header)
			}
			if !tt.wantNext && rec.Body.Len() == 0 {
				t.Error("expected a problem body on rejection, got empty")
			}
		})
	}
}
