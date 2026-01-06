package consolidation

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// FetchAllPods retrieves all pods cluster-wide and groups them by node name
func FetchAllPods(ctx context.Context, client kubernetes.Interface) (map[string][]corev1.Pod, error) {
	podList, err := client.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	podsByNode := make(map[string][]corev1.Pod)
	for _, pod := range podList.Items {
		if nodeName := pod.Spec.NodeName; nodeName != "" {
			podsByNode[nodeName] = append(podsByNode[nodeName], pod)
		}
	}
	return podsByNode, nil
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
