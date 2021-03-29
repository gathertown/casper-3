// Package kubernetes provides methods for interacting with
// an existing kubernetes cluster in a Kubestack environment.
package kubernetes

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Cluster API struct for a kubernetes clusters
type Cluster struct {
	Client kubernetes.Interface
}

// New creates a new in-cluster kubernetes client
func New() (*Cluster, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return &Cluster{Client: clientset}, nil
}
