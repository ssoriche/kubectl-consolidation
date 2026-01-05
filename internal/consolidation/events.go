package consolidation

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// FetchNodeEvents retrieves events for a specific node
func FetchNodeEvents(ctx context.Context, client kubernetes.Interface, nodeName string) ([]corev1.Event, error) {
	listOpts := metav1.ListOptions{
		FieldSelector: "involvedObject.kind=Node,involvedObject.name=" + nodeName,
	}

	eventList, err := client.CoreV1().Events("").List(ctx, listOpts)
	if err != nil {
		return nil, err
	}

	return eventList.Items, nil
}

// FetchAllNodeEvents retrieves events for all nodes in a single API call
func FetchAllNodeEvents(ctx context.Context, client kubernetes.Interface) (map[string][]corev1.Event, error) {
	listOpts := metav1.ListOptions{
		FieldSelector: "involvedObject.kind=Node",
	}

	eventList, err := client.CoreV1().Events("").List(ctx, listOpts)
	if err != nil {
		return nil, err
	}

	// Group events by node name
	eventsByNode := make(map[string][]corev1.Event)
	for _, event := range eventList.Items {
		nodeName := event.InvolvedObject.Name
		eventsByNode[nodeName] = append(eventsByNode[nodeName], event)
	}

	return eventsByNode, nil
}
