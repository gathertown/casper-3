package config

import (
	"os"
	"reflect"
	"testing"
)

func setenv(t *testing.T, key, value string) {
	t.Helper()
	t.Logf("Setting env %q=%q", key, value)
	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("Failed setting env %q as %q: %v", key, value, err)
	}
}

func unsetenv(t *testing.T, key string) {
	t.Helper()
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("Failed unsetting env %q: %v", key, err)
	}
}

func TestFromEnv(t *testing.T) {
	setenv(t, "ENV", "development")
	setenv(t, "INTERVAL", "61")
	setenv(t, "PROVIDER", "digitalocean")
	setenv(t, "LABEL_KEY", "doks.digitalocean.com/node-pool")
	setenv(t, "LABEL_VALUES", "sfu")
	setenv(t, "TOKEN", "abcd1231")
	setenv(t, "SUBDOMAIN", "dev")
	setenv(t, "ZONE", "k8s.gather.town")
	setenv(t, "CLOUDFLARE_PROXIED_NODE_POOLS", "sfu, engine")

	cfg := FromEnv()

	if got, want := cfg.Env, "development"; got != want {
		t.Errorf("FromEnv() 'ENV' = %q; want %q", got, want)
	}

	if got, want := cfg.ScanIntervalSeconds, "61"; got != want {
		t.Errorf("FromEnv() 'INTERVAL' = %q; want %q", got, want)
	}

	if got, want := cfg.LabelKey, "doks.digitalocean.com/node-pool"; got != want {
		t.Errorf("FromEnv() 'LABEL_KEY' = %q; want %q", got, want)
	}

	if got, want := cfg.LabelValues, "sfu"; got != want {
		t.Errorf("FromEnv() 'LABEL_VALUES' = %q; want %q", got, want)
	}

	if got, want := cfg.Provider, "digitalocean"; got != want {
		t.Errorf("FromEnv() 'PROVIDER' = %q; want %q", got, want)
	}

	if got, want := cfg.Token, "abcd1231"; got != want {
		t.Errorf("FromEnv() 'TOKEN' = %q; want %q", got, want)
	}

	if got, want := cfg.Subdomain, "dev"; got != want {
		t.Errorf("FromEnv() 'SUBDOMAIN' = %q; want %q", got, want)
	}

	if got, want := cfg.Zone, "k8s.gather.town"; got != want {
		t.Errorf("FromEnv() 'ZONE' = %q; want %q", got, want)
	}
	if got, want := cfg.CloudflareProxiedNodePools, []string{"sfu", "engine"}; reflect.DeepEqual(want, got) == false {
		t.Errorf("FromEnv() 'CLOUDFLARE_PROXIED_NODE_POOLS' = %q; want %q", got, want)
	}

	unsetenv(t, "ENV")
	unsetenv(t, "INTERVAL")
	unsetenv(t, "PROVIDER")
	unsetenv(t, "LABEL_KEY")
	unsetenv(t, "LABEL_VALUES")
	unsetenv(t, "TOKEN")
	unsetenv(t, "SUBDOMAIN")
	unsetenv(t, "ZONE")
}

func TestSplitAndRejoin(t *testing.T) {
	type test struct {
		input string
		sep   string
		want  string
	}

	tests := []test{
		{input: "sfu,gameserver", sep: ",", want: "sfu, gameserver"},
		{input: "sfu, gameserver", sep: ",", want: "sfu, gameserver"},
		{input: "sfu, ,gameserver", sep: ",", want: "sfu, gameserver"},
		{input: "sfu,,,gameserver", sep: ",", want: "sfu, gameserver"},
		{input: "sfu,gameserver  ", sep: ",", want: "sfu, gameserver"},
		{input: "  sfu  ,  gameserver  ", sep: ",", want: "sfu, gameserver"},
		{input: " sfu,", sep: ",", want: "sfu"},
		{input: "sfu", sep: ",", want: "sfu"},
		{input: " sfu ", sep: ",", want: "sfu"},
		{input: "", sep: ",", want: ""},
		{input: ",", sep: ",", want: ""},
	}

	for _, tc := range tests {
		got := splitAndRejoin(tc.input, tc.sep)
		if !reflect.DeepEqual(tc.want, got) {
			t.Fatalf("expected: %v, got: %v", tc.want, got)
		}
	}
}

func TestStringToList(t *testing.T) {
	type test struct {
		input string
		sep   string
		want  []string
	}

	tests := []test{
		{input: "sfu,engine", sep: ",", want: []string{"sfu", "engine"}},
		{input: "sfu, engine", sep: ",", want: []string{"sfu", "engine"}},
		{input: "sfu, ,engine", sep: ",", want: []string{"sfu", "engine"}},
		{input: "sfu,,,engine", sep: ",", want: []string{"sfu", "engine"}},
		{input: "sfu,engine  ", sep: ",", want: []string{"sfu", "engine"}},
		{input: "  sfu  ,  engine  ", sep: ",", want: []string{"sfu", "engine"}},
		{input: " sfu,", sep: ",", want: []string{"sfu"}},
		{input: "sfu", sep: ",", want: []string{"sfu"}},
		{input: " sfu ", sep: ",", want: []string{"sfu"}},
		{input: "", sep: ",", want: []string{}},
		{input: ",", sep: ",", want: []string{}},
	}

	for _, tc := range tests {
		got := stringToList(tc.input)
		if !reflect.DeepEqual(tc.want, got) {
			t.Fatalf("expected: %v, got: %v", tc.want, got)
		}
	}
}
