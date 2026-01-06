# Performance Optimizations Design

## Overview

This design addresses three high/medium impact performance issues identified in the codebase:

1. **Per-node pod API calls** (High) - N API calls reduced to 1
2. **Sequential CollectPodBlockers** (Medium) - Add batch fetching
3. **Regex recompilation** (Medium) - Compile once at init

## 1. Batch Pod Fetching

### Current State

`collector.go:110` calls `FetchPodsOnNode()` for each node inside `collectNodeInfo()`. On a 100-node cluster, this means 100 API calls to the kube-apiserver.

### Design

Add `FetchAllPods()` to `pods.go`:

```go
// FetchAllPods retrieves all pods and groups them by node name
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
```

### Changes to collector.go

1. `Collect()` calls `FetchAllPods()` early, alongside `FetchAllNodeEvents()`
2. `collectParallel()` signature changes to accept `podsByNode map[string][]corev1.Pod`
3. `collectNodeInfo()` signature changes to accept `pods []corev1.Pod` directly instead of fetching

### Updated Collect Flow

```go
func (c *Collector) Collect(ctx context.Context, nodeNames []string, selector string) ([]NodeInfo, error) {
    nodes, err := FetchNodes(ctx, c.client, nodeNames, selector)
    if err != nil {
        return nil, err
    }
    if len(nodes) == 0 {
        return nil, nil
    }

    // Fetch all pods and events in parallel (both are independent)
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
        eventsByNode = make(map[string][]corev1.Event)
    }

    return c.collectParallel(ctx, nodes, podsByNode, eventsByNode)
}
```

### Updated collectParallel

```go
func (c *Collector) collectParallel(
    ctx context.Context,
    nodes []corev1.Node,
    podsByNode map[string][]corev1.Pod,
    eventsByNode map[string][]corev1.Event,
) ([]NodeInfo, error) {
    // ... same structure, but pass pods to collectNodeInfo
    info, err := c.collectNodeInfo(&nodes[idx], podsByNode[nodes[idx].Name], eventsByNode[nodes[idx].Name])
}
```

### Updated collectNodeInfo

```go
func (c *Collector) collectNodeInfo(node *corev1.Node, pods []corev1.Pod, events []corev1.Event) NodeInfo {
    // No longer needs ctx or makes API calls
    // ... rest of logic unchanged
}
```

## 2. Batch CollectPodBlockers

### Current State

`CollectPodBlockers()` iterates sequentially through node names, calling `FetchPodsOnNode()` for each.

### Design

Fetch all pods once, then filter by the requested nodes:

```go
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
        pods := podsByNode[nodeName]
        blockers := FindBlockingPods(pods, nodeName)
        allBlockers = append(allBlockers, blockers...)
    }

    return allBlockers, nil
}
```

## 3. Regex Compilation at Init

### Current State

`NormalizeEventMessage()` in `blockers.go:67-80` compiles 9 regexes on every call.

### Design

Move to package-level initialization:

```go
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
```

## Implementation Order

1. **Regex fix** (blockers.go) - Smallest change, no signature changes
2. **FetchAllPods** (pods.go) - Add new function
3. **Update Collect/collectParallel/collectNodeInfo** (collector.go) - Wire up batch fetching
4. **Update CollectPodBlockers** (collector.go) - Use batch fetching

## Testing

- Existing unit tests should continue to pass
- Run `go test ./...` after each change
- Manual verification on a test cluster if available

## Files Modified

- `internal/consolidation/blockers.go` - Regex init
- `internal/consolidation/pods.go` - Add FetchAllPods
- `internal/consolidation/collector.go` - Update signatures and flow
