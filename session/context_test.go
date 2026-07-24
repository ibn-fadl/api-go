package session

import (
	"context"
	"testing"
)

func TestContextWithID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{name: "valid", id: "s_789", wantErr: false},
		{name: "empty", id: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, err := ContextWithID(context.Background(), tt.id)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ContextWithID(%q) error = %v, wantErr %v", tt.id, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			got, err := IDFromContext(ctx)
			if err != nil {
				t.Fatalf("IDFromContext() unexpected error: %v", err)
			}
			if got != tt.id {
				t.Errorf("IDFromContext() = %q, want %q", got, tt.id)
			}
		})
	}
}

func TestIDFromContext_Absent(t *testing.T) {
	if _, err := IDFromContext(context.Background()); err == nil {
		t.Fatal("IDFromContext() on empty context: expected error, got nil")
	}
}
