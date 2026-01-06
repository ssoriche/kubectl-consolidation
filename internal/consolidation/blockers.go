package consolidation

import (
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/ssoriche/kubectl-consolidation/internal/karpenter"
)

// BlockerType represents a reason why a node cannot be consolidated
type BlockerType string

const (
	BlockerHighUtilization    BlockerType = "high-utilization"
	BlockerDoNotEvict         BlockerType = "do-not-evict"
	BlockerDoNotDisrupt       BlockerType = "do-not-disrupt"
	BlockerDoNotConsolidate   BlockerType = "do-not-consolidate"
	BlockerPDBViolation       BlockerType = "pdb-violation"
	BlockerNonReplicated      BlockerType = "non-replicated"
	BlockerWouldIncreaseCost  BlockerType = "would-increase-cost"
	BlockerInUseSecurityGroup BlockerType = "in-use-security-group"
	BlockerOnDemandProtection BlockerType = "on-demand-protection"
	BlockerLocalStorage       BlockerType = "local-storage"
)

// HighUtilizationThreshold is the percentage above which utilization is considered high
const HighUtilizationThreshold = 80

// PodBlocker represents a pod that is blocking consolidation
type PodBlocker struct {
	NodeName  string
	Namespace string
	PodName   string
	Age       string
	Reason    BlockerType
}

// DetectPodBlocker checks if a pod has annotations that block consolidation
func DetectPodBlocker(pod *corev1.Pod) (BlockerType, bool) {
	if pod == nil || pod.Annotations == nil {
		return "", false
	}

	if pod.Annotations[karpenter.AnnotationDoNotEvict] == "true" {
		return BlockerDoNotEvict, true
	}
	if pod.Annotations[karpenter.AnnotationDoNotDisrupt] == "true" {
		return BlockerDoNotDisrupt, true
	}
	if pod.Annotations[karpenter.AnnotationDoNotConsolidate] == "true" {
		return BlockerDoNotConsolidate, true
	}

	return "", false
}

// blockerPatterns maps regex patterns to blocker types, compiled once at init
var blockerPatterns = []struct {
	pattern *regexp.Regexp
	blocker BlockerType
}{
	{regexp.MustCompile(`pdb.*prevent`), BlockerPDBViolation},
	{regexp.MustCompile(`local storage`), BlockerLocalStorage},
	{regexp.MustCompile(`non-replicated`), BlockerNonReplicated},
	{regexp.MustCompile(`would increase cost`), BlockerWouldIncreaseCost},
	{regexp.MustCompile(`in-use security group`), BlockerInUseSecurityGroup},
	{regexp.MustCompile(`on-demand`), BlockerOnDemandProtection},
	{regexp.MustCompile(`do-not-consolidate`), BlockerDoNotConsolidate},
	{regexp.MustCompile(`do-not-disrupt`), BlockerDoNotDisrupt},
	{regexp.MustCompile(`do-not-evict`), BlockerDoNotEvict},
}

// NormalizeEventMessage converts verbose Karpenter event messages to short blocker codes
func NormalizeEventMessage(message string) BlockerType {
	if message == "" {
		return ""
	}

	lower := strings.ToLower(message)
	for _, p := range blockerPatterns {
		if p.pattern.MatchString(lower) {
			return p.blocker
		}
	}

	return ""
}

// DetectBlockers analyzes pods, events, and utilization to find consolidation blockers
func DetectBlockers(pods []corev1.Pod, events []corev1.Event, cpuUtil, memUtil int, existingPodNames map[string]bool) []BlockerType {
	blockerSet := make(map[BlockerType]bool)

	// Check high utilization
	if cpuUtil >= HighUtilizationThreshold || memUtil >= HighUtilizationThreshold {
		blockerSet[BlockerHighUtilization] = true
	}

	// Check pod annotations
	for i := range pods {
		if blocker, found := DetectPodBlocker(&pods[i]); found {
			blockerSet[blocker] = true
		}
	}

	// Check events
	for _, event := range events {
		// Only process consolidation-related events
		if !isConsolidationEvent(event) {
			continue
		}

		// If event references a pod, check if pod still exists
		if podName := extractPodFromMessage(event.Message); podName != "" {
			if !existingPodNames[podName] {
				continue
			}
		}

		if blocker := NormalizeEventMessage(event.Message); blocker != "" {
			blockerSet[blocker] = true
		}
	}

	// Convert set to slice
	blockers := make([]BlockerType, 0, len(blockerSet))
	for blocker := range blockerSet {
		blockers = append(blockers, blocker)
	}

	return blockers
}

func isConsolidationEvent(event corev1.Event) bool {
	reason := event.Reason
	message := strings.ToLower(event.Message)

	return reason == "CannotConsolidate" ||
		reason == "DeprovisioningBlocked" ||
		reason == "DisruptionBlocked" ||
		strings.Contains(message, "consolidat") ||
		strings.Contains(message, "deprovision") ||
		strings.Contains(message, "disrupt")
}

var podNameRegex = regexp.MustCompile(`Pod "([^"]+)"`)

func extractPodFromMessage(message string) string {
	matches := podNameRegex.FindStringSubmatch(message)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

// FormatBlockers converts a slice of blockers to a display string
func FormatBlockers(blockers []BlockerType) string {
	if len(blockers) == 0 {
		return "<none>"
	}

	strs := make([]string, len(blockers))
	for i, b := range blockers {
		strs[i] = string(b)
	}
	return strings.Join(strs, ",")
}
