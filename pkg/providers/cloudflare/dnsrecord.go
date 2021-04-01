package cloudflare

import (
	"context"
	"fmt"
	"os"
	"strings"

	cloudflare "github.com/cloudflare/cloudflare-go"
	"github.com/gathertown/casper-3/internal/config"
	common "github.com/gathertown/casper-3/pkg"
	"github.com/gathertown/casper-3/pkg/log"
)

var cfg = config.FromEnv()
var logger = log.New(os.Stdout, cfg.Env)
var label = fmt.Sprintf("heritage=casper-3,environment=%s", cfg.Env)

type Node = common.Node
type CloudFlareDNS struct{}

func NewCFClient() *cloudflare.API {
	api, err := cloudflare.NewWithAPIToken(cfg.Token)
	if err != nil {
		fmt.Println(err)
	}
	return api
}

func (d CloudFlareDNS) Sync(nodes []Node) (bool, error) {
	var nodeHostnames, dnsRecords []string

	// Setup the client
	client := NewCFClient()

	// The source of truth are the TXT records as they are created and deleted alongside 'A' records.
	recordType := "TXT"

	// Fetch all TXT DNS
	txtRecords, err := getRecords(context.TODO(), client, cfg.Zone, recordType)
	if err != nil {
		logger.Info("Error occured while fetching records", "provider", cfg.Provider, "zone", cfg.Zone)
		return false, err
	}

	// Generate arrays
	for _, record := range txtRecords {
		if record.Data == label {
			cName := strings.Split(record.Name, ".") // e.g. convert "sfu-v81hha.dev" to "sfu-v81hha" to allow comparison with hostnames
			dnsRecords = append(dnsRecords, cName[0])
		}
	}
	logger.Debug("DNS Records Found", "records", dnsRecords)

	for _, node := range nodes {
		nodeHostnames = append(nodeHostnames, node.Name)
	}
	logger.Debug("SFU nodes found", "nodes", nodeHostnames)

	// Find new entries
	addEntries := common.Compare(nodeHostnames, dnsRecords)
	logger.Info("Entries to be added", "entries", addEntries)
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
				logger.Info("IP address not found for entry", "name", name, "zone", cfg.Zone)
			} else {
				_, err := addRecord(context.TODO(), client, cfg.Zone, name, addressIPv4, cfg.Env)
				if err != nil {
					return false, err
				}
			}
		}
	}

	// Remove stale entries
	deleteEntries := compare(dnsRecords, nodeHostnames)
	logger.Info("Entries to be deleted", "entries", deleteEntries)
	if len(deleteEntries) > 0 {
		for _, name := range deleteEntries {
			// The 'Name' entry is the FQDN
			cName := fmt.Sprintf("%s.%s.%s", name, cfg.Subdomain, cfg.Zone)
			logger.Debug("Launching deletion", "record", cName)
			_, err := deleteRecord(context.TODO(), client, cfg.Zone, cName)
			if err != nil {
				return false, err
			}
		}
	}

	// Find kubernetes nodes to register
	return true, nil
}

func getRecords(ctx context.Context, client *cloudflare.API, zone string, recordType string) ([]cloudflare.DNSRecord, error) {

	// Get ZoneID
	zoneID, err := client.ZoneIDByName(zone)
	if err != nil {
		return nil, err
	}

	// Filtering by content doesn't work unfortunately,
	// see https://github.com/cloudflare/cloudflare-go/issues/613
	record := cloudflare.DNSRecord{Type: recordType}
	records, err := client.DNSRecords(ctx, zoneID, record)
	if err != nil {
		return nil, err
	}

	logger.Debug("Fetched DNS records", "type", recordType, "records", records)
	return records, err
}

func deleteRecord(ctx context.Context, client *cloudflare.API, zone string, name string) (bool, error) {
	// Get ZoneID
	zoneID, err := client.ZoneIDByName(zone)
	if err != nil {
		return false, err
	}

	fqdn := fmt.Sprintf("%s.%s", name, zone)
	txtRecord := cloudflare.DNSRecord{Name: fqdn, Type: "TXT"}
	txtRecords, err := client.DNSRecords(ctx, zoneID, txtRecord)
	if err != nil {
		return false, err
	}
	logger.Debug("TXT record to be deleted", "records", txtRecords)

	aRecord := cloudflare.DNSRecord{Name: fqdn, Type: "A"}
	aRecords, err := client.DNSRecords(ctx, zoneID, aRecord)
	if err != nil {
		return false, err
	}
	logger.Debug("A record to be deleted", "records", aRecords)

	// merge slices
	records := append(txtRecords, aRecords...)
	logger.Debug("Records to be deleted", "records", records)

	for _, record := range records {
		logger.Debug("Deleting record", "record", record)
		// response := client.DeleteDNSRecord(ctx, zoneID, record.ID)
		// if err != nil {
		// 	return false, err
		// }
		response := "records not actually deleted!"
		logger.Info("Deleted DNS record", "zone", zone, "record", record.Name, "type", record.Type, "response", response)
	}
	return true, nil
}

func addRecord(ctx context.Context, client *cloudflare.API, zone string, name string, addressIPv4 string, env string) (bool, error) {

	// Get ZoneID
	zoneID, err := client.ZoneIDByName(zone)
	if err != nil {
		return false, err
	}

	aRecordRequest := cloudflare.DNSRecord{
		Type: "A",
		Name: name,
		Data: addressIPv4,
		TTL:  1800,
	}

	txtRecordRequest := cloudflare.DNSRecord{
		Type: "TXT",
		Name: name,
		Data: label,
		TTL:  1800,
	}

	aRecord, err := client.CreateDNSRecord(ctx, zoneID, aRecordRequest)
	if err != nil {
		return false, err
	}
	logger.Info("Added record", "zone", zone, "name", name, "type", "A", "record", aRecord)

	txtRecord, err := client.CreateDNSRecord(ctx, zone, txtRecordRequest)
	if err != nil {
		return false, err
	}
	logger.Info("Added DNS record", "zone", zone, "name", name, "type", "TXT", "record", txtRecord)

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
