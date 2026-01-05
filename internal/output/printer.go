package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"gopkg.in/yaml.v3"

	"github.com/ssoriche/kubectl-consolidation/internal/consolidation"
	"github.com/ssoriche/kubectl-consolidation/internal/karpenter"
)

// Printer handles output formatting
type Printer struct {
	out          io.Writer
	noHeaders    bool
	outputFormat string
	capabilities *karpenter.ClusterCapabilities
}

// NewPrinter creates a new Printer
func NewPrinter(capabilities *karpenter.ClusterCapabilities, outputFormat string, noHeaders bool) *Printer {
	return &Printer{
		out:          os.Stdout,
		noHeaders:    noHeaders,
		outputFormat: outputFormat,
		capabilities: capabilities,
	}
}

// PrintNodes outputs node information in the requested format
func (p *Printer) PrintNodes(nodes []consolidation.NodeInfo) error {
	switch p.outputFormat {
	case "json":
		return p.printNodesJSON(nodes)
	case "yaml":
		return p.printNodesYAML(nodes)
	default:
		return p.printNodesTable(nodes)
	}
}

func (p *Printer) printNodesTable(nodes []consolidation.NodeInfo) error {
	w := tabwriter.NewWriter(p.out, 0, 0, 2, ' ', 0)

	poolHeader := p.capabilities.DeterminePoolColumnHeader()

	if !p.noHeaders {
		if _, err := fmt.Fprintf(w, "NAME\tSTATUS\tROLES\tAGE\tVERSION\t%s\tCAPACITY-TYPE\tCPU-UTIL\tMEM-UTIL\tCONSOLIDATION-BLOCKER\n", poolHeader); err != nil {
			return err
		}
	}

	for _, info := range nodes {
		node := info.Node
		status := consolidation.GetNodeStatus(node)
		roles := consolidation.GetNodeRoles(node)
		age := consolidation.FormatAge(node.CreationTimestamp.Time)
		version := node.Status.NodeInfo.KubeletVersion

		poolName := info.PoolName
		if poolName == "" {
			poolName = "<none>"
		}
		capacityType := info.CapacityType
		if capacityType == "" {
			capacityType = "<none>"
		}

		cpuUtil := consolidation.FormatUtilization(info.CPUUtilization)
		memUtil := consolidation.FormatUtilization(info.MemoryUtilization)
		blockers := consolidation.FormatBlockers(info.Blockers)

		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			node.Name, status, roles, age, version,
			poolName, capacityType, cpuUtil, memUtil, blockers); err != nil {
			return err
		}
	}

	return w.Flush()
}

type nodeOutput struct {
	Name              string   `json:"name" yaml:"name"`
	Status            string   `json:"status" yaml:"status"`
	Roles             string   `json:"roles" yaml:"roles"`
	Age               string   `json:"age" yaml:"age"`
	Version           string   `json:"version" yaml:"version"`
	PoolName          string   `json:"poolName" yaml:"poolName"`
	CapacityType      string   `json:"capacityType" yaml:"capacityType"`
	CPUUtilization    string   `json:"cpuUtilization" yaml:"cpuUtilization"`
	MemoryUtilization string   `json:"memoryUtilization" yaml:"memoryUtilization"`
	Blockers          []string `json:"blockers" yaml:"blockers"`
}

func (p *Printer) nodesToOutput(nodes []consolidation.NodeInfo) []nodeOutput {
	out := make([]nodeOutput, len(nodes))
	for i, info := range nodes {
		blockers := make([]string, len(info.Blockers))
		for j, b := range info.Blockers {
			blockers[j] = string(b)
		}

		out[i] = nodeOutput{
			Name:              info.Node.Name,
			Status:            consolidation.GetNodeStatus(info.Node),
			Roles:             consolidation.GetNodeRoles(info.Node),
			Age:               consolidation.FormatAge(info.Node.CreationTimestamp.Time),
			Version:           info.Node.Status.NodeInfo.KubeletVersion,
			PoolName:          info.PoolName,
			CapacityType:      info.CapacityType,
			CPUUtilization:    consolidation.FormatUtilization(info.CPUUtilization),
			MemoryUtilization: consolidation.FormatUtilization(info.MemoryUtilization),
			Blockers:          blockers,
		}
	}
	return out
}

func (p *Printer) printNodesJSON(nodes []consolidation.NodeInfo) error {
	out := p.nodesToOutput(nodes)
	encoder := json.NewEncoder(p.out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(out)
}

func (p *Printer) printNodesYAML(nodes []consolidation.NodeInfo) error {
	out := p.nodesToOutput(nodes)
	encoder := yaml.NewEncoder(p.out)
	encoder.SetIndent(2)
	return encoder.Encode(out)
}

// PrintPodBlockers outputs pod blocker information
func (p *Printer) PrintPodBlockers(blockers []consolidation.PodBlocker) error {
	switch p.outputFormat {
	case "json":
		return p.printPodBlockersJSON(blockers)
	case "yaml":
		return p.printPodBlockersYAML(blockers)
	default:
		return p.printPodBlockersTable(blockers)
	}
}

func (p *Printer) printPodBlockersTable(blockers []consolidation.PodBlocker) error {
	w := tabwriter.NewWriter(p.out, 0, 0, 2, ' ', 0)

	if !p.noHeaders {
		if _, err := fmt.Fprintln(w, "NODE\tNAMESPACE\tPOD\tAGE\tREASON"); err != nil {
			return err
		}
	}

	for _, b := range blockers {
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			b.NodeName, b.Namespace, b.PodName, b.Age, b.Reason); err != nil {
			return err
		}
	}

	return w.Flush()
}

type podBlockerOutput struct {
	NodeName  string `json:"nodeName" yaml:"nodeName"`
	Namespace string `json:"namespace" yaml:"namespace"`
	PodName   string `json:"podName" yaml:"podName"`
	Age       string `json:"age" yaml:"age"`
	Reason    string `json:"reason" yaml:"reason"`
}

func podBlockersToOutput(blockers []consolidation.PodBlocker) []podBlockerOutput {
	out := make([]podBlockerOutput, len(blockers))
	for i, b := range blockers {
		out[i] = podBlockerOutput{
			NodeName:  b.NodeName,
			Namespace: b.Namespace,
			PodName:   b.PodName,
			Age:       b.Age,
			Reason:    string(b.Reason),
		}
	}
	return out
}

func (p *Printer) printPodBlockersJSON(blockers []consolidation.PodBlocker) error {
	out := podBlockersToOutput(blockers)
	encoder := json.NewEncoder(p.out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(out)
}

func (p *Printer) printPodBlockersYAML(blockers []consolidation.PodBlocker) error {
	out := podBlockersToOutput(blockers)
	encoder := yaml.NewEncoder(p.out)
	encoder.SetIndent(2)
	return encoder.Encode(out)
}
