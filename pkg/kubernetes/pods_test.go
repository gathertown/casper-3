package kubernetes

import (
	"context"
	"testing"

	"github.com/gathertown/casper-3/internal/config"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	f "k8s.io/client-go/kubernetes/fake"
)

var mockPodsOpts = []struct {
	podName    string
	labelKey   string
	labelValue string
}{
	{"router-0", cfg.SyncPodLabelKey, cfg.SyncPodLabelValue},
	{"router-1", "casper-3.gather.town/sync", "true"},
	{"router-2", "casper-3.gather.town", "false"},
	{"router-3", "casper-3.gather.town/domain", ""},
	{"router-4", "casper-3.gather.town/donothing", "nil"},
}

var mockNodeOpts = struct {
	localIP     string
	internalIP  string
	externalIP  string
	clusterName string
	nodeName    string
	labelKey    string
	labelValue  string
}{
	"127.0.0.1", "10.0.0.1", "1.1.1.1", "test", "sfu-8mh0d", config.FromEnv().LabelKey, "sfu",
}

func setupClusterWithPods(t *testing.T) Cluster {
	t.Helper()
	c := Cluster{Client: f.NewSimpleClientset()}
	opts := metav1.CreateOptions{}
	labels := map[string]string{
		mockNodeOpts.labelKey: mockNodeOpts.labelValue,
	}
	nodeStatus := v1.NodeStatus{
		Addresses: []v1.NodeAddress{
			{Type: v1.NodeHostName, Address: mockNodeOpts.localIP},
			{Type: v1.NodeInternalIP, Address: mockNodeOpts.internalIP},
			{Type: v1.NodeExternalIP, Address: mockNodeOpts.externalIP},
		},
	}

	node := &v1.Node{ObjectMeta: metav1.ObjectMeta{Name: mockNodeOpts.nodeName, ClusterName: mockNodeOpts.clusterName, Labels: labels}, Status: nodeStatus}
	_, _ = c.Client.CoreV1().Nodes().Create(context.TODO(), node, opts)

	for _, p := range mockPodsOpts {
		podLabels := map[string]string{
			p.labelKey: p.labelValue,
		}
		pod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: p.podName, Labels: podLabels}, Spec: v1.PodSpec{NodeName: node.Name}}
		_, _ = c.Client.CoreV1().Pods("").Create(context.TODO(), pod, opts)

	}
	return c
}

func TestGetPods(t *testing.T) {
	c := setupClusterWithPods(t)
	cfg := config.FromEnv()
	pods := 2
	p, _ := c.GetPods(cfg.SyncPodLabelKey, cfg.SyncPodLabelValue)

	// test number of pods with label
	if len(p.Items) != pods {
		t.Errorf("Expecting %v pods, got %v pods", pods, len(p.Items))
	}

	podNamesList := []string{"router-0", "router-1"}

	// fetch pod names
	for _, pod := range p.Items {
		if !contains(podNamesList, pod.Name) {
			t.Errorf("Expecting one of the following pod Name(s) %v, got %v pods", podNamesList, pod.Name)
		}
	}
}
