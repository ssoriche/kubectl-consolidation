package consolidation

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// CalculateUtilization computes CPU and memory utilization based on pod requests.
// Returns percentages relative to node allocatable resources.
func CalculateUtilization(node *corev1.Node, pods []corev1.Pod) (cpuPercent, memPercent int) {
	allocatable := node.Status.Allocatable
	if allocatable == nil {
		return 0, 0
	}

	allocatableCPU := allocatable.Cpu()
	allocatableMem := allocatable.Memory()

	if allocatableCPU.IsZero() || allocatableMem.IsZero() {
		return 0, 0
	}

	var totalCPURequests, totalMemRequests resource.Quantity

	for _, pod := range pods {
		// Skip completed or failed pods
		if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			continue
		}

		for _, container := range pod.Spec.Containers {
			if container.Resources.Requests != nil {
				if cpu, ok := container.Resources.Requests[corev1.ResourceCPU]; ok {
					totalCPURequests.Add(cpu)
				}
				if mem, ok := container.Resources.Requests[corev1.ResourceMemory]; ok {
					totalMemRequests.Add(mem)
				}
			}
		}

		// Also count init containers (they run before main containers)
		for _, container := range pod.Spec.InitContainers {
			if container.Resources.Requests != nil {
				if cpu, ok := container.Resources.Requests[corev1.ResourceCPU]; ok {
					totalCPURequests.Add(cpu)
				}
				if mem, ok := container.Resources.Requests[corev1.ResourceMemory]; ok {
					totalMemRequests.Add(mem)
				}
			}
		}
	}

	// Calculate percentages
	cpuPercent = calculatePercentage(totalCPURequests, *allocatableCPU)
	memPercent = calculatePercentage(totalMemRequests, *allocatableMem)

	return cpuPercent, memPercent
}

func calculatePercentage(used, total resource.Quantity) int {
	if total.IsZero() {
		return 0
	}

	// Use MilliValue for CPU to get millicores, Value for memory to get bytes
	usedValue := used.MilliValue()
	totalValue := total.MilliValue()

	if totalValue == 0 {
		return 0
	}

	return int((usedValue * 100) / totalValue)
}

// FormatUtilization formats a utilization percentage for display
func FormatUtilization(percent int) string {
	return formatInt(percent) + "%"
}

func formatInt(n int) string {
	if n < 0 {
		return "-" + formatInt(-n)
	}
	if n < 10 {
		return string(rune('0' + n))
	}
	return formatInt(n/10) + string(rune('0'+n%10))
}

// FormatAge formats a duration as a human-readable age string
func FormatAge(t time.Time) string {
	d := time.Since(t)

	if d < time.Minute {
		return formatInt(int(d.Seconds())) + "s"
	}
	if d < time.Hour {
		return formatInt(int(d.Minutes())) + "m"
	}
	if d < 24*time.Hour {
		return formatInt(int(d.Hours())) + "h"
	}
	return formatInt(int(d.Hours()/24)) + "d"
}
