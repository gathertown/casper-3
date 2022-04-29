// Package config provides functions that allow to construct the service
// configuration from the environment.
package config

import (
	"os"
	"strings"
)

const (
	defaultEnv                 = "development"
	defaultLabelKey            = "doks.digitalocean.com/node-pool"
	defaultLabelValues         = "sfu"
	defaultProvider            = "digitalocean"
	defaultScanIntervalSeconds = "60"
	defaultToken               = "abcd123"
	defaultZone                = "k8s.gather.town"
	defaultSubdomain           = ""     // effective only for DigitalOcean provider
	defaultLogLevel            = "info" // use to "debug" for debug level, everything else is INFO
	defaultAllowSyncPods       = "false"
	defaultSyncPodLabelKey     = "casper-3.gather.town/sync"
	defaultSyncPodLabelValue   = "true"
	defaultCloudFlareProxied   = "false"
)

// Config contains service information that can be changed from the
// environment.
type Config struct {
	Env                 string
	LabelKey            string
	LabelValues         string
	Provider            string
	ScanIntervalSeconds string
	Token               string
	Zone                string
	Subdomain           string
	LogLevel            string
	AllowSyncPods       string
	SyncPodLabelKey     string
	SyncPodLabelValue   string
	CloudFlareProxied   string
}

// FromEnv returns the service configuration from the environment variables.
// If an environment variable is not found, then a default value is provided.
func FromEnv() *Config {
	var (
		env                 = getenv("ENV", defaultEnv)
		scanIntervalSeconds = getenv("INTERVAL", defaultScanIntervalSeconds)
		labelKey            = getenv("LABEL_KEY", defaultLabelKey)
		labelValues         = getenv("LABEL_VALUES", defaultLabelValues)
		provider            = getenv("PROVIDER", defaultProvider)
		token               = getenv("TOKEN", defaultToken)
		subdomain           = getenv("SUBDOMAIN", defaultSubdomain)
		zone                = getenv("ZONE", defaultZone)
		logLevel            = getenv("LOGLEVEL", defaultLogLevel)
		allowSyncPods       = getenv("ALLOW_SYNC_PODS", defaultAllowSyncPods)
		syncPodLabelKey     = getenv("SYNC_POD_LABEL_KEY", defaultSyncPodLabelKey)
		syncPodLabelValue   = getenv("SYNC_POD_LABEL_VALUE", defaultSyncPodLabelValue)
		cloudFlareProxied   = getenv("CLOUDFLARE_PROXIED", defaultCloudFlareProxied)
	)

	c := &Config{
		Env:                 env,
		ScanIntervalSeconds: scanIntervalSeconds,
		LabelKey:            labelKey,
		LabelValues:         splitAndRejoin(labelValues, ","),
		Provider:            provider,
		Token:               token,
		Subdomain:           subdomain,
		Zone:                zone,
		LogLevel:            logLevel,
		AllowSyncPods:       allowSyncPods,
		SyncPodLabelKey:     syncPodLabelKey,
		SyncPodLabelValue:   syncPodLabelValue,
		CloudFlareProxied:   cloudFlareProxied,
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

// splitAndRejoin splits a string with a given seperator
// and then join it back, with all the extraneous space and
// trailing seperators removed
func splitAndRejoin(str string, sep string) string {
	parsed := []string{}
	for _, v := range strings.Split(str, sep) {
		val := strings.TrimSpace(v)
		if val == "" {
			continue
		}
		parsed = append(parsed, val)
	}
	return strings.Join(parsed, ", ")
}
