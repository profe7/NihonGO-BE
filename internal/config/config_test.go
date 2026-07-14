package config

import "testing"

func TestDatabaseURL(t *testing.T) {
	cfg := Config{
		DBUser:     "u",
		DBPassword: "p",
		DBHost:     "h",
		DBPort:     "5432",
		DBName:     "db",
	}
	want := "postgres://u:p@h:5432/db?sslmode=disable"
	if got := cfg.DatabaseURL(); got != want {
		t.Errorf("DatabaseURL() = %q, want %q", got, want)
	}
}

func TestGetenv(t *testing.T) {
	t.Setenv("PRESENT_KEY", "value")

	cases := []struct {
		name     string
		key      string
		fallback string
		want     string
	}{
		{"uses env when set", "PRESENT_KEY", "fallback", "value"},
		{"uses fallback when unset", "MISSING_KEY", "fallback", "fallback"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := getenv(tc.key, tc.fallback); got != tc.want {
				t.Errorf("getenv(%q, %q) = %q, want %q", tc.key, tc.fallback, got, tc.want)
			}
		})
	}
}

func TestLoadReadsEnv(t *testing.T) {
	t.Setenv("PORT", "9999")
	t.Setenv("JWT_SECRET", "s3cr3t")

	cfg := Load()

	if cfg.Port != "9999" {
		t.Errorf("Port = %q, want 9999", cfg.Port)
	}
	if cfg.JWTSecret != "s3cr3t" {
		t.Errorf("JWTSecret = %q, want s3cr3t", cfg.JWTSecret)
	}
}
