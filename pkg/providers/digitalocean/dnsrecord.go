package digitalocean

import (
	"context"
	"fmt"
	"os"

	"github.com/digitalocean/godo"
	"github.com/gathertown/casper-3/internal/config"
	common "github.com/gathertown/casper-3/pkg"
	"github.com/gathertown/casper-3/pkg/log"
)

var cfg = config.FromEnv()
var logger = log.New(os.Stdout, cfg.Env)

type Node = common.Node
type DigitalOceanDNS struct{}

func NewDOClient() *godo.Client {
	return godo.NewFromToken(cfg.Token)
}

func (d DigitalOceanDNS) Sync(nodes []Node) (bool, error) {
	var records []godo.DomainRecord
	var nodeHostnames, dnsRecords []string

	// Setup the client
	client := NewDOClient()

	// The source of truth are the TXT records as they are created and deleted alongside 'A' records.
	recordType := "TXT"

	// Fetch all TXT DNS
	txtRecords, err := getRecords(context.TODO(), client, cfg.Zone, recordType)
	if err != nil {
		logger.Info("Error occured while fetching records", "provider", cfg.Provider, "zone", cfg.Zone, "host", cfg.Subdomain)
		return false, err
	}

	// Generate arrays
	for _, record := range txtRecords {
		if record.Data == fmt.Sprintf("heritage=casper-3,environment=%s", cfg.Env) {
			records = append(records, record)
			dnsRecords = append(dnsRecords, record.Name)
		}
	}

	for _, node := range nodes {
		nodeHostnames = append(nodeHostnames, node.Name)
	}

	// Find new entries
	addEntries := compare(nodeHostnames, dnsRecords)
	if len(addEntries) > 0 {
		for _, name := range addEntries {
			addressIPv4 := ""
			// this loop seems a bit inefficient at first glance
			// entries are bellow 1k, so shouldn't really matter.
			for _, entry := range nodes {
				if entry.Name == name {
					addressIPv4 = entry.ExternalIP
				}
			}
			// Does this check make sense?
			if addressIPv4 == "" {
				logger.Info("IP address not found for entry", "name", name, "zone", cfg.Zone, "subdomain", cfg.Subdomain)
			} else {
				_, err := addRecord(context.TODO(), client, cfg.Zone, name, cfg.Subdomain, addressIPv4)
				if err != nil {
					return false, err
				}
			}
		}
	}

	// Remove stale entries
	deleteEntries := compare(dnsRecords, nodeHostnames)
	if len(deleteEntries) > 0 {
		for _, name := range deleteEntries {
			_, err := deleteRecord(context.TODO(), client, cfg.Zone, name)
			if err != nil {
				return false, err
			}
		}
	}

	// Find kubernetes nodes to register
	return true, nil
}

func getRecords(ctx context.Context, client *godo.Client, domain string, recordType string) ([]godo.DomainRecord, error) {
	opt := &godo.ListOptions{
		Page:    1,
		PerPage: 1000,
	}
	records, _, err := client.Domains.RecordsByType(ctx, domain, recordType, opt)

	if err != nil {
		return nil, err
	}
	logger.Debug("Fetched DNS records", "type", recordType, "records", records)
	return records, err
}

func deleteRecord(ctx context.Context, client *godo.Client, zone string, name string) (bool, error) {
	opt := &godo.ListOptions{
		Page:    1,
		PerPage: 1000,
	}
	txtRecords, _, err := client.Domains.RecordsByTypeAndName(ctx, zone, "TXT", name, opt)
	if err != nil {
		return false, err
	}

	aRecords, _, err := client.Domains.RecordsByTypeAndName(ctx, zone, "A", name, opt)
	if err != nil {
		return false, err
	}

	// merge slices
	records := append(txtRecords, aRecords...)

	for _, record := range records {
		response, err := client.Domains.DeleteRecord(ctx, zone, record.ID)
		if err != nil {
			return false, err
		}
		logger.Info("Deleted DNS record", "zone", zone, "record", record.Name, "type", record.Type, "response", response)
	}
	return true, nil
}

func addRecord(ctx context.Context, client *godo.Client, zone string, name string, sub string, addressIPv4 string) (bool, error) {
	aRecordRequest := &godo.DomainRecordEditRequest{
		Type: "A",
		Name: fmt.Sprintf("%s.%s", name, sub), // Workaround for subdomains to work properly on digital ocean.
		Data: addressIPv4,
		TTL:  1800,
	}

	txtRecordRequest := &godo.DomainRecordEditRequest{
		Type: "TXT",
		Name: fmt.Sprintf("%s.%s", name, sub),
		Data: addressIPv4,
		TTL:  1800,
	}

	aRecord, aRecordResponse, err := client.Domains.CreateRecord(ctx, zone, aRecordRequest)
	if err != nil {
		return false, err
	}
	logger.Info("Added record", "zone", zone, "name", name, "type", "A", "response", aRecordResponse, "record", aRecord)
	txtRecord, txtRecordResponse, err := client.Domains.CreateRecord(ctx, zone, txtRecordRequest)
	if err != nil {
		return false, err
	}
	logger.Info("Added DNS record", "zone", zone, "name", name, "type", "TXT", "response", txtRecordResponse, "record", txtRecord)

	return true, err
}

// Compare slices: https://stackoverflow.com/a/45428032/577133
// Returns []string of elements found in 'a' but not in 'b'.
func compare(a, b []string) []string {
	mb := make(map[string]struct{}, len(b))

	for _, x := range b {
		mb[x] = struct{}{}
	}

	var diff []string

	for _, x := range a {
		if _, found := mb[x]; !found {
			diff = append(diff, x)
		}
	}

	return diff
}
