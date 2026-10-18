package main

import "testing"

func TestIdentityFromEnv(t *testing.T) {
	tests := []struct {
		name      string
		env       map[string]string
		want      identity
		wantError bool
	}{
		{"nothing set: the image's own user", nil, identity{65532, 65532}, false},
		{"PUID alone gives the group the same number", map[string]string{"PUID": "1000"}, identity{1000, 1000}, false},
		{"both", map[string]string{"PUID": "1000", "PGID": "100"}, identity{1000, 100}, false},
		{"PGID alone", map[string]string{"PGID": "100"}, identity{65532, 100}, false},
		{"root is refused as user", map[string]string{"PUID": "0"}, identity{}, true},
		{"root is refused as group", map[string]string{"PUID": "1000", "PGID": "0"}, identity{}, true},
		{"text is refused", map[string]string{"PUID": "alice"}, identity{}, true},
		{"negative is refused", map[string]string{"PGID": "-5"}, identity{}, true},
		{"too large is refused", map[string]string{"PUID": "99999999999"}, identity{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := identityFromEnv(func(k string) string { return tt.env[k] })
			if (err != nil) != tt.wantError {
				t.Fatalf("error = %v, want error: %v", err, tt.wantError)
			}
			if err == nil && got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}
