package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ssoriche/kubectl-consolidation/internal/consolidation"
	"github.com/ssoriche/kubectl-consolidation/internal/karpenter"
	"github.com/ssoriche/kubectl-consolidation/internal/kube"
	"github.com/ssoriche/kubectl-consolidation/internal/output"
)

var version = "dev"

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	var opts options

	cmd := &cobra.Command{
		Use:   "kubectl-consolidation [flags] [NODE...]",
		Short: "Show Karpenter consolidation blockers for nodes",
		Long: `Shows nodes with Karpenter consolidation blocker information including
nodepool/provisioner, capacity type, CPU/memory utilization, and reasons
why nodes cannot be consolidated.

Automatically detects Karpenter API version (v1alpha5, v1beta1, v1) and
adapts output accordingly. Supports mixed-version clusters during migrations.`,
		Example: `  # Show all nodes with consolidation information
  kubectl consolidation

  # Show specific nodes
  kubectl consolidation node-1 node-2

  # Filter nodes by label
  kubectl consolidation -l karpenter.sh/capacity-type=spot

  # Show detailed pod blockers for a node
  kubectl consolidation --pods node-1`,
		Version:      version,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd.Context(), args, opts)
		},
	}

	cmd.Flags().BoolVar(&opts.pods, "pods", false, "Show detailed pod-level blockers (requires node names)")
	cmd.Flags().StringVarP(&opts.selector, "selector", "l", "", "Label selector for nodes")
	cmd.Flags().StringVarP(&opts.output, "output", "o", "", "Output format (json, yaml)")
	cmd.Flags().BoolVar(&opts.noHeaders, "no-headers", false, "Don't print headers")

	return cmd
}

type options struct {
	pods      bool
	selector  string
	output    string
	noHeaders bool
}

func run(ctx context.Context, args []string, opts options) error {
	// Validate --pods requires node names
	if opts.pods && len(args) == 0 {
		return fmt.Errorf("--pods flag requires at least one node name")
	}

	// Create Kubernetes client
	client, err := kube.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Create discovery client for CRD detection
	discoveryClient, err := kube.NewDiscoveryClient()
	if err != nil {
		return fmt.Errorf("failed to create discovery client: %w", err)
	}

	// Detect Karpenter capabilities
	capabilities, err := karpenter.DetectCapabilities(ctx, discoveryClient)
	if err != nil {
		// Non-fatal: continue with empty capabilities
		capabilities = &karpenter.ClusterCapabilities{}
	}

	// Create collector and printer
	collector := consolidation.NewCollector(client, capabilities)
	printer := output.NewPrinter(capabilities, opts.output, opts.noHeaders)

	// Handle --pods mode
	if opts.pods {
		blockers, err := collector.CollectPodBlockers(ctx, args)
		if err != nil {
			return fmt.Errorf("failed to collect pod blockers: %w", err)
		}
		return printer.PrintPodBlockers(blockers)
	}

	// Default: show node table
	nodes, err := collector.Collect(ctx, args, opts.selector)
	if err != nil {
		return fmt.Errorf("failed to collect node information: %w", err)
	}

	return printer.PrintNodes(nodes)
}
