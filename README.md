# kubectl-consolidation

A kubectl plugin that shows Karpenter consolidation blockers for nodes.

## Features

- Shows nodes with consolidation blocker information
- Displays NODEPOOL/PROVISIONER, CAPACITY-TYPE, CPU-UTIL, MEM-UTIL columns
- Automatically detects Karpenter API version (v1alpha5, v1beta1, v1)
- Supports mixed-version clusters during migrations
- Shows blocking pods with `--pods` flag
- Outputs in table, JSON, or YAML format

## Installation

### Via krew (recommended)

```bash
kubectl krew install consolidation
```

### Manual installation

Download the appropriate binary from the [releases page](https://github.com/ssoriche/kubectl-consolidation/releases), extract it, and place it in your PATH.

```bash
# Example for macOS (Apple Silicon)
tar -xzf kubectl-consolidation_darwin_arm64.tar.gz
chmod +x kubectl-consolidation
sudo mv kubectl-consolidation /usr/local/bin/
```

## Usage

```bash
# Show all nodes with consolidation information
kubectl consolidation

# Show specific nodes
kubectl consolidation node-1 node-2

# Filter nodes by label
kubectl consolidation -l karpenter.sh/capacity-type=spot

# Show detailed pod blockers for a node
kubectl consolidation --pods node-1

# Output as JSON
kubectl consolidation -o json

# Output as YAML
kubectl consolidation -o yaml
```

## Output Example

```
NAME                          STATUS   ROLES    AGE   VERSION   NODEPOOL   CAPACITY-TYPE   CPU-UTIL   MEM-UTIL   CONSOLIDATION-BLOCKER
ip-10-0-1-100.ec2.internal    Ready    <none>   5d    v1.28.0   default    spot            45%        62%        <none>
ip-10-0-1-101.ec2.internal    Ready    <none>   3d    v1.28.0   default    spot            82%        71%        high-utilization
ip-10-0-1-102.ec2.internal    Ready    <none>   1d    v1.28.0   default    on-demand       55%        48%        do-not-evict
```

## Blocker Types

| Blocker | Description |
|---------|-------------|
| `high-utilization` | Node CPU or memory utilization >= 80% |
| `do-not-evict` | Pod has `karpenter.sh/do-not-evict` annotation |
| `do-not-disrupt` | Pod has `karpenter.sh/do-not-disrupt` annotation |
| `do-not-consolidate` | Node/Pod has `do-not-consolidate` annotation |
| `pdb-violation` | PodDisruptionBudget prevents disruption |
| `non-replicated` | Pod has no controller (standalone) |
| `local-storage` | Pod uses local storage |
| `would-increase-cost` | Consolidation would increase costs |
| `in-use-security-group` | Node security group in use |
| `on-demand-protection` | Would delete on-demand node |

## Karpenter Version Support

The plugin automatically detects which Karpenter version is installed:

| Version | Node Labels | Column Header |
|---------|-------------|---------------|
| v1alpha5 | `karpenter.sh/provisioner-name` | PROVISIONER |
| v1beta1/v1 | `karpenter.sh/nodepool` | NODEPOOL |

Mixed-version clusters are supported during migrations.

## Development

```bash
# Enter devbox shell
devbox shell

# Build
make build

# Run tests
make test

# Run linter
make lint

# Build for all platforms
make build-all
```

## License

MIT
