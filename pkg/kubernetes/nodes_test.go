package kubernetes

import (
	"context"
	"os"
	"sort"
	"testing"

	"github.com/gathertown/casper-3/internal/config"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	f "k8s.io/client-go/kubernetes/fake"
)

var nodeOpts = []struct {
	localIP     string
	internalIP  string
	externalIP  string
	clusterName string
	nodeName    string
	labelKey    string
	labelValue  string
}{
	{"127.0.0.1", "10.0.0.1", "1.1.1.1", "test", "sfu-8mh0d", config.FromEnv().LabelKey, "sfu"},
	{"127.0.0.1", "10.0.0.2", "1.1.1.2", "test", "sfu-8quob", config.FromEnv().LabelKey, "sfu"},
	{"127.0.0.1", "10.0.0.3", "1.1.1.3", "test", "default-8quob", "k8s.label.key/gather", "false"},
	{"127.0.0.1", "10.0.0.4", "1.1.1.4", "test", "default-8q8gq", "k8s.label.key/gather", "false"},
	{"127.0.0.1", "10.0.0.5", "1.1.1.5", "test", "default-8ub75", "k8s.label.key/gather", "false"},
	{"127.0.0.1", "10.0.0.6", "1.1.1.6", "test", "monitoring-835tv", "k8s.label.key/gather", "false"},
	{"127.0.0.1", "10.0.0.7", "1.1.1.7", "test", "router-4quob", config.FromEnv().LabelKey, "router"},
}

func contains(s []string, searchterm string) bool {
	i := sort.SearchStrings(s, searchterm)
	return i < len(s) && s[i] == searchterm
}

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

func setupCluster(t *testing.T) Cluster {
	t.Helper()
	c := Cluster{Client: f.NewSimpleClientset()}
	opts := metav1.CreateOptions{}
	for _, tt := range nodeOpts {
		labels := map[string]string{
			tt.labelKey: tt.labelValue,
		}
		nodeStatus := v1.NodeStatus{
			Addresses: []v1.NodeAddress{
				{Type: v1.NodeHostName, Address: tt.localIP},
				{Type: v1.NodeInternalIP, Address: tt.internalIP},
				{Type: v1.NodeExternalIP, Address: tt.externalIP},
			},
		}

		node := &v1.Node{ObjectMeta: metav1.ObjectMeta{Name: tt.nodeName, ClusterName: tt.clusterName, Labels: labels}, Status: nodeStatus}
		_, _ = c.Client.CoreV1().Nodes().Create(context.TODO(), node, opts)

	}
	return c
}

func TestGetNodes(t *testing.T) {
	setenv(t, "LABEL_VALUES", "sfu,router")

	c := setupCluster(t)
	cfg := config.FromEnv()
	nodes := 3
	n, _ := c.GetNodes(cfg.LabelKey, cfg.LabelValues)

	// test number of nods with label
	if len(n.Items) != nodes {
		t.Errorf("Expecting %v nodes, got %v nodes", nodes, len(n.Items))
	}

	externalIPList := []string{"1.1.1.1", "1.1.1.2", "1.1.1.7"}
	sort.Strings(externalIPList)

	// fetch IP addresses of nodes
	for _, node := range n.Items {
		if !contains(externalIPList, node.Status.Addresses[2].Address) {
			t.Errorf("Expecting one of the following externalIP(s) %v, got %v nodes", externalIPList, node.Status.Addresses[2].Address)
		}
	}
	unsetenv(t, "LABEL_VALUES")
}
