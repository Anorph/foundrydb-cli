package cmd

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/spf13/cobra"
)

// The platform-owned edge gateway PoPs (points of presence) are created,
// scaled, rolled, and retired only through the admin endpoints under
// /admin/edge. A PoP is one edge service with node_count >= 2 (a primary that
// holds the serving floating IP plus one or more hot standbys), giving in-house
// intra-PoP high availability via floating-IP handoff on failure. These
// commands require an admin account.

var edgeCmd = &cobra.Command{
	Use:   "edge",
	Short: "Administer the edge gateway fleet (admin only)",
	Long: `Administer the platform edge gateway fleet.

The edge tier is a set of points of presence (PoPs) that front app services.
Each PoP is one edge service sized at node_count >= 2 (a primary holding the
serving floating IP plus one or more hot standbys) for intra-PoP high
availability. These commands manage the fleet and require an admin account.`,
}

var edgeNodesCmd = &cobra.Command{
	Use:   "nodes",
	Short: "List, create, scale, and delete edge PoP nodes",
}

var edgeNodesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List the live edge fleet",
	Args:  cobra.NoArgs,
	RunE:  runEdgeNodesList,
}

var edgeNodesCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Provision a new edge PoP in a zone",
	Args:  cobra.NoArgs,
	RunE:  runEdgeNodesCreate,
}

var edgeNodesScaleCmd = &cobra.Command{
	Use:   "scale <node-id>",
	Short: "Set an edge PoP's desired VM count",
	Long: `Set an edge PoP's desired VM count.

Scaling up backfills standbys; scaling down retires the newest standbys (never
the primary). --count must be at least 1; 2 or more keeps the PoP highly
available. The PoP must be Running; the change converges asynchronously.`,
	Args: cobra.ExactArgs(1),
	RunE: runEdgeNodesScale,
}

var edgeNodesDeleteCmd = &cobra.Command{
	Use:   "delete <node-id>",
	Short: "Queue an edge PoP for deletion",
	Args:  cobra.ExactArgs(1),
	RunE:  runEdgeNodesDelete,
}

var edgeRollCmd = &cobra.Command{
	Use:   "roll",
	Short: "Drive a graceful one-node-at-a-time image roll of a PoP",
}

var edgeRollStartCmd = &cobra.Command{
	Use:   "start <node-id>",
	Short: "Begin a graceful roll of a PoP onto the current base template",
	Args:  cobra.ExactArgs(1),
	RunE:  runEdgeRollStart,
}

var edgeRollStatusCmd = &cobra.Command{
	Use:   "status <node-id>",
	Short: "Show a PoP's current roll progress",
	Args:  cobra.ExactArgs(1),
	RunE:  runEdgeRollStatus,
}

var edgeRollCancelCmd = &cobra.Command{
	Use:   "cancel <node-id>",
	Short: "Clear a PoP's roll marker so the reconciler stops retiring nodes",
	Args:  cobra.ExactArgs(1),
	RunE:  runEdgeRollCancel,
}

var edgeOverviewCmd = &cobra.Command{
	Use:   "overview",
	Short: "Show the consolidated read-only edge fleet snapshot",
	Args:  cobra.NoArgs,
	RunE:  runEdgeOverview,
}

var edgeRecoveryCmd = &cobra.Command{
	Use:   "recovery",
	Short: "Show the shared HA recovery telemetry snapshot",
	Args:  cobra.NoArgs,
	RunE:  runEdgeRecovery,
}

var edgeRoutesCmd = &cobra.Command{
	Use:   "routes",
	Short: "Show the apps routed through the edge and their per-PoP request rate",
	Args:  cobra.NoArgs,
	RunE:  runEdgeRoutes,
}

var (
	edgeCreateZone      string
	edgeCreatePlan      string
	edgeCreateNodeCount int
	edgeScaleCount      int
	edgeRoutesWindow    int
)

