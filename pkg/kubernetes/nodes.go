package kubernetes

import (
	"context"
	"fmt"
	"os"

	"github.com/gathertown/casper-3/internal/config"
	common "github.com/gathertown/casper-3/pkg"
	"github.com/gathertown/casper-3/pkg/log"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Node = common.Node

var cfg = config.FromEnv()
var logger = log.New(os.Stdout, cfg.Env)

// GetNodes returns the list of cluster nodes
func (c *Cluster) Nodes() ([]Node, error) {
	var nodes []Node

	n, err := c.GetNodes(cfg.LabelKey, cfg.LabelValue)
	if err != nil {
		return nil, err
	}

	for _, node := range n.Items {
		nodes = append(nodes, Node{node.Name, node.Status.Addresses[2].Address})
	}

	return nodes, nil
}

// GetNodes returns the list of cluster nodes
func (c *Cluster) GetNodes(labelKey string, labelValue string) (*v1.NodeList, error) {
	labelSelector := fmt.Sprintf("%s=%s", labelKey, labelValue)
	opts := metav1.ListOptions{
		LabelSelector: labelSelector,
		Limit:         300,
	}
	n, err := c.Client.CoreV1().Nodes().List(context.TODO(), opts)
	if err != nil {
		return nil, err
	}
	return n, nil
}
