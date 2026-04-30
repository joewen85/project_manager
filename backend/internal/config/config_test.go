package config

import "testing"

func TestConfigValidateRejectsInsecureJWTSecret(t *testing.T) {
	cases := []string{
		"",
		"   ",
		"change-me",
		"change-me-in-production",
		"CHANGE-ME-IN-PRODUCTION",
	}

	for _, secret := range cases {
		cfg := Config{JWTSecret: secret}
		if err := cfg.Validate(); err == nil {
			t.Fatalf("expected JWT secret %q to be rejected", secret)
		}
	}
}

func TestConfigValidateAcceptsStrongJWTSecret(t *testing.T) {
	cfg := Config{JWTSecret: "x3i6m9Q2!u4w7#z0K1@p8n5t"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected JWT secret to pass validation: %v", err)
	}
}
