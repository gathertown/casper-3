package cloudflare

import (
	"context"
	"fmt"
	"os"
	"strings"

	cloudflare "github.com/cloudflare/cloudflare-go"
	"github.com/gathertown/casper-3/internal/config"
	"github.com/gathertown/casper-3/internal/metrics"
	common "github.com/gathertown/casper-3/pkg"
	"github.com/gathertown/casper-3/pkg/log"
)

var cfg = config.FromEnv()
var logger = log.New(os.Stdout, cfg.LogLevel)
var label = fmt.Sprintf("heritage=casper-3,environment=%s", cfg.Env)

type Node = common.Node
type Pod = common.Pod
type CloudFlareDNS struct{}

func NewCFClient() *cloudflare.API {
	// If we have debug mode enabled, pass that over to the CF client as well
	debug := false
	if strings.ToLower(cfg.LogLevel) == "debug" {
		debug = true
	}
	api, err := cloudflare.NewWithAPIToken(cfg.Token, cloudflare.Debug(debug))
	if err != nil {
		metrics.ExecErrInc(err.Error())
		logger.Error("Error while creating client", "provider", cfg.Provider, "zone", cfg.Zone, "error", err.Error())
	}
	return api
}

func (d CloudFlareDNS) Sync(nodes []Node) {
	var nodeHostnames, dnsRecords []string

	// Setup the client
	client := NewCFClient()

	// The source of truth are the TXT records as they are created and deleted alongside 'A' records.
	recordType := "TXT"

	// Count all records in the zone. Useful for alerting purposes.
	// This call is expensive. Takes up to ~50s for 3k records. Run in a Goroutine.
	go func() {
		allRecords, err := getAllRecords(context.TODO(), client, cfg.Zone)
		if err != nil {
			metrics.ExecErrInc(err.Error())
			logger.Error("Error occured while fetching all records", "provider", cfg.Provider, "zone", cfg.Zone, "error", err.Error())
		} else {
			metrics.DNSRecordsTotal(cfg.Provider, allRecords)
		}
	}()

	// Fetch all TXT DNS that contain cluster's label
	txtRecords, err := getRecordsPerTypePerContent(context.TODO(), client, cfg.Zone, recordType, label)
	if err != nil {
		metrics.ExecErrInc(err.Error())
		logger.Error("Error occured while fetching records", "provider", cfg.Provider, "zone", cfg.Zone, "error", err.Error())
		return
	}

	// Generate arrays
	for _, record := range txtRecords {
		// convert "sfu-v81hha.dev" to "sfu-v81hha" to allow comparison with hostnames
		cName := strings.Split(record.Name, ".")
		dnsRecords = append(dnsRecords, cName[0])
	}
	logger.Debug("DNS records found", "records", dnsRecords)

	for _, node := range nodes {
		// convert "ip-1-2-3-4.ec.internal" to "ip-1-2-3-4" to avoid DNS A record setup failure on Cloudflare
		nodeHostnames = append(nodeHostnames, node.Name)
	}
	logger.Debug("SFU nodes found", "nodes", nodeHostnames)

	// Find new entries
	addEntries := common.Compare(nodeHostnames, dnsRecords)
	if len(addEntries) > 0 {
		logger.Info("Entries to be added", "entries", addEntries)
		for _, name := range addEntries {
			addressIPv4 := ""
			// this loop seems a bit inefficient at first glance
			// entries are bellow 1k, so shouldn't really matter.
			for _, entry := range nodes {
				logger.Debug("entries", "name", name, "zone", cfg.Zone, "entries", entry.ExternalIP)
				if entry.Name == name {
					addressIPv4 = entry.ExternalIP
				}
			}
			// Does this check make sense?
			if addressIPv4 == "" {
				logger.Info("IP address not found for entry", "name", name, "zone", cfg.Zone)
			} else {
				_, err := addRecord(context.TODO(), client, cfg.Zone, cfg.Subdomain, name, addressIPv4, "", "", cfg.Env)
				if err != nil {
					metrics.ExecErrInc(err.Error())
					logger.Error("Error occured while adding record", "provider", cfg.Provider, "zone", cfg.Zone, "name", name, "error", err.Error())
				}
			}
		}
	}

	// Remove stale entries
	deleteEntries := common.Compare(dnsRecords, nodeHostnames)
	if len(deleteEntries) > 0 {
		logger.Info("Entries to be deleted", "entries", deleteEntries)
		for _, name := range deleteEntries {
			// The 'Name' entry is the FQDN
			cName := fmt.Sprintf("%s.%s", name, cfg.Zone)
			if cfg.Subdomain != "" {
				cName = fmt.Sprintf("%s.%s.%s", name, cfg.Subdomain, cfg.Zone)
			}
			if isRecordSafeForDeletion := common.RecordPrefixMatchesNodePrefixes(cName, nodeHostnames); !isRecordSafeForDeletion {
				logger.Info("Casper-3 wants to delete this record", "record", cName, "Skipping..")
				continue
			}
			logger.Debug("Launching deletion", "record", cName)
			_, err := deleteRecord(context.TODO(), client, cfg.Zone, cName)
			if err != nil {
				metrics.ExecErrInc(err.Error())
				logger.Error("Error occured while launching deletion", "provider", cfg.Provider, "zone", cfg.Zone, "error", err.Error())
			}
		}
	}

	// Find kubernetes nodes to register
	return
}

