package main

import (
	"fmt"
	"os"
	"strconv"

	"time"

	"github.com/gathertown/casper-3/internal/config"
	common "github.com/gathertown/casper-3/pkg"
	"github.com/gathertown/casper-3/pkg/kubernetes"
	"github.com/gathertown/casper-3/pkg/log"
	cloudflare "github.com/gathertown/casper-3/pkg/providers/cloudflare"
	digitalocean "github.com/gathertown/casper-3/pkg/providers/digitalocean"
)

type Node = common.Node
type provider interface {
	Sync(nodes []Node) (bool, error)
}

// run labels nodes if label is missing
func main() {
	// Generic configuration setup
	cfg := config.FromEnv()
	logger := log.New(os.Stdout, cfg.Env)
	interval, err := strconv.ParseInt(cfg.ScanIntervalSeconds, 10, 64)
	if err != nil {
		msg := fmt.Sprintf("%v", err)
		logger.Info(msg)
		return
	}
	logger.Info("Launching casper-3", "labelKey", cfg.LabelKey, "labelValues", cfg.LabelValues, "interval", cfg.ScanIntervalSeconds, "environment", cfg.Env, "TXT identifier", fmt.Sprintf("heritage=casper-3,environment=%s", cfg.Env))

	// Run loop based on interval. Check if there are unlabelled instances.
	// If there are unlabelled instances, add label. If not, skip.
	var p provider
	if cfg.Provider == "digitalocean" {
		p = digitalocean.DigitalOceanDNS{}
	}
	if cfg.Provider == "cloudflare" {
		p = cloudflare.CloudFlareDNS{}
	}
	for {
		c, err := kubernetes.New()
		if err != nil {
			msg := fmt.Sprintf("%v", err)
			logger.Info(msg)
			return
		}

		n, err := c.Nodes()
		if err != nil {
			fmt.Println(err)
		}

		_, err = p.Sync(n)
		if err != nil {
			msg := fmt.Sprintf("%v", err)
			logger.Info(msg)
		}

		time.Sleep(time.Duration(interval) * time.Second)
	}
}
