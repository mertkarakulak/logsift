package normalize

import "testing"

func TestMessage(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "UUID replaced before numbers",
			input: "document 550e8400-e29b-41d4-a716-446655440000 not found",
			want:  "document <UUID> not found",
		},
		{
			name:  "IPv4 with port",
			input: "connect() failed to 192.168.1.100:8080",
			want:  "connect() failed to <IP>",
		},
		{
			name:  "IPv4 without port",
			input: "client: 10.0.0.5, server: example.com",
			want:  "client: <IP>, server: example.com",
		},
		{
			// "256" in "sha256" is not preceded by a word boundary so the bare-number
			// replacement does not match it; only the hex hash is replaced.
			name:  "long hex hash",
			input: "sha256: deadbeefcafe1234abcdef0123456789",
			want:  "sha256: <HEX>",
		},
		{
			name:  "bare numbers",
			input: "worker process 1234 exited with code 1",
			want:  "worker process <NUM> exited with code <NUM>",
		},
		{
			name:  "UUID not shredded by number replacement",
			input: "request 3f8a1c2d-4e5b-6a7b-8c9d-0e1f2a3b4c5d failed",
			want:  "request <UUID> failed",
		},
		{
			name:  "IPv6 address",
			input: "connect from ::1 refused",
			want:  "connect from <IPv6> refused",
		},
		{
			name:  "multiple IPs on same line",
			input: "request from 1.2.3.4 proxied through 5.6.7.8",
			want:  "request from <IP> proxied through <IP>",
		},
		{
			name:  "no dynamic content unchanged",
			input: "signal process started",
			want:  "signal process started",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Message(tt.input)
			if got != tt.want {
				t.Errorf("Message(%q)\n  got  %q\n  want %q", tt.input, got, tt.want)
			}
		})
	}
}
