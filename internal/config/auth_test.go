package config

import "testing"

func TestServerTokenPrecedence(t *testing.T) {
	t.Setenv("PLEXCTL_TOKEN", "environment")
	cfg := Config{Token: "account"}
	cfg.HomeUser.AccessToken = "home-user"
	if cfg.ServerToken() != "environment" {
		t.Fatal("environment token did not take precedence")
	}
	t.Setenv("PLEXCTL_TOKEN", "")
	if cfg.ServerToken() != "home-user" {
		t.Fatal("Home user token did not take precedence over account token")
	}
	cfg.HomeUser.AccessToken = ""
	if cfg.ServerToken() != "account" {
		t.Fatal("account token fallback failed")
	}
}
