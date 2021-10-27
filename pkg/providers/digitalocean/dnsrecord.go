package digitalocean

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/digitalocean/godo"
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
type DigitalOceanDNS struct{}

func NewDOClient() *godo.Client {
	return godo.NewFromToken(cfg.Token)
}

func (d DigitalOceanDNS) Sync(nodes []Node) {
	var nodeHostnames, dnsRecords []string

	// Setup the client
	client := NewDOClient()

	// The source of truth are the TXT records as they are created and deleted alongside 'A' records.
	recordType := "TXT"

	// Count all records in the zone. Useful for alerting purposes
	allRecords, err := getAllRecords(context.TODO(), client, cfg.Zone)
	if err != nil {
		msg := fmt.Sprintf("%v", err)
		metrics.ExecErrInc(msg)
		logger.Info("Error occured while fetching all records", "provider", cfg.Provider, "zone", cfg.Zone, "error", msg)
		logger.Info(err.Error())
		return
	}
	metrics.DNSRecordsTotal(cfg.Provider, allRecords)

	// Fetch all TXT DNS
	txtRecords, err := getRecords(context.TODO(), client, cfg.Zone, recordType)
	if err != nil {
		msg := fmt.Sprintf("%v", err)
		metrics.ExecErrInc(msg)
		logger.Info("Error occured while fetching records", "provider", cfg.Provider, "zone", cfg.Zone, "host", cfg.Subdomain)
		logger.Info(err.Error())
		return
	}

	// Generate arrays
	for _, record := range txtRecords {
		if record.Data == label {
			cName := strings.Split(record.Name, ".") // e.g. convert "sfu-v81hha.dev" to "sfu-v81hha" to allow comparison with hostnames
			dnsRecords = append(dnsRecords, cName[0])
		}
	}

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
				logger.Info("IP address not found for entry", "name", name, "zone", cfg.Zone, "subdomain", cfg.Subdomain)
			} else {
				_, err := addRecord(context.TODO(), client, cfg.Zone, name, cfg.Subdomain, addressIPv4, "", "", cfg.Env)
				if err != nil {
					msg := fmt.Sprintf("%v", err)
					metrics.ExecErrInc(msg)
					logger.Info(err.Error())
				}
			}
		}
	}

	// Remove stale entries
	deleteEntries := common.Compare(dnsRecords, nodeHostnames)
	if len(deleteEntries) > 0 {
		for _, name := range deleteEntries {
			// The 'Name' entry is the FQDN
			cName := fmt.Sprintf("%s.%s.%s", name, cfg.Subdomain, cfg.Zone)
			logger.Debug("Launching deletion", "record", cName)
			_, err := deleteRecord(context.TODO(), client, cfg.Zone, cName)
			if err != nil {
				msg := fmt.Sprintf("%v", err)
				metrics.ExecErrInc(msg)
				logger.Info(err.Error())
			}
		}
	}

	// Find kubernetes nodes to register
	return
}

func (c DigitalOceanDNS) SyncPods(pods []Pod) {
	var names, dnsRecords []string
	var txtRecordsFromPods []godo.DomainRecord

	// Setup the client
	client := NewDOClient()

	// The source of truth are the TXT records as they are created and deleted alongside 'A' records.
	// The logical flow is the following:
	// fetch txtRecords that have been created from a pod-sync operation --> indicator for this, is the existence of the `pod-sync=true` string on the txt data.
	// save pod names that have the `casper-3.gather.town/sync: "true"` label.
	// compare pod names with cNames --> if diff, then create dns records.
	// compare cNames with pod names --> if diff, then delete the stale resources.
	// compare pod names with existing txt records that have been created from a pod-sync operation --> if cname is equal to pod name, but the txtLabel has different assignedNode in comparison with the current pod assignedNode, then delete the outdated records and recreate them with proper configuration

	recordType := "TXT"

	// Fetch all TXT DNS
	txtRecords, err := getRecords(context.TODO(), client, cfg.Zone, recordType)
	if err != nil {
		msg := fmt.Sprintf("%v", err)
		metrics.ExecErrInc(msg)
		logger.Info("Error occured while fetching records", "provider", cfg.Provider, "zone", cfg.Zone, "host", cfg.Subdomain)
		logger.Info(err.Error())
		return
	}

	// Generate arrays
	for _, txtRecord := range txtRecords {
		if strings.Contains(txtRecord.Data, "pod-sync=true") { // save only the txtRecords that have been created from a pod-sync operation
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
				_, err := addRecord(context.TODO(), client, cfg.Zone, podName, cfg.Subdomain, addressIPv4, txtRecordName, txtLabel, cfg.Env)
				if err != nil {
					msg := fmt.Sprintf("%v", err)
					metrics.ExecErrInc(msg)
					logger.Info(err.Error())
				}
			}
		}
	}

	// Remove stale entries
	deleteEntries := common.Compare(dnsRecords, names)
	if len(deleteEntries) > 0 {
		logger.Info("Entries to be deleted", "entries", deleteEntries)
		for _, name := range deleteEntries {
			cName := fmt.Sprintf("%s.%s", name, cfg.Zone)
			if cfg.Subdomain != "" {
				cName = fmt.Sprintf("%s.%s.%s", name, cfg.Subdomain, cfg.Zone)
			}
			logger.Debug("Launching deletion", "record", cName)
			_, err := deleteRecord(context.TODO(), client, cfg.Zone, cName)
			if err != nil {
				msg := fmt.Sprintf("%v", err)
				metrics.ExecErrInc(msg)
				logger.Info(err.Error())
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
			if cName[0] == podName && !strings.Contains(txt.Data, assignedNode) {
				// then delete existing record and recreate new ones
				logger.Debug("Found a pod with that might got rescheduled on a different node", podName)
				cName := fmt.Sprintf("%s.%s", podName, cfg.Zone)
				if cfg.Subdomain != "" {
					cName = fmt.Sprintf("%s.%s.%s", podName, cfg.Subdomain, cfg.Zone)
				}
				logger.Debug("Launching deletion", "record", cName)
				_, err := deleteRecord(context.TODO(), client, cfg.Zone, cName)
				if err != nil {
					msg := fmt.Sprintf("%v", err)
					metrics.ExecErrInc(msg)
					logger.Info(err.Error())
				}
				_, _err := addRecord(context.TODO(), client, cfg.Zone, podName, cfg.Subdomain, addressIPv4, podName, txtLabel, cfg.Env)
				if _err != nil {
					msg := fmt.Sprintf("%v", err)
					metrics.ExecErrInc(msg)
					logger.Info(err.Error())
				}
			}
		}
	}

	// Find kubernetes pods to register
	return
}

