package consolidation

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// FetchPodsOnNode retrieves all pods running on a specific node
func FetchPodsOnNode(ctx context.Context, client kubernetes.Interface, nodeName string) ([]corev1.Pod, error) {
	listOpts := metav1.ListOptions{
		FieldSelector: "spec.nodeName=" + nodeName,
	}

	podList, err := client.CoreV1().Pods("").List(ctx, listOpts)
	if err != nil {
		return nil, err
	}

	return podList.Items, nil
}

// BuildPodNameSet creates a set of "namespace/name" strings for quick lookup
func BuildPodNameSet(pods []corev1.Pod) map[string]bool {
	set := make(map[string]bool, len(pods))
	for _, pod := range pods {
		key := pod.Namespace + "/" + pod.Name
		set[key] = true
	}
	return set
}

// FindBlockingPods returns pods that have consolidation-blocking annotations
func FindBlockingPods(pods []corev1.Pod, nodeName string) []PodBlocker {
	var blockers []PodBlocker

	for i := range pods {
		pod := &pods[i]
		if blocker, found := DetectPodBlocker(pod); found {
			blockers = append(blockers, PodBlocker{
				NodeName:  nodeName,
				Namespace: pod.Namespace,
				PodName:   pod.Name,
				Age:       FormatAge(pod.CreationTimestamp.Time),
				Reason:    blocker,
			})
		}
	}

	return blockers
}
