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

func TestResourceTokensStayWithTheirServerAndAccount(t *testing.T) {
	t.Setenv("PLEXCTL_TOKEN", "")
	cfg := Config{Token: "account", DefaultServer: "mine", Servers: map[string]Server{"mine": {URL: "https://mine.invalid"}}}
	source := cfg.TokenSource()
	cfg.Servers["friend"] = Server{Name: "Friend", AccessToken: "shared-token", TokenSource: source, Shared: true}
	cfg.HomeUser.AccessToken = "old-server-token"
	if err := cfg.SetDefaultServer("friend"); err != nil {
		t.Fatal(err)
	}
	if cfg.ServerToken() != "shared-token" || cfg.AccountToken() != "account" || cfg.HomeUser.AccessToken != "" {
		t.Fatal("switching servers reused credentials or changed the account token")
	}
	namespace := cfg.MediaCacheNamespace()
	cfg.HomeUser.AuthToken = "different-profile"
	if cfg.HasServerToken() || cfg.ServerToken() != "different-profile" || cfg.MediaCacheNamespace() == namespace {
		t.Fatal("a different Home profile reused the previous user's access or cache")
	}
	t.Setenv("PLEXCTL_TOKEN", "account")
	if cfg.ServerToken() != "shared-token" {
		t.Fatal("resource token for the environment account was not selected")
	}
	t.Setenv("PLEXCTL_TOKEN", "different-account")
	if cfg.ServerToken() != "different-account" {
		t.Fatal("resource token escaped its account scope")
	}
}
