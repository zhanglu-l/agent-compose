package adapters

import (
	"testing"

	driverpkg "agent-compose/pkg/driver"
	domain "agent-compose/pkg/model"
)

func TestDomainStreamFromDriver(t *testing.T) {
	tests := []struct {
		name   string
		stream driverpkg.StdioStream
		want   domain.StdioStream
	}{
		{name: "zero", want: domain.StdioStdout},
		{name: "stdout", stream: driverpkg.StdioStdout, want: domain.StdioStdout},
		{name: "stderr", stream: driverpkg.StdioStderr, want: domain.StdioStderr},
		{name: "unknown", stream: driverpkg.StdioStream("future"), want: domain.StdioStdout},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := domainStreamFromDriver(tt.stream); got != tt.want {
				t.Fatalf("domainStreamFromDriver(%q) = %q, want %q", tt.stream, got, tt.want)
			}
		})
	}
}
