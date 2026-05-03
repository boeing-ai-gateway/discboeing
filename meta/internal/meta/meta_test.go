package meta

import (
	"testing"
	"time"

	"github.com/obot-platform/discobot/meta/internal/config"
)

func TestJWTRotationCheckInterval(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.Config
		want time.Duration
	}{
		{
			name: "caps long intervals at one hour",
			cfg: &config.Config{JWTSigning: config.JWTSigningConfig{
				RotationInterval: 72 * time.Hour,
				PrepublishWindow: 24 * time.Hour,
			}},
			want: time.Hour,
		},
		{
			name: "uses prepublish quarter when shorter",
			cfg: &config.Config{JWTSigning: config.JWTSigningConfig{
				RotationInterval: 24 * time.Hour,
				PrepublishWindow: 2 * time.Hour,
			}},
			want: 30 * time.Minute,
		},
		{
			name: "floors short intervals at one minute",
			cfg: &config.Config{JWTSigning: config.JWTSigningConfig{
				RotationInterval: time.Minute,
				PrepublishWindow: time.Minute,
			}},
			want: time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := jwtRotationCheckInterval(tt.cfg); got != tt.want {
				t.Fatalf("expected %s, got %s", tt.want, got)
			}
		})
	}
}