func (c CloudFlareDNS) SyncPods(pods []Pod) {
	var names, dnsRecords []string
	var txtRecordsFromPods []cloudflare.DNSRecord

	// Setup the client
	client := NewCFClient()

	// The source of truth are the TXT records as they are created and deleted alongside 'A' records.
	// The logical flow is the following:
	// fetch txtRecords that have been created from a pod-sync operation --> indicator for this, is the existence of the `pod-sync=true` string on the txt data.
	// save pod names that have the `casper-3.gather.town/sync: "true"` label.
	// compare pod names with cNames --> if diff, then create dns records.
	// compare cNames with pod names --> if diff, then delete the stale resources.
	// compare pod names with existing txt records that have been created from a pod-sync operation --> if cname is equal to pod name, but the txtLabel has different assignedNode in comparison with the current pod assignedNode, then delete the outdated records and recreate them with proper configuration

	recordType := "TXT"

	// Fetch all TXT DNS that contain cluster's label
	txtRecords, err := getRecordsPerTypePerContent(context.TODO(), client, cfg.Zone, recordType, label)
	if err != nil {
		metrics.ExecErrInc(err.Error())
		logger.Info("Error occured while fetching records", "provider", cfg.Provider, "zone", cfg.Zone, "host", cfg.Subdomain)
		return
	}

	// Generate arrays
	for _, txtRecord := range txtRecords {
		recordData := fmt.Sprintf("%v", txtRecord.Content) // convert interface{} to string

		if strings.Contains(recordData, "pod-sync=true") { // save only the txtRecords that have been created from a pod-sync operation
			cName := strings.Split(txtRecord.Name, ".")
			dnsRecords = append(dnsRecords, cName[0])
			txtRecordsFromPods = append(txtRecordsFromPods, txtRecord)
		}
	}

	for _, pod := range pods {
		names = append(names, pod.Name)
	}
	logger.Debug("Pods found", "pods", names)

	// Find new entries
	addEntries := common.Compare(names, dnsRecords)
	if len(addEntries) > 0 {
		logger.Info("Entries to be added", "entries", addEntries)
		for _, name := range addEntries {
			podName := ""
			assignedNode := ""
			addressIPv4 := ""

			for _, pod := range pods {
				if pod.Name == name {
					podName = pod.Name
					assignedNode = pod.AssignedNode.Name
					addressIPv4 = pod.AssignedNode.ExternalIP
				}
			}

			if addressIPv4 == "" {
				logger.Info("IP address not found for entry", "name", name, "zone", cfg.Zone, "subdomain", cfg.Subdomain)
			} else {
				txtLabel := fmt.Sprintf("heritage=casper-3,pod-sync=true,environment=%s,podName=%s,assignedNode=%s,addressIPv4=%s", cfg.Env, podName, assignedNode, addressIPv4)
				txtRecordName := podName
				_, err := addRecord(context.TODO(), client, cfg.Zone, cfg.Subdomain, podName, addressIPv4, txtRecordName, txtLabel, cfg.Env)
				if err != nil {
					metrics.ExecErrInc(err.Error())
					logger.Error("Error occured while adding records", "provider", cfg.Provider, "zone", cfg.Zone, "error", err.Error())
				}
			}
		}
	}

	// Remove stale entries
	deleteEntries := common.Compare(dnsRecords, names)
	if len(deleteEntries) > 0 {
		logger.Info("Entries to be deleted", "entries", deleteEntries)
		for _, name := range deleteEntries {
			// The 'Name' entry is the FQDN
			cName := fmt.Sprintf("%s.%s", name, cfg.Zone)
			if cfg.Subdomain != "" {
				cName = fmt.Sprintf("%s.%s.%s", name, cfg.Subdomain, cfg.Zone)
			}
			logger.Debug("Launching deletion", "record", cName)
			_, err := deleteRecord(context.TODO(), client, cfg.Zone, cName)
			if err != nil {
				metrics.ExecErrInc(err.Error())
				logger.Error("Error occured while fetching records", "provider", cfg.Provider, "zone", cfg.Zone, "error", err.Error())
			}
		}
	}

	// Detect if an already registered pod has been rescheduled on a different node and update records accordingly
	for _, pod := range pods {
		podName := pod.Name
		assignedNode := pod.AssignedNode.Name
		addressIPv4 := pod.AssignedNode.ExternalIP
		txtLabel := fmt.Sprintf("heritage=casper-3,pod-sync=true,environment=%s,podName=%s,assignedNode=%s,addressIPv4=%s", cfg.Env, podName, assignedNode, addressIPv4)
		for _, txt := range txtRecordsFromPods {
			cName := strings.Split(txt.Name, ".")
			txtData := fmt.Sprintf("%v", txt.Content) // convert interface{} to string
			if cName[0] == podName && !strings.Contains(txtData, assignedNode) {
				// then delete existing record and recreate new ones
				logger.Debug("Found a pod with that might got rescheduled on a different node", podName)
				cName := fmt.Sprintf("%s.%s", podName, cfg.Zone)
				if cfg.Subdomain != "" {
					cName = fmt.Sprintf("%s.%s.%s", podName, cfg.Subdomain, cfg.Zone)
				}
				logger.Debug("Launching deletion", "record", cName)
				_, err := deleteRecord(context.TODO(), client, cfg.Zone, cName)
				if err != nil {
					metrics.ExecErrInc(err.Error())
					logger.Error("Error occured while deleting record", "provider", cfg.Provider, "zone", cfg.Zone, "error", err.Error())
				}
				_, _err := addRecord(context.TODO(), client, cfg.Zone, cfg.Subdomain, podName, addressIPv4, podName, txtLabel, cfg.Env)
				if _err != nil {
					metrics.ExecErrInc(err.Error())
					logger.Error("Error occured while adding record", "provider", cfg.Provider, "zone", cfg.Zone, "error", err.Error())
				}
			}
		}
	}
	// Find kubernetes pods to register
	return
}

