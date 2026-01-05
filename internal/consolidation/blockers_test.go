package consolidation

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ssoriche/kubectl-consolidation/internal/karpenter"
)

func TestDetectPodBlocker(t *testing.T) {
	tests := []struct {
		name          string
		pod           *corev1.Pod
		expectedType  BlockerType
		expectedFound bool
	}{
		{
			name:          "nil pod",
			pod:           nil,
			expectedType:  "",
			expectedFound: false,
		},
		{
			name: "pod without annotations",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pod",
				},
			},
			expectedType:  "",
			expectedFound: false,
		},
		{
			name: "pod with do-not-evict",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pod",
					Annotations: map[string]string{
						karpenter.AnnotationDoNotEvict: "true",
					},
				},
			},
			expectedType:  BlockerDoNotEvict,
			expectedFound: true,
		},
		{
			name: "pod with do-not-disrupt",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pod",
					Annotations: map[string]string{
						karpenter.AnnotationDoNotDisrupt: "true",
					},
				},
			},
			expectedType:  BlockerDoNotDisrupt,
			expectedFound: true,
		},
		{
			name: "pod with do-not-consolidate",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pod",
					Annotations: map[string]string{
						karpenter.AnnotationDoNotConsolidate: "true",
					},
				},
			},
			expectedType:  BlockerDoNotConsolidate,
			expectedFound: true,
		},
		{
			name: "pod with annotation set to false",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pod",
					Annotations: map[string]string{
						karpenter.AnnotationDoNotEvict: "false",
					},
				},
			},
			expectedType:  "",
			expectedFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blockerType, found := DetectPodBlocker(tt.pod)
			if blockerType != tt.expectedType {
				t.Errorf("DetectPodBlocker() type = %v, want %v", blockerType, tt.expectedType)
			}
			if found != tt.expectedFound {
				t.Errorf("DetectPodBlocker() found = %v, want %v", found, tt.expectedFound)
			}
		})
	}
}

func TestNormalizeEventMessage(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		expected BlockerType
	}{
		{
			name:     "empty message",
			message:  "",
			expected: "",
		},
		{
			name:     "pdb violation",
			message:  "Cannot consolidate node because PDB would prevent eviction",
			expected: BlockerPDBViolation,
		},
		{
			name:     "local storage",
			message:  "Pod uses local storage and cannot be moved",
			expected: BlockerLocalStorage,
		},
		{
			name:     "non-replicated pod",
			message:  "Pod is non-replicated and has no controller",
			expected: BlockerNonReplicated,
		},
		{
			name:     "would increase cost",
			message:  "Consolidation would increase cost due to reserved instances",
			expected: BlockerWouldIncreaseCost,
		},
		{
			name:     "in-use security group",
			message:  "Node has in-use security group that cannot be removed",
			expected: BlockerInUseSecurityGroup,
		},
		{
			name:     "on-demand protection",
			message:  "Cannot consolidate on-demand node",
			expected: BlockerOnDemandProtection,
		},
		{
			name:     "do-not-consolidate annotation",
			message:  "Pod has do-not-consolidate annotation",
			expected: BlockerDoNotConsolidate,
		},
		{
			name:     "do-not-disrupt annotation",
			message:  "Pod has do-not-disrupt annotation set",
			expected: BlockerDoNotDisrupt,
		},
		{
			name:     "do-not-evict annotation",
			message:  "Pod has do-not-evict annotation",
			expected: BlockerDoNotEvict,
		},
		{
			name:     "unrecognized message",
			message:  "Some random event message",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeEventMessage(tt.message)
			if got != tt.expected {
				t.Errorf("NormalizeEventMessage() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDetectBlockers(t *testing.T) {
	tests := []struct {
		name         string
		pods         []corev1.Pod
		events       []corev1.Event
		cpuUtil      int
		memUtil      int
		podNames     map[string]bool
		wantBlockers []BlockerType
	}{
		{
			name:         "no blockers",
			pods:         nil,
			events:       nil,
			cpuUtil:      50,
			memUtil:      50,
			podNames:     nil,
			wantBlockers: nil,
		},
		{
			name:         "high cpu utilization",
			pods:         nil,
			events:       nil,
			cpuUtil:      85,
			memUtil:      50,
			podNames:     nil,
			wantBlockers: []BlockerType{BlockerHighUtilization},
		},
		{
			name:         "high memory utilization",
			pods:         nil,
			events:       nil,
			cpuUtil:      50,
			memUtil:      90,
			podNames:     nil,
			wantBlockers: []BlockerType{BlockerHighUtilization},
		},
		{
			name: "pod with blocking annotation",
			pods: []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "blocking-pod",
						Namespace: "default",
						Annotations: map[string]string{
							karpenter.AnnotationDoNotEvict: "true",
						},
					},
				},
			},
			events:       nil,
			cpuUtil:      50,
			memUtil:      50,
			podNames:     map[string]bool{"default/blocking-pod": true},
			wantBlockers: []BlockerType{BlockerDoNotEvict},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectBlockers(tt.pods, tt.events, tt.cpuUtil, tt.memUtil, tt.podNames)

			if len(got) != len(tt.wantBlockers) {
				t.Errorf("DetectBlockers() returned %d blockers, want %d", len(got), len(tt.wantBlockers))
				return
			}

			// Check that all expected blockers are present
			gotSet := make(map[BlockerType]bool)
			for _, b := range got {
				gotSet[b] = true
			}
			for _, want := range tt.wantBlockers {
				if !gotSet[want] {
					t.Errorf("DetectBlockers() missing blocker %v", want)
				}
			}
		})
	}
}

func TestFormatBlockers(t *testing.T) {
	tests := []struct {
		name     string
		blockers []BlockerType
		expected string
	}{
		{
			name:     "no blockers",
			blockers: nil,
			expected: "<none>",
		},
		{
			name:     "empty blockers",
			blockers: []BlockerType{},
			expected: "<none>",
		},
		{
			name:     "single blocker",
			blockers: []BlockerType{BlockerHighUtilization},
			expected: "high-utilization",
		},
		{
			name:     "multiple blockers",
			blockers: []BlockerType{BlockerHighUtilization, BlockerDoNotEvict},
			expected: "high-utilization,do-not-evict",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatBlockers(tt.blockers)
			if got != tt.expected {
				t.Errorf("FormatBlockers() = %v, want %v", got, tt.expected)
			}
		})
	}
}
