package nix

import "testing"

func TestGetStorePathName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		out  string
	}{
		{
			name: "full nix store path",
			in:   "/nix/store/2467crcbg119q5jb5nwqxm0c87ls3wnv-source-1.2.3",
			out:  "source-1.2.3",
		},
		{
			name: "without /nix/store prefix",
			in:   "2467crcbg119q5jb5nwqxm0c87ls3wnv-source-1.2.3",
			out:  "source-1.2.3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, err := GetStorePathName(tt.in)
			if err != nil {
				t.Fatal(err)
			}

			if name != tt.out {
				t.Errorf("unexpected name: \"%s\" != \"%s\"", name, tt.out)
			}
		})
	}
}
