package consolidation

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestCalculateUtilization(t *testing.T) {
	tests := []struct {
		name           string
		node           *corev1.Node
		pods           []corev1.Pod
		expectedCPU    int
		expectedMemory int
	}{
		{
			name: "empty node",
			node: &corev1.Node{
				Status: corev1.NodeStatus{
					Allocatable: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("4"),
						corev1.ResourceMemory: resource.MustParse("8Gi"),
					},
				},
			},
			pods:           nil,
			expectedCPU:    0,
			expectedMemory: 0,
		},
		{
			name: "50% utilization",
			node: &corev1.Node{
				Status: corev1.NodeStatus{
					Allocatable: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("4"),
						corev1.ResourceMemory: resource.MustParse("8Gi"),
					},
				},
			},
			pods: []corev1.Pod{
				{
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("2"),
										corev1.ResourceMemory: resource.MustParse("4Gi"),
									},
								},
							},
						},
					},
				},
			},
			expectedCPU:    50,
			expectedMemory: 50,
		},
		{
			name: "multiple pods",
			node: &corev1.Node{
				Status: corev1.NodeStatus{
					Allocatable: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("4"),
						corev1.ResourceMemory: resource.MustParse("8Gi"),
					},
				},
			},
			pods: []corev1.Pod{
				{
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("1"),
										corev1.ResourceMemory: resource.MustParse("2Gi"),
									},
								},
							},
						},
					},
				},
				{
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("2"),
										corev1.ResourceMemory: resource.MustParse("4Gi"),
									},
								},
							},
						},
					},
				},
			},
			expectedCPU:    75,
			expectedMemory: 75,
		},
		{
			name: "skip completed pods",
			node: &corev1.Node{
				Status: corev1.NodeStatus{
					Allocatable: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("4"),
						corev1.ResourceMemory: resource.MustParse("8Gi"),
					},
				},
			},
			pods: []corev1.Pod{
				{
					Status: corev1.PodStatus{
						Phase: corev1.PodSucceeded,
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("2"),
										corev1.ResourceMemory: resource.MustParse("4Gi"),
									},
								},
							},
						},
					},
				},
			},
			expectedCPU:    0,
			expectedMemory: 0,
		},
		{
			name: "millicores",
			node: &corev1.Node{
				Status: corev1.NodeStatus{
					Allocatable: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1000m"),
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
				},
			},
			pods: []corev1.Pod{
				{
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("500m"),
										corev1.ResourceMemory: resource.MustParse("512Mi"),
									},
								},
							},
						},
					},
				},
			},
			expectedCPU:    50,
			expectedMemory: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cpuPercent, memPercent := CalculateUtilization(tt.node, tt.pods)
			if cpuPercent != tt.expectedCPU {
				t.Errorf("CalculateUtilization() CPU = %v, want %v", cpuPercent, tt.expectedCPU)
			}
			if memPercent != tt.expectedMemory {
				t.Errorf("CalculateUtilization() Memory = %v, want %v", memPercent, tt.expectedMemory)
			}
		})
	}
}

func TestFormatUtilization(t *testing.T) {
	tests := []struct {
		percent  int
		expected string
	}{
		{0, "0%"},
		{50, "50%"},
		{100, "100%"},
	}

	for _, tt := range tests {
		got := FormatUtilization(tt.percent)
		if got != tt.expected {
			t.Errorf("FormatUtilization(%d) = %v, want %v", tt.percent, got, tt.expected)
		}
	}
}

func TestFormatAge(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		time     time.Time
		expected string
	}{
		{
			name:     "seconds",
			time:     now.Add(-30 * time.Second),
			expected: "30s",
		},
		{
			name:     "minutes",
			time:     now.Add(-5 * time.Minute),
			expected: "5m",
		},
		{
			name:     "hours",
			time:     now.Add(-3 * time.Hour),
			expected: "3h",
		},
		{
			name:     "days",
			time:     now.Add(-2 * 24 * time.Hour),
			expected: "2d",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatAge(tt.time)
			if got != tt.expected {
				t.Errorf("FormatAge() = %v, want %v", got, tt.expected)
			}
		})
	}
}
