package common

import (
	"fmt"
	"strings"
)

// shared structures
type Node struct {
	Name       string
	ExternalIP string
}

type Pod struct {
	Name         string
	AssignedNode Node
	Labels       map[string]string
}

// Compare slices: https://stackoverflow.com/a/45428032/577133
// Returns []string of elements found in 'a' but not in 'b'.
func Compare(a, b []string) []string {
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

// Checks if a recordName follows the prefix pattern that k8s nodes have
func RecordPrefixMatchesNodePrefixes(recordName string, nodeNames []string) bool {
	for _, node := range nodeNames {
		nodePrefix := strings.Split(node, "-")[0]
		prefix := fmt.Sprintf("%s-", nodePrefix)
		if strings.HasPrefix(recordName, prefix) {
			return true
		}
	}
	return false
}
