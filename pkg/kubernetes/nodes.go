package kubernetes

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/gathertown/casper-3/internal/config"
	"github.com/gathertown/casper-3/internal/metrics"
	common "github.com/gathertown/casper-3/pkg"
	"github.com/gathertown/casper-3/pkg/log"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Node = common.Node

var cfg = config.FromEnv()
var logger = log.New(os.Stdout, cfg.LogLevel)

// Returns []Node struct listing hostname and IPv4 address
func (c *Cluster) Nodes() ([]Node, error) {
	var nodes []Node

	n, err := c.GetNodes(cfg.LabelKey, cfg.LabelValues)
	if err != nil {
		metrics.ExecErrInc(err.Error())
		return nil, err
	}

	for _, node := range n.Items {
		foundIP := false
		for _, addr := range node.Status.Addresses {
			if addr.Type != "ExternalIP" {
				// if `ExternalIP` not found hop to the next iteration
				continue
			}
			nodeName := strings.Split(node.Name, ".")[0]
			logger.Debug("IPv4 address found", "node", nodeName, "IPv4", addr.Address)
			nodes = append(nodes, Node{nodeName, addr.Address})
			foundIP = true
			break
		}
		if !foundIP {
			logger.Info("No IPv4 address found", "node", node.Name)
		}
	}

	return nodes, nil
}

// GetNodes returns the list of cluster nodes
func (c *Cluster) GetNodes(labelKey string, labelValues string) (*v1.NodeList, error) {
	labelSelector := fmt.Sprintf("%s in (%s)", labelKey, labelValues)
	opts := metav1.ListOptions{
		LabelSelector: labelSelector,
		Limit:         300,
	}
	n, err := c.Client.CoreV1().Nodes().List(context.TODO(), opts)
	if err != nil {
		metrics.ExecErrInc(err.Error())
		return nil, err
	}
	return n, nil
}

func (c *Cluster) getExternalIpByNodeName(nodeName string) (string, error) {
	n, err := c.Client.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
	if err != nil {
		metrics.ExecErrInc(err.Error())
		return "", err
	}
	ip := n.Status.Addresses[2].Address
	return ip, nil
}
