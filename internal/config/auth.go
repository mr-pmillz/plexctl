package config

import "os"

// ServerToken selects the same credentials for metadata, artwork, and streams.
func (c *Config) ServerToken() string {
	if token := os.Getenv("PLEXCTL_TOKEN"); token != "" {
		return token
	}
	if c.HomeUser.AccessToken != "" {
		return c.HomeUser.AccessToken
	}
	return c.Token
}
