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

	// Fetch all node events in one call for efficiency
	eventsByNode, err := FetchAllNodeEvents(ctx, c.client)
	if err != nil {
		// Non-fatal: continue without events
		eventsByNode = make(map[string][]corev1.Event)
	}

	// Process nodes concurrently
	return c.collectParallel(ctx, nodes, eventsByNode)
}

const maxWorkers = 10

func (c *Collector) collectParallel(ctx context.Context, nodes []corev1.Node, eventsByNode map[string][]corev1.Event) ([]NodeInfo, error) {
	results := make([]NodeInfo, len(nodes))
	errs := make([]error, len(nodes))

	// Use a semaphore to limit concurrency
	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup

	for i := range nodes {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			info, err := c.collectNodeInfo(ctx, &nodes[idx], eventsByNode[nodes[idx].Name])
			if err != nil {
				errs[idx] = err
				return
			}
			results[idx] = info
		}(i)
	}

	wg.Wait()

	// Check for errors (return first error encountered)
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}

	return results, nil
}

func (c *Collector) collectNodeInfo(ctx context.Context, node *corev1.Node, events []corev1.Event) (NodeInfo, error) {
	info := NodeInfo{
		Node: node,
	}

	// Get Karpenter info
	info.PoolName, info.PoolVersion = karpenter.GetPoolName(node)
	info.CapacityType = karpenter.GetCapacityType(node)

	// Fetch pods on this node
	pods, err := FetchPodsOnNode(ctx, c.client, node.Name)
	if err != nil {
		return info, err
	}

	// Calculate utilization
	info.CPUUtilization, info.MemoryUtilization = CalculateUtilization(node, pods)

	// Build pod name set for event validation
	podNameSet := BuildPodNameSet(pods)

	// Detect blockers
	info.Blockers = DetectBlockers(pods, events, info.CPUUtilization, info.MemoryUtilization, podNameSet)

	return info, nil
}

// CollectPodBlockers gathers detailed pod blocker information for specific nodes
func (c *Collector) CollectPodBlockers(ctx context.Context, nodeNames []string) ([]PodBlocker, error) {
	var allBlockers []PodBlocker

	for _, nodeName := range nodeNames {
		pods, err := FetchPodsOnNode(ctx, c.client, nodeName)
		if err != nil {
			return nil, err
		}

		blockers := FindBlockingPods(pods, nodeName)
		allBlockers = append(allBlockers, blockers...)
	}

	return allBlockers, nil
}