func getRecordsPerTypePerContent(ctx context.Context, client *cloudflare.API, zone string, recordType string, contentLabel string) ([]cloudflare.DNSRecord, error) {

	// Get ZoneID
	zoneID, err := client.ZoneIDByName(zone)
	if err != nil {
		metrics.ExecErrInc(err.Error())
		return nil, err
	}

	// Filtering by content doesn't work unfortunately,
	// see https://github.com/cloudflare/cloudflare-go/issues/613
	record := cloudflare.DNSRecord{Type: recordType, Content: "contains:" + contentLabel}
	records, err := client.DNSRecords(ctx, zoneID, record)
	if err != nil {
		metrics.ExecErrInc(err.Error())
		return nil, err
	}

	logger.Debug("Fetched DNS records", "type", recordType)
	return records, err
}

func deleteRecord(ctx context.Context, client *cloudflare.API, zone string, fqdn string) (bool, error) {
	// Get ZoneID
	zoneID, err := client.ZoneIDByName(zone)
	if err != nil {
		metrics.ExecErrInc(err.Error())
		return false, err
	}

	logger.Debug("Deleting", "FQDN", fqdn)

	txtRecord := cloudflare.DNSRecord{Name: fqdn, Type: "TXT"}
	txtRecords, err := client.DNSRecords(ctx, zoneID, txtRecord)
	if err != nil {
		metrics.ExecErrInc(err.Error())
		return false, err
	}

	aRecord := cloudflare.DNSRecord{Name: fqdn, Type: "A"}
	aRecords, err := client.DNSRecords(ctx, zoneID, aRecord)
	if err != nil {
		metrics.ExecErrInc(err.Error())
		return false, err
	}

	records := append(txtRecords, aRecords...)

	for _, record := range records {
		// validate record to be deleted. Only records with name same as the fqdn input and type `TXT` or `A` are allowed to be deleted
		if record.Name == fqdn && (record.Type == "TXT" || record.Type == "A") {
			err := client.DeleteDNSRecord(ctx, zoneID, record.ID)
			if err != nil {
				metrics.ExecErrInc(err.Error())
				return false, err
			}
			logger.Info("Deleted DNS record", "zone", zone, "record", record.Name, "type", record.Type)
		} else {
			err := fmt.Errorf("deleteRecord() wants to delete wrong record. Record Name: %v Record Type: %v", record.Name, record.Type)
			return false, err
		}
	}
	return true, nil
}

