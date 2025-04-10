package exec

import "testing"

func TestParseTarget(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		outUser string
		outHost string
		outPort string
	}{
		{
			name:    "without user or port",
			in:      "host",
			outUser: "",
			outHost: "host",
			outPort: "",
		},
		{
			name:    "with user but not port",
			in:      "user@host",
			outUser: "user",
			outHost: "host",
			outPort: "",
		},
		{
			name:    "without user but with port",
			in:      "host:222",
			outUser: "",
			outHost: "host",
			outPort: "222",
		},
		{
			name:    "with everything",
			in:      "user@host:222",
			outUser: "user",
			outHost: "host",
			outPort: "222",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, host, port := parseTarget(tt.in)

			if user != tt.outUser {
				t.Errorf("unexpected user: \"%s\" != \"%s\"", user, tt.outUser)
			}
			if host != tt.outHost {
				t.Errorf("unexpected host: \"%s\" != \"%s\"", host, tt.outHost)
			}
			if port != tt.outPort {
				t.Errorf("unexpected port: \"%s\" != \"%s\"", port, tt.outPort)
			}
		})
	}
}