func getRecords(ctx context.Context, client *godo.Client, domain string, recordType string) ([]godo.DomainRecord, error) {
	opt := &godo.ListOptions{
		Page:    1,
		PerPage: 1000,
	}
	records, _, err := client.Domains.RecordsByType(ctx, domain, recordType, opt)

	if err != nil {
		msg := fmt.Sprintf("%v", err)
		metrics.ExecErrInc(msg)
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
		msg := fmt.Sprintf("%v", err)
		metrics.ExecErrInc(msg)
		return false, err
	}

	aRecords, _, err := client.Domains.RecordsByTypeAndName(ctx, zone, "A", name, opt)
	if err != nil {
		msg := fmt.Sprintf("%v", err)
		metrics.ExecErrInc(msg)
		return false, err
	}

	records := append(txtRecords, aRecords...)

	for _, record := range records {
		logger.Debug("Deleting", "record", record)
		response, err := client.Domains.DeleteRecord(ctx, zone, record.ID)
		if err != nil {
			msg := fmt.Sprintf("%v", err)
			metrics.ExecErrInc(msg)
			return false, err
		}
		logger.Info("Deleted DNS record", "zone", zone, "record", record.Name, "type", record.Type, "responseStatus", response.Status)
	}
	return true, nil
}

func addRecord(ctx context.Context, client *godo.Client, zone string, name string, sub string, addressIPv4 string, txtRecordName string, txtLabel string, env string) (bool, error) {
	if txtRecordName == "" {
		txtRecordName = name
	}

	if txtLabel == "" {
		txtLabel = label
	}

	aRecordRequest := &godo.DomainRecordEditRequest{
		Type: "A",
		Name: fmt.Sprintf("%s.%s", name, sub), // Workaround for subdomains to work properly on digital ocean.
		Data: addressIPv4,
		TTL:  1800,
	}

	txtRecordRequest := &godo.DomainRecordEditRequest{
		Type: "TXT",
		Name: fmt.Sprintf("%s.%s", txtRecordName, sub),
		Data: txtLabel,
		TTL:  1800,
	}

	_, aRecordResponse, err := client.Domains.CreateRecord(ctx, zone, aRecordRequest)
	if err != nil {
		msg := fmt.Sprintf("%v", err)
		metrics.ExecErrInc(msg)
		return false, err
	}
	logger.Info("Added record", "zone", zone, "name", name, "type", "A", "responseStatus", aRecordResponse.Status)

	_, txtRecordResponse, err := client.Domains.CreateRecord(ctx, zone, txtRecordRequest)
	if err != nil {
		msg := fmt.Sprintf("%v", err)
		metrics.ExecErrInc(msg)
		return false, err
	}
	logger.Info("Added DNS record", "zone", zone, "name", name, "type", "TXT", "responseStatus", txtRecordResponse.Status)

	return true, err
}

func getAllRecords(ctx context.Context, client *godo.Client, domain string) (float64, error) {
	var n float64
	opt := &godo.ListOptions{
		Page:    0,
		PerPage: 100,
	}
	for {
		records, resp, err := client.Domains.Records(ctx, domain, opt)
		if err != nil {
			fmt.Println(err)
		}
		n += float64(len(records))
		if resp.Links == nil || resp.Links.IsLastPage() {
			break
		}
		page, err := resp.Links.CurrentPage()
		if err != nil {
			return 0.0, err
		}
		// set the page we want for the next request
		opt.Page = page + 1
	}

	return n, nil
}