func init() {
	edgeNodesCreateCmd.Flags().StringVar(&edgeCreateZone, "zone", "", "UpCloud zone for the new PoP (required)")
	edgeNodesCreateCmd.Flags().StringVar(&edgeCreatePlan, "plan", "", "compute plan for the PoP VMs (optional; platform default applies when empty)")
	edgeNodesCreateCmd.Flags().IntVar(&edgeCreateNodeCount, "count", 0, "number of VMs in the PoP (optional; values below the primary-plus-standby default are raised to it)")
	_ = edgeNodesCreateCmd.MarkFlagRequired("zone")

	edgeNodesScaleCmd.Flags().IntVar(&edgeScaleCount, "count", 0, "desired VM count for the PoP (required; at least 1, 2+ keeps HA)")
	_ = edgeNodesScaleCmd.MarkFlagRequired("count")

	edgeRoutesCmd.Flags().IntVar(&edgeRoutesWindow, "window-minutes", 0, "aggregation window in minutes (0 uses the server default of 5; values above 1440 are clamped)")

	edgeNodesCmd.AddCommand(edgeNodesListCmd)
	edgeNodesCmd.AddCommand(edgeNodesCreateCmd)
	edgeNodesCmd.AddCommand(edgeNodesScaleCmd)
	edgeNodesCmd.AddCommand(edgeNodesDeleteCmd)

	edgeRollCmd.AddCommand(edgeRollStartCmd)
	edgeRollCmd.AddCommand(edgeRollStatusCmd)
	edgeRollCmd.AddCommand(edgeRollCancelCmd)

	edgeCmd.AddCommand(edgeNodesCmd)
	edgeCmd.AddCommand(edgeRollCmd)
	edgeCmd.AddCommand(edgeOverviewCmd)
	edgeCmd.AddCommand(edgeRecoveryCmd)
	edgeCmd.AddCommand(edgeRoutesCmd)

	rootCmd.AddCommand(edgeCmd)
}

// edgeAdminNode mirrors one admin PoP row from /admin/edge/nodes.
type edgeAdminNode struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Zone            string `json:"zone"`
	PlanName        string `json:"plan_name"`
	Status          string `json:"status"`
	NodeCount       int    `json:"node_count"`
	TargetNodeCount int    `json:"target_node_count"`
}

func runEdgeNodesList(cmd *cobra.Command, args []string) error {
	var resp struct {
		Nodes []edgeAdminNode `json:"nodes"`
	}
	if err := edgeGet("/admin/edge/nodes", &resp); err != nil {
		return err
	}
	if jsonOut {
		return printJSON(resp)
	}
	if len(resp.Nodes) == 0 {
		fmt.Println("No edge PoPs.")
		return nil
	}
	fmt.Printf("%-38s %-24s %-10s %-12s %-10s %s\n", "ID", "NAME", "ZONE", "STATUS", "PLAN", "NODES")
	for _, n := range resp.Nodes {
		fmt.Printf("%-38s %-24s %-10s %-12s %-10s %d/%d\n",
			n.ID, n.Name, n.Zone, n.Status, n.PlanName, n.NodeCount, n.TargetNodeCount)
	}
	return nil
}

func runEdgeNodesCreate(cmd *cobra.Command, args []string) error {
	body := map[string]interface{}{"zone": edgeCreateZone}
	if edgeCreatePlan != "" {
		body["plan_name"] = edgeCreatePlan
	}
	if edgeCreateNodeCount > 0 {
		body["node_count"] = edgeCreateNodeCount
	}
	var node edgeAdminNode
	if err := edgePost("/admin/edge/nodes", body, &node); err != nil {
		return err
	}
	if jsonOut {
		return printJSON(node)
	}
	fmt.Printf("Provisioning edge PoP %s in %s (status %s).\n", node.ID, node.Zone, node.Status)
	fmt.Printf("Poll progress with 'fdb edge nodes list' or 'fdb edge overview'.\n")
	return nil
}

func runEdgeNodesScale(cmd *cobra.Command, args []string) error {
	body := map[string]interface{}{"node_count": edgeScaleCount}
	if err := edgePatch(fmt.Sprintf("/admin/edge/nodes/%s", args[0]), body); err != nil {
		return err
	}
	fmt.Printf("Requested scale of edge PoP %s to %d node(s); the change converges asynchronously.\n", args[0], edgeScaleCount)
	return nil
}

