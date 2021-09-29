package kubernetes

import (
	"context"
	"fmt"

	common "github.com/gathertown/casper-3/pkg"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Pod = common.Pod

// Returns []Pod struct listing pod name, assigned Node and podLabels
func (c *Cluster) Pods() ([]Pod, error) {
	var pods []Pod

	p, err := c.GetPods(cfg.SyncPodLabelKey, cfg.SyncPodLabelValue)
	if err != nil {
		return nil, err
	}

	for _, pod := range p.Items {
		externalIp, err := c.getExternalIpByNodeName(pod.Spec.NodeName)
		if err != nil {
			return nil, err
		}
		podLabels := make(map[string]string)
		podLabels = pod.Labels
		pods = append(pods, Pod{pod.Name, Node{pod.Spec.NodeName, externalIp}, podLabels})
	}

	return pods, nil
}

// GetPods returns the list of cluster pods with the label: casper-3.gather.town/sync=true
func (c *Cluster) GetPods(labelKey string, labelValue string) (*v1.PodList, error) {
	labelSelector := fmt.Sprintf("%s=%s", labelKey, labelValue)
	opts := metav1.ListOptions{
		LabelSelector: labelSelector,
		Limit:         300,
	}
	p, err := c.Client.CoreV1().Pods("").List(context.TODO(), opts)
	if err != nil {
		return nil, err
	}
	return p, nil
}