func addRecord(ctx context.Context, client *cloudflare.API, zone string, subdomain string, name string, addressIPv4 string, txtRecordName string, txtLabel string, env string) (bool, error) {
	// Construct FQDN by populating 'name' field: sfu-123 vs sfu-123.region-a.env.cloud
	sName := name

	if txtRecordName == "" {
		txtRecordName = name
	}

	if txtLabel == "" {
		txtLabel = label
	}

	if subdomain != "" {
		sName = fmt.Sprintf("%s.%s", name, subdomain)
		txtRecordName = fmt.Sprintf("%s.%s", txtRecordName, subdomain)
	}

	// Get ZoneID
	zoneID, err := client.ZoneIDByName(zone)
	if err != nil {
		metrics.ExecErrInc(err.Error())
		return false, err
	}

	txtRecordRequest := cloudflare.DNSRecord{
		Type:    "TXT",
		Name:    txtRecordName,
		Content: txtLabel,
		TTL:     1800,
	}

	logger.Info("trying to add record", "zone", zone, "name", txtRecordName, "type", "TXT")
	txtRecord, err := client.CreateDNSRecord(ctx, zoneID, txtRecordRequest)
	if err != nil {
		metrics.ExecErrInc(err.Error())
		return false, err
	}

	proxied := false
	for _, p := range cfg.CloudflareProxiedNodePools {
		if strings.HasPrefix(name, p) {
			proxied = true
			break
		}
	}

	logger.Info("Added DNS record", "zone", zone, "name", sName, "type", "TXT", "success", txtRecord.Success)

	aRecordRequest := cloudflare.DNSRecord{
		Type:    "A",
		Name:    sName,
		Content: addressIPv4,
		TTL:     1800,
		Proxied: &proxied,
	}

	logger.Info("trying to add record", "zone", zone, "name", sName, "type", "A")
	aRecord, err := client.CreateDNSRecord(ctx, zoneID, aRecordRequest)
	if err != nil {
		metrics.ExecErrInc(err.Error())
		return false, err
	}
	logger.Info("Added record", "zone", zone, "name", sName, "type", "A", "success", aRecord.Success, "content", addressIPv4, "proxied", proxied)

	return true, err
}

func getAllRecords(ctx context.Context, client *cloudflare.API, zone string) (float64, error) {

	// Get ZoneID
	zoneID, err := client.ZoneIDByName(zone)
	if err != nil {
		metrics.ExecErrInc(err.Error())
		return 0.0, err
	}

	record := cloudflare.DNSRecord{}
	records, err := client.DNSRecords(ctx, zoneID, record)
	if err != nil {
		metrics.ExecErrInc(err.Error())
		return 0.0, err
	}
	return float64(len(records)), err
}