func runEdgeNodesDelete(cmd *cobra.Command, args []string) error {
	if err := edgeDelete(fmt.Sprintf("/admin/edge/nodes/%s", args[0])); err != nil {
		return err
	}
	fmt.Printf("Queued edge PoP %s for deletion.\n", args[0])
	return nil
}

// edgeRollStatus mirrors the /admin/edge/nodes/{id}/roll response.
type edgeRollStatus struct {
	InProgress        bool    `json:"in_progress"`
	RequestedAt       *string `json:"requested_at,omitempty"`
	TargetNodes       int     `json:"target_nodes"`
	RunningNodes      int     `json:"running_nodes"`
	RemainingOldNodes int     `json:"remaining_old_nodes"`
	ReplacedNodes     int     `json:"replaced_nodes"`
}

func printEdgeRoll(nodeID string, s edgeRollStatus) {
	fmt.Printf("PoP:               %s\n", nodeID)
	fmt.Printf("Roll in progress:  %t\n", s.InProgress)
	if s.RequestedAt != nil && *s.RequestedAt != "" {
		fmt.Printf("Requested at:      %s\n", *s.RequestedAt)
	}
	fmt.Printf("Target nodes:      %d\n", s.TargetNodes)
	fmt.Printf("Running nodes:     %d\n", s.RunningNodes)
	fmt.Printf("Replaced nodes:    %d\n", s.ReplacedNodes)
	fmt.Printf("Remaining old:     %d\n", s.RemainingOldNodes)
}

func runEdgeRollStart(cmd *cobra.Command, args []string) error {
	var s edgeRollStatus
	if err := edgePost(fmt.Sprintf("/admin/edge/nodes/%s/roll", args[0]), nil, &s); err != nil {
		return err
	}
	if jsonOut {
		return printJSON(s)
	}
	printEdgeRoll(args[0], s)
	return nil
}

func runEdgeRollStatus(cmd *cobra.Command, args []string) error {
	var s edgeRollStatus
	if err := edgeGet(fmt.Sprintf("/admin/edge/nodes/%s/roll", args[0]), &s); err != nil {
		return err
	}
	if jsonOut {
		return printJSON(s)
	}
	printEdgeRoll(args[0], s)
	return nil
}

func runEdgeRollCancel(cmd *cobra.Command, args []string) error {
	if err := edgeDelete(fmt.Sprintf("/admin/edge/nodes/%s/roll", args[0])); err != nil {
		return err
	}
	fmt.Printf("Cleared the roll marker for edge PoP %s; any in-flight replacement completes normally.\n", args[0])
	return nil
}

func runEdgeOverview(cmd *cobra.Command, args []string) error {
	var resp struct {
		Autoscale struct {
			Enabled         bool    `json:"enabled"`
			MaxNodes        int     `json:"max_nodes"`
			ScaleUpRPS      float64 `json:"scale_up_rps"`
			ScaleDownRPS    float64 `json:"scale_down_rps"`
			CooldownSeconds int     `json:"cooldown_seconds"`
		} `json:"autoscale"`
		PoPs []struct {
			ID              string  `json:"id"`
			Name            string  `json:"name"`
			Zone            string  `json:"zone"`
			Status          string  `json:"status"`
			NodeCount       int     `json:"node_count"`
			TargetNodeCount int     `json:"target_node_count"`
			Deficit         int     `json:"deficit"`
			ServingFIP      *string `json:"serving_fip"`
			Recovery        *struct {
				InProgress bool   `json:"in_progress"`
				Phase      string `json:"phase"`
			} `json:"recovery"`
		} `json:"pops"`
	}
	if err := edgeGet("/admin/edge/overview", &resp); err != nil {
		return err
	}
	if jsonOut {
		return printJSON(resp)
	}
	fmt.Printf("Autoscale policy: enabled=%t max_nodes=%d scale_up_rps=%.1f scale_down_rps=%.1f cooldown=%ds\n\n",
		resp.Autoscale.Enabled, resp.Autoscale.MaxNodes, resp.Autoscale.ScaleUpRPS, resp.Autoscale.ScaleDownRPS, resp.Autoscale.CooldownSeconds)
	if len(resp.PoPs) == 0 {
		fmt.Println("No edge PoPs.")
		return nil
	}
	fmt.Printf("%-24s %-10s %-12s %-8s %-8s %-16s %s\n", "NAME", "ZONE", "STATUS", "NODES", "DEFICIT", "SERVING FIP", "RECOVERY")
	for _, p := range resp.PoPs {
		fip := "-"
		if p.ServingFIP != nil {
			fip = *p.ServingFIP
		}
		recovery := "-"
		if p.Recovery != nil && p.Recovery.InProgress {
			recovery = "in progress"
			if p.Recovery.Phase != "" {
				recovery = p.Recovery.Phase
			}
		}
		fmt.Printf("%-24s %-10s %-12s %d/%-6d %-8d %-16s %s\n",
			p.Name, p.Zone, p.Status, p.NodeCount, p.TargetNodeCount, p.Deficit, fip, recovery)
	}
	return nil
}

