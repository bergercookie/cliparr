package main

import "testing"

func TestIdentityFromEnv(t *testing.T) {
	tests := []struct {
		name      string
		env       map[string]string
		want      identity
		wantSet   bool
		wantError bool
	}{
		{"nothing set: keep the user Cliparr was started as", nil, identity{}, false, false},
		{"PUID alone gives the group the same number", map[string]string{"PUID": "1000"}, identity{1000, 1000}, true, false},
		{"both", map[string]string{"PUID": "1000", "PGID": "100"}, identity{1000, 100}, true, false},
		{"PGID alone: the image's own user", map[string]string{"PGID": "100"}, identity{65532, 100}, true, false},
		{"root is refused as user", map[string]string{"PUID": "0"}, identity{}, false, true},
		{"root is refused as group", map[string]string{"PUID": "1000", "PGID": "0"}, identity{}, false, true},
		{"text is refused", map[string]string{"PUID": "alice"}, identity{}, false, true},
		{"negative is refused", map[string]string{"PGID": "-5"}, identity{}, false, true},
		{"too large is refused", map[string]string{"PUID": "99999999999"}, identity{}, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, set, err := identityFromEnv(func(k string) string { return tt.env[k] })
			if (err != nil) != tt.wantError {
				t.Fatalf("error = %v, want error: %v", err, tt.wantError)
			}
			if err == nil && (got != tt.want || set != tt.wantSet) {
				t.Errorf("got %+v set=%v, want %+v set=%v", got, set, tt.want, tt.wantSet)
			}
		})
	}
}
