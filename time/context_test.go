package time

import (
	"context"
	"testing"

	// tzdata is embedded so IANA zone lookups are hermetic: these tests pass
	// on minimal images (scratch/distroless CI) that ship no system zoneinfo.
	_ "time/tzdata"
)

func TestContextWithZone(t *testing.T) {
	tests := []struct {
		name    string
		tz      string
		wantErr bool
	}{
		{name: "iana zone", tz: "Asia/Jakarta", wantErr: false},
		{name: "utc", tz: "UTC", wantErr: false},
		{name: "empty", tz: "", wantErr: true},
		{name: "invalid", tz: "Mars/Olympus", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, err := ContextWithZone(context.Background(), tt.tz)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ContextWithZone(%q) error = %v, wantErr %v", tt.tz, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			got, err := ZoneFromContext(ctx)
			if err != nil {
				t.Fatalf("ZoneFromContext() unexpected error: %v", err)
			}
			if got != tt.tz {
				t.Errorf("ZoneFromContext() = %q, want %q", got, tt.tz)
			}
		})
	}
}

func TestZoneFromContext_Absent(t *testing.T) {
	if _, err := ZoneFromContext(context.Background()); err == nil {
		t.Fatal("ZoneFromContext() on empty context: expected error, got nil")
	}
}
