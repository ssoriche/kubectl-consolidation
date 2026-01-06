package consolidation

import (
	"context"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/ssoriche/kubectl-consolidation/internal/karpenter"
)

// NodeInfo contains all consolidation-relevant information for a node
type NodeInfo struct {
	Node              *corev1.Node
	PoolName          string
	PoolVersion       karpenter.APIVersion
	CapacityType      string
	CPUUtilization    int
	MemoryUtilization int
	Blockers          []BlockerType
}

// Collector gathers consolidation data from the cluster
type Collector struct {
	client       kubernetes.Interface
	capabilities *karpenter.ClusterCapabilities
}

// NewCollector creates a new Collector
func NewCollector(client kubernetes.Interface, capabilities *karpenter.ClusterCapabilities) *Collector {
	return &Collector{
		client:       client,
		capabilities: capabilities,
	}
}

// Collect gathers consolidation data for nodes matching the criteria
func (c *Collector) Collect(ctx context.Context, nodeNames []string, selector string) ([]NodeInfo, error) {
	// Fetch nodes
	nodes, err := FetchNodes(ctx, c.client, nodeNames, selector)
	if err != nil {
		return nil, err
	}

	if len(nodes) == 0 {
		return nil, nil
	}

	// Fetch all pods and events in parallel (single API call each)
	var podsByNode map[string][]corev1.Pod
	var eventsByNode map[string][]corev1.Event
	var podErr, eventErr error

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		podsByNode, podErr = FetchAllPods(ctx, c.client)
	}()
	go func() {
		defer wg.Done()
		eventsByNode, eventErr = FetchAllNodeEvents(ctx, c.client)
	}()
	wg.Wait()

	if podErr != nil {
		return nil, podErr
	}
	if eventErr != nil {
		// Non-fatal: continue without events
		eventsByNode = make(map[string][]corev1.Event)
	}

	// Process nodes concurrently
	return c.collectParallel(nodes, podsByNode, eventsByNode)
}

const maxWorkers = 10

func (c *Collector) collectParallel(nodes []corev1.Node, podsByNode map[string][]corev1.Pod, eventsByNode map[string][]corev1.Event) ([]NodeInfo, error) {
	results := make([]NodeInfo, len(nodes))

	// Use a semaphore to limit concurrency
	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup

	for i := range nodes {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			nodeName := nodes[idx].Name
			results[idx] = c.collectNodeInfo(&nodes[idx], podsByNode[nodeName], eventsByNode[nodeName])
		}(i)
	}

	wg.Wait()

	return results, nil
}

func (c *Collector) collectNodeInfo(node *corev1.Node, pods []corev1.Pod, events []corev1.Event) NodeInfo {
	info := NodeInfo{
		Node: node,
	}

	// Get Karpenter info
	info.PoolName, info.PoolVersion = karpenter.GetPoolName(node)
	info.CapacityType = karpenter.GetCapacityType(node)

	// Calculate utilization
	info.CPUUtilization, info.MemoryUtilization = CalculateUtilization(node, pods)

	// Build pod name set for event validation
	podNameSet := BuildPodNameSet(pods)

	// Detect blockers
	info.Blockers = DetectBlockers(pods, events, info.CPUUtilization, info.MemoryUtilization, podNameSet)

	return info
}

// CollectPodBlockers gathers detailed pod blocker information for specific nodes
func (c *Collector) CollectPodBlockers(ctx context.Context, nodeNames []string) ([]PodBlocker, error) {
	// Build set of requested nodes for O(1) lookup
	nodeSet := make(map[string]bool, len(nodeNames))
	for _, name := range nodeNames {
		nodeSet[name] = true
	}

	// Fetch all pods once
	podsByNode, err := FetchAllPods(ctx, c.client)
	if err != nil {
		return nil, err
	}

	var allBlockers []PodBlocker
	for nodeName := range nodeSet {
		blockers := FindBlockingPods(podsByNode[nodeName], nodeName)
		allBlockers = append(allBlockers, blockers...)
	}

	return allBlockers, nil
}
