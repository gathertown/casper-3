// Package config provides functions that allow to construct the service
// configuration from the environment.
package config

import (
	"os"
)

const (
	defaultEnv                 = "development"
	defaultLabelKey            = "doks.digitalocean.com/node-pool"
	defaultLabelValue          = "sfu"
	defaultProvider            = "digitalocean"
	defaultScanIntervalSeconds = "60"
	defaultToken               = "abcd123"
	defaultZone                = "k8s.gather.town"
	defaultSubdomain           = "" // effective only for DigitalOcean provider
)

// Config contains service information that can be changed from the
// environment.
type Config struct {
	Env                 string
	LabelKey            string
	LabelValue          string
	Provider            string
	ScanIntervalSeconds string
	Token               string
	Zone                string
	Subdomain           string
}

// FromEnv returns the service configuration from the environment variables.
// If an environment variable is not found, then a default value is provided.
func FromEnv() *Config {
	var (
		env                 = getenv("ENV", defaultEnv)
		scanIntervalSeconds = getenv("INTERVAL", defaultScanIntervalSeconds)
		labelKey            = getenv("LABEL_KEY", defaultLabelKey)
		labelValue          = getenv("LABEL_VALUE", defaultLabelValue)
		provider            = getenv("PROVIDER", defaultProvider)
		token               = getenv("TOKEN", defaultToken)
		subdomain           = getenv("SUBDOMAIN", defaultSubdomain)
		zone                = getenv("ZONE", defaultZone)
	)

	c := &Config{
		Env:                 env,
		ScanIntervalSeconds: scanIntervalSeconds,
		LabelKey:            labelKey,
		LabelValue:          labelValue,
		Provider:            provider,
		Token:               token,
		Subdomain:           subdomain,
		Zone:                zone,
	}
	return c
}

func getenv(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}
