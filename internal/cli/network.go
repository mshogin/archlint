package cli

import (
	"fmt"
	"os"
	"sort"

	"github.com/mshogin/archlint/internal/analyzer"
	"github.com/spf13/cobra"
)

var networkCmd = &cobra.Command{
	Use:   "network [directory]",
	Short: "Compute network metrics for the architecture graph",
	Long: `Analyze Go source code, build the architecture graph, and compute
advanced network/graph metrics including:

  - Betweenness Centrality (Brandes): identifies architectural bottlenecks
  - PageRank: identifies critical foundation nodes
  - Community Detection + Modularity: checks if clusters match packages
  - Clustering Coefficient: detects tightly coupled cliques
  - Average Shortest Path Length + Diameter: measures graph depth/flatness
  - Degree Distribution + Entropy: characterizes connectivity patterns
  - Small-World Coefficient: sigma = (C/C_rand) / (L/L_rand)

Examples:
  archlint network .
  archlint network ./internal`,
	Args: cobra.ExactArgs(1),
	RunE: runNetwork,
}

func init() {
	rootCmd.AddCommand(networkCmd)
}

func runNetwork(cmd *cobra.Command, args []string) error {
	codeDir := args[0]

	if _, err := os.Stat(codeDir); os.IsNotExist(err) {
		return fmt.Errorf("%w: %s", errDirNotExist, codeDir)
	}

	fmt.Printf("Analyzing architecture: %s\n", codeDir)

	var a *analyzer.GoAnalyzer
	if analyzer.DetectTypeScriptProject(codeDir) {
		tsAnalyzer := analyzer.NewTypeScriptAnalyzer()
		graph, err := tsAnalyzer.Analyze(codeDir)
		if err != nil {
			return fmt.Errorf("analysis error: %w", err)
		}
		fmt.Printf("Nodes: %d  Edges: %d\n\n", len(graph.Nodes), len(graph.Edges))
		nm := analyzer.ComputeNetworkMetrics(graph)
		printNetworkMetrics(nm)
	} else {
		a = analyzer.NewGoAnalyzer()
		graph, err := a.Analyze(codeDir)
		if err != nil {
			return fmt.Errorf("analysis error: %w", err)
		}
		fmt.Printf("Nodes: %d  Edges: %d\n\n", len(graph.Nodes), len(graph.Edges))
		nm := analyzer.ComputeNetworkMetrics(graph)
		printNetworkMetrics(nm)
	}

	return nil
}

func printNetworkMetrics(nm *analyzer.NetworkMetrics) {
	// --- Global metrics ---
	fmt.Println("=== Global Metrics ===")
	fmt.Printf("  Diameter:                     %d\n", nm.Diameter)
	fmt.Printf("  Average Shortest Path Length: %.4f\n", nm.AverageShortestPath)
	fmt.Printf("  Global Clustering Coefficient:%.4f\n", nm.GlobalClusteringCoefficient)
	fmt.Printf("  Small-World Coefficient (σ):  %.4f\n", nm.SmallWorldCoefficient)
	fmt.Printf("  Graph Entropy (H):            %.4f nats\n", nm.DegreeEntropy)
	fmt.Printf("  Modularity (Q):               %.4f\n", nm.Modularity)
	fmt.Println()

	// --- Degree distribution ---
	fmt.Println("=== Degree Distribution ===")
	if len(nm.DegreeHistogram) == 0 {
		fmt.Println("  (empty)")
	} else {
		keys := make([]int, 0, len(nm.DegreeHistogram))
		for k := range nm.DegreeHistogram {
			keys = append(keys, k)
		}
		sort.Ints(keys)
		for _, k := range keys {
			fmt.Printf("  degree %3d: %d node(s)\n", k, nm.DegreeHistogram[k])
		}
	}
	fmt.Println()

	// --- Top 10 betweenness centrality ---
	fmt.Println("=== Top Betweenness Centrality (bottlenecks) ===")
	printTopFloat(nm.BetweennessCentrality, 10, "%.6f")
	fmt.Println()

	// --- Top 10 PageRank ---
	fmt.Println("=== Top PageRank (critical foundations) ===")
	printTopFloat(nm.PageRank, 10, "%.6f")
	fmt.Println()

	// --- Communities ---
	fmt.Println("=== Detected Communities ===")
	if len(nm.Communities) == 0 {
		fmt.Println("  (none)")
	} else {
		// Group by community.
		grouped := make(map[int][]string)
		for node, com := range nm.Communities {
			grouped[com] = append(grouped[com], node)
		}
		comIDs := make([]int, 0, len(grouped))
		for c := range grouped {
			comIDs = append(comIDs, c)
		}
		sort.Ints(comIDs)
		for _, c := range comIDs {
			members := grouped[c]
			sort.Strings(members)
			if len(members) > 5 {
				fmt.Printf("  community %d (%d members): %v ... (+%d more)\n",
					c, len(members), members[:5], len(members)-5)
			} else {
				fmt.Printf("  community %d (%d members): %v\n", c, len(members), members)
			}
		}
	}
	fmt.Println()

	// --- Top 10 clustering coefficients ---
	fmt.Println("=== Top Clustering Coefficients (tightly coupled) ===")
	printTopFloat(nm.ClusteringCoefficient, 10, "%.4f")
}

// printTopFloat prints the top-n entries from a float map, sorted by value descending.
func printTopFloat(m map[string]float64, n int, format string) {
	type entry struct {
		key string
		val float64
	}
	entries := make([]entry, 0, len(m))
	for k, v := range m {
		entries = append(entries, entry{k, v})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].val != entries[j].val {
			return entries[i].val > entries[j].val
		}
		return entries[i].key < entries[j].key
	})
	if len(entries) == 0 {
		fmt.Println("  (empty)")
		return
	}
	limit := n
	if limit > len(entries) {
		limit = len(entries)
	}
	for i := 0; i < limit; i++ {
		fmt.Printf("  %-50s "+format+"\n", entries[i].key, entries[i].val)
	}
	if len(entries) > n {
		fmt.Printf("  ... (%d more)\n", len(entries)-n)
	}
}
