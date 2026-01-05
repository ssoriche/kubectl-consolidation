package karpenter

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDetectNodeVersion(t *testing.T) {
	tests := []struct {
		name     string
		node     *corev1.Node
		expected APIVersion
	}{
		{
			name:     "nil node",
			node:     nil,
			expected: APIVersionUnknown,
		},
		{
			name: "node with no labels",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
				},
			},
			expected: APIVersionUnknown,
		},
		{
			name: "v1beta1 node with nodepool label",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
					Labels: map[string]string{
						LabelNodePool: "default",
					},
				},
			},
			expected: APIVersionV1Beta1,
		},
		{
			name: "v1alpha5 node with provisioner label",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
					Labels: map[string]string{
						LabelProvisionerName: "default",
					},
				},
			},
			expected: APIVersionV1Alpha5,
		},
		{
			name: "node with both labels prefers v1beta1",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
					Labels: map[string]string{
						LabelNodePool:        "default",
						LabelProvisionerName: "legacy",
					},
				},
			},
			expected: APIVersionV1Beta1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectNodeVersion(tt.node)
			if got != tt.expected {
				t.Errorf("DetectNodeVersion() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGetPoolName(t *testing.T) {
	tests := []struct {
		name            string
		node            *corev1.Node
		expectedName    string
		expectedVersion APIVersion
	}{
		{
			name:            "nil node",
			node:            nil,
			expectedName:    "",
			expectedVersion: APIVersionUnknown,
		},
		{
			name: "v1beta1 nodepool",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						LabelNodePool: "my-nodepool",
					},
				},
			},
			expectedName:    "my-nodepool",
			expectedVersion: APIVersionV1Beta1,
		},
		{
			name: "v1alpha5 provisioner",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						LabelProvisionerName: "my-provisioner",
					},
				},
			},
			expectedName:    "my-provisioner",
			expectedVersion: APIVersionV1Alpha5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, version := GetPoolName(tt.node)
			if name != tt.expectedName {
				t.Errorf("GetPoolName() name = %v, want %v", name, tt.expectedName)
			}
			if version != tt.expectedVersion {
				t.Errorf("GetPoolName() version = %v, want %v", version, tt.expectedVersion)
			}
		})
	}
}

func TestGetCapacityType(t *testing.T) {
	tests := []struct {
		name     string
		node     *corev1.Node
		expected string
	}{
		{
			name:     "nil node",
			node:     nil,
			expected: "",
		},
		{
			name: "spot capacity",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						LabelCapacityType: "spot",
					},
				},
			},
			expected: "spot",
		},
		{
			name: "on-demand capacity",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						LabelCapacityType: "on-demand",
					},
				},
			},
			expected: "on-demand",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetCapacityType(tt.node)
			if got != tt.expected {
				t.Errorf("GetCapacityType() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestClusterCapabilities_DeterminePoolColumnHeader(t *testing.T) {
	tests := []struct {
		name         string
		capabilities ClusterCapabilities
		expected     string
	}{
		{
			name:         "no capabilities",
			capabilities: ClusterCapabilities{},
			expected:     "NODEPOOL",
		},
		{
			name: "v1beta1 nodepools",
			capabilities: ClusterCapabilities{
				HasNodePools: true,
			},
			expected: "NODEPOOL",
		},
		{
			name: "v1beta1 nodeclaims",
			capabilities: ClusterCapabilities{
				HasNodeClaims: true,
			},
			expected: "NODEPOOL",
		},
		{
			name: "v1alpha5 only",
			capabilities: ClusterCapabilities{
				HasProvisioners: true,
				HasMachines:     true,
			},
			expected: "PROVISIONER",
		},
		{
			name: "mixed cluster prefers nodepool",
			capabilities: ClusterCapabilities{
				HasNodePools:    true,
				HasProvisioners: true,
			},
			expected: "NODEPOOL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.capabilities.DeterminePoolColumnHeader()
			if got != tt.expected {
				t.Errorf("DeterminePoolColumnHeader() = %v, want %v", got, tt.expected)
			}
		})
	}
}
