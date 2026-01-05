package consolidation

import (
	"context"
	"sort"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// FetchNodes retrieves nodes from the cluster, optionally filtered by names or label selector.
// Results are sorted by creation timestamp (oldest first).
func FetchNodes(ctx context.Context, client kubernetes.Interface, nodeNames []string, selector string) ([]corev1.Node, error) {
	listOpts := metav1.ListOptions{}
	if selector != "" {
		listOpts.LabelSelector = selector
	}

	nodeList, err := client.CoreV1().Nodes().List(ctx, listOpts)
	if err != nil {
		return nil, err
	}

	nodes := nodeList.Items

	// Filter by specific node names if provided
	if len(nodeNames) > 0 {
		nameSet := make(map[string]bool)
		for _, name := range nodeNames {
			nameSet[name] = true
		}

		filtered := make([]corev1.Node, 0, len(nodeNames))
		for _, node := range nodes {
			if nameSet[node.Name] {
				filtered = append(filtered, node)
			}
		}
		nodes = filtered
	}

	// Sort by creation timestamp (oldest first)
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].CreationTimestamp.Before(&nodes[j].CreationTimestamp)
	})

	return nodes, nil
}

// GetNodeStatus returns a simplified status string for a node
func GetNodeStatus(node *corev1.Node) string {
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			if condition.Status == corev1.ConditionTrue {
				return "Ready"
			}
			return "NotReady"
		}
	}
	return "Unknown"
}

// GetNodeRoles returns a comma-separated string of node roles
func GetNodeRoles(node *corev1.Node) string {
	roles := []string{}
	for label := range node.Labels {
		if len(label) > 24 && label[:24] == "node-role.kubernetes.io/" {
			role := label[24:]
			if role != "" {
				roles = append(roles, role)
			}
		}
	}
	if len(roles) == 0 {
		return "<none>"
	}
	sort.Strings(roles)
	return join(roles, ",")
}

func join(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