func runEdgeRecovery(cmd *cobra.Command, args []string) error {
	var resp struct {
		ByKind []struct {
			ServiceKind        string  `json:"service_kind"`
			Attempts           int     `json:"attempts"`
			Errors             int     `json:"errors"`
			AvgDurationSeconds float64 `json:"avg_duration_seconds"`
		} `json:"by_kind"`
		DeficitByService []struct {
			ServiceID string  `json:"service_id"`
			Deficit   float64 `json:"deficit"`
		} `json:"deficit_by_service"`
		ReconcilerTicks struct {
			Scanned         int `json:"scanned"`
			CandidatesFound int `json:"candidates_found"`
			QueryFailed     int `json:"query_failed"`
		} `json:"reconciler_ticks"`
	}
	if err := edgeGet("/admin/edge/recovery", &resp); err != nil {
		return err
	}
	if jsonOut {
		return printJSON(resp)
	}
	fmt.Printf("Reconciler ticks: scanned=%d candidates=%d query_failed=%d\n\n",
		resp.ReconcilerTicks.Scanned, resp.ReconcilerTicks.CandidatesFound, resp.ReconcilerTicks.QueryFailed)
	if len(resp.ByKind) > 0 {
		fmt.Printf("%-14s %-10s %-8s %s\n", "KIND", "ATTEMPTS", "ERRORS", "AVG DURATION (s)")
		for _, k := range resp.ByKind {
			fmt.Printf("%-14s %-10d %-8d %.1f\n", k.ServiceKind, k.Attempts, k.Errors, k.AvgDurationSeconds)
		}
	}
	if len(resp.DeficitByService) > 0 {
		fmt.Printf("\nPer-service node deficit:\n")
		for _, d := range resp.DeficitByService {
			fmt.Printf("  %s: %.0f\n", d.ServiceID, d.Deficit)
		}
	}
	return nil
}

func runEdgeRoutes(cmd *cobra.Command, args []string) error {
	path := "/admin/edge/routes"
	if edgeRoutesWindow > 0 {
		path += "?window_minutes=" + strconv.Itoa(edgeRoutesWindow)
	}
	var resp struct {
		WindowMinutes int `json:"window_minutes"`
		Apps          []struct {
			ServiceID      string  `json:"service_id"`
			Name           string  `json:"name"`
			Status         string  `json:"status"`
			Zone           string  `json:"zone"`
			RequestsPerSec float64 `json:"requests_per_sec"`
		} `json:"apps"`
	}
	if err := edgeGet(path, &resp); err != nil {
		return err
	}
	if jsonOut {
		return printJSON(resp)
	}
	fmt.Printf("Window: %d minute(s)\n\n", resp.WindowMinutes)
	if len(resp.Apps) == 0 {
		fmt.Println("No apps are currently routed through the edge.")
		return nil
	}
	fmt.Printf("%-38s %-24s %-10s %-10s %s\n", "SERVICE ID", "NAME", "ZONE", "STATUS", "REQ/S")
	for _, a := range resp.Apps {
		fmt.Printf("%-38s %-24s %-10s %-10s %.2f\n", a.ServiceID, a.Name, a.Zone, a.Status, a.RequestsPerSec)
	}
	return nil
}

// edgePatch sends a PATCH with body to path and ignores the response body.
func edgePatch(path string, body interface{}) error {
	_, err := edgeDoRequest(http.MethodPatch, path, body)
	return err
}
