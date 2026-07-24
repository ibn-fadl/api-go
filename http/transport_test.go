package http

import (
	"context"
	nethttp "net/http"
	"testing"
)

// testKey is an unexported, package-local context key, mirroring how the real
// domain packages define theirs — and demonstrating that WithHeader works with
// any comparable key without needing to name its type.
type testKey struct{}

type stubRoundTripper struct {
	got  *nethttp.Request
	resp *nethttp.Response
}

func (s *stubRoundTripper) RoundTrip(r *nethttp.Request) (*nethttp.Response, error) {
	s.got = r
	return s.resp, nil
}

func TestWithHeader(t *testing.T) {
	tests := []struct {
		name    string
		ctx     context.Context
		wantVal string // "" means the header must be left unset
	}{
		{name: "present", ctx: context.WithValue(context.Background(), testKey{}, "u_123"), wantVal: "u_123"},
		{name: "absent", ctx: context.Background(), wantVal: ""},
		{name: "empty string", ctx: context.WithValue(context.Background(), testKey{}, ""), wantVal: ""},
		{name: "non-string", ctx: context.WithValue(context.Background(), testKey{}, 42), wantVal: ""},
	}
	p := WithHeader(testKey{}, "user-id")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := nethttp.Header{}
			p(tt.ctx, h)
			if got := h.Get("user-id"); got != tt.wantVal {
				t.Errorf("header = %q, want %q", got, tt.wantVal)
			}
		})
	}
}

func TestTransport_RoundTrip(t *testing.T) {
	stub := &stubRoundTripper{resp: &nethttp.Response{StatusCode: nethttp.StatusOK}}
	tr := &Transport{
		Base:        stub,
		Propagators: []Propagator{WithHeader(testKey{}, "user-id")},
	}

	ctx := context.WithValue(context.Background(), testKey{}, "u_123")
	req, err := nethttp.NewRequestWithContext(ctx, nethttp.MethodGet, "https://example.test/", nil)
	if err != nil {
		t.Fatalf("NewRequestWithContext: %v", err)
	}

	if _, err := tr.RoundTrip(req); err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}

	// The RoundTripper contract forbids mutating the original request.
	if got := req.Header.Get("user-id"); got != "" {
		t.Errorf("original request was mutated: user-id = %q, want unset", got)
	}
	// The base tripper must receive a clone carrying the propagated header.
	if stub.got == nil {
		t.Fatal("base round tripper was not invoked")
	}
	if stub.got == req {
		t.Error("base received the original request, want a clone")
	}
	if got := stub.got.Header.Get("user-id"); got != "u_123" {
		t.Errorf("propagated header = %q, want u_123", got)
	}
}

func TestTransport_BaseDefaultsToDefaultTransport(t *testing.T) {
	tr := &Transport{}
	if tr.base() != nethttp.DefaultTransport {
		t.Error("base() with nil Base should return http.DefaultTransport")
	}
}
