// Package archmotifbridge adapts archlint's architectural graph (internal/model)
// to archmotif's metric engine via the public github.com/mshogin/archmotif fork
// (pkg/archmotifimport.Builder -> pkg/archmotifmetrics.ComputeMetrics). It lets
// archlint reuse archmotif's research-grade metrics (Newman modularity Q, motif
// redundancy, anomaly detectors) WITHOUT GraphML or reflection — just an
// in-memory graph hand-off.
//
// A runtime fallback (variant A from the kgatilin-reflex-archmotif research:
// own modularity over the package import graph) keeps archlint working if the
// archmotif provider fails for any reason, so callers always get a Report.
package archmotifbridge

import (
	"errors"
	"fmt"
	"sort"

	"github.com/kgatilin/archmotif/pkg/archmotifimport"
	"github.com/kgatilin/archmotif/pkg/archmotifmetrics"
	"github.com/mshogin/archlint/internal/model"
)

var errNilGraph = errors.New("archmotifbridge: nil graph")

func msgf(format string, args ...any) string { return fmt.Sprintf(format, args...) }

// Anomaly is archlint's view of one archmotif-flagged region.
type Anomaly struct {
	Metric    string
	Code      string
	Message   string
	Score     float64
	PrimaryID string
}

// Report is the provider-agnostic metric result. Source identifies which
// provider produced it ("archmotif" or "fallback") so a caller can label
// degradation output honestly.
type Report struct {
	Source        string
	Modularity    float64
	HasModularity bool
	GraphMetrics  map[string]float64
	Anomalies     []Anomaly
	Notes         []string // mapping diagnostics: skipped nodes/edges, fallback reason
}

// MetricsProvider computes a Report from an archlint graph.
type MetricsProvider interface {
	Name() string
	Compute(g *model.Graph) (Report, error)
}

// Compute runs the archmotif provider and, on any error, falls back to the
// built-in provider. This is the entry point archlint callers should use.
func Compute(g *model.Graph) Report {
	primary := ArchmotifProvider{}
	rep, err := primary.Compute(g)
	if err == nil {
		return rep
	}
	fb := FallbackProvider{}
	frep, ferr := fb.Compute(g)
	frep.Notes = append([]string{"archmotif provider unavailable: " + err.Error()}, frep.Notes...)
	if ferr != nil {
		frep.Notes = append(frep.Notes, "fallback also failed: "+ferr.Error())
	}
	return frep
}

// ---------------------------------------------------------------------------
// archmotif provider: model.Graph -> archmotifimport.Builder -> ComputeMetrics
// ---------------------------------------------------------------------------

// ArchmotifProvider computes metrics through the archmotif fork.
type ArchmotifProvider struct{}

func (ArchmotifProvider) Name() string { return "archmotif" }

// edgeKindMap maps archlint edge types to archmotif dependency kinds. Structural
// "contains" is handled by Builder's Add* parenting; field_read/field_write are
// dropped (archmotif's vocabulary stops at the symbol level for these).
var edgeKindMap = map[string]archmotifimport.DependencyKind{
	model.EdgeImport: archmotifimport.DependencyDependsOn,
	model.EdgeCalls:  archmotifimport.DependencyCalls,
	model.EdgeUses:   archmotifimport.DependencyReferences,
	model.EdgeEmbeds: archmotifimport.DependencyEmbeds,
}

func (ArchmotifProvider) Compute(g *model.Graph) (Report, error) {
	rep := Report{Source: "archmotif", GraphMetrics: map[string]float64{}}
	if g == nil {
		return rep, errNilGraph
	}

	b := archmotifimport.NewBuilder()

	// Index nodes by ID and resolve structural parents from `contains` edges
	// (From = parent, To = child).
	entity := make(map[string]string, len(g.Nodes))
	for _, n := range g.Nodes {
		entity[n.ID] = n.Entity
	}
	parent := make(map[string]string)
	for _, e := range g.Edges {
		if e.Type == model.EdgeContains {
			parent[e.To] = e.From
		}
	}

	added := make(map[string]bool, len(g.Nodes))
	skip := 0
	note := func() { skip++ }

	// Tier 1: packages and external packages.
	for _, n := range g.Nodes {
		if n.Entity == model.EntityPackage || n.Entity == model.EntityExternal {
			if err := b.AddPackage(n.ID, "", ""); err != nil {
				note()
				continue
			}
			added[n.ID] = true
		}
	}
	// Tier 2: types (struct/interface) under their package.
	for _, n := range g.Nodes {
		if n.Entity != model.EntityStruct && n.Entity != model.EntityInterface {
			continue
		}
		pkg := parent[n.ID]
		if !added[pkg] {
			note()
			continue
		}
		if err := b.AddType(n.ID, pkg, n.Entity == model.EntityInterface, ""); err != nil {
			note()
			continue
		}
		added[n.ID] = true
	}
	// Tier 3: functions under their package.
	for _, n := range g.Nodes {
		if n.Entity != model.EntityFunction {
			continue
		}
		pkg := parent[n.ID]
		if !added[pkg] {
			note()
			continue
		}
		if err := b.AddFunction(n.ID, pkg); err != nil {
			note()
			continue
		}
		added[n.ID] = true
	}
	// Tier 4: methods under their receiver type.
	for _, n := range g.Nodes {
		if n.Entity != model.EntityMethod {
			continue
		}
		typ := parent[n.ID]
		if !added[typ] {
			note()
			continue
		}
		if err := b.AddMethod(n.ID, typ); err != nil {
			note()
			continue
		}
		added[n.ID] = true
	}
	// Tier 5: fields under their struct.
	for _, n := range g.Nodes {
		if n.Entity != model.EntityField {
			continue
		}
		st := parent[n.ID]
		if !added[st] {
			note()
			continue
		}
		if err := b.AddField(n.ID, st, ""); err != nil {
			note()
			continue
		}
		added[n.ID] = true
	}

	// Edges: map non-structural edges; skip when an endpoint was not added.
	edgeSkip := 0
	for _, e := range g.Edges {
		kind, ok := edgeKindMap[e.Type]
		if !ok {
			continue // contains / field_read / field_write
		}
		if !added[e.From] || !added[e.To] {
			edgeSkip++
			continue
		}
		if err := b.AddDependency(e.From, e.To, kind); err != nil {
			edgeSkip++
		}
	}

	g2, err := b.Build()
	if err != nil {
		return rep, err
	}
	m, err := archmotifmetrics.ComputeMetrics(g2)
	if err != nil {
		return rep, err
	}

	rep.Modularity = m.Modularity
	rep.HasModularity = m.HasModularity
	for _, gmv := range m.Graph {
		rep.GraphMetrics[gmv.Metric] = gmv.Value
	}
	for _, a := range m.Anomalies {
		rep.Anomalies = append(rep.Anomalies, Anomaly{
			Metric: a.Metric, Code: a.Code, Message: a.Message,
			Score: a.Score, PrimaryID: a.PrimaryID,
		})
	}
	if skip > 0 {
		rep.Notes = append(rep.Notes, msgf("skipped %d node(s) with missing parent", skip))
	}
	if edgeSkip > 0 {
		rep.Notes = append(rep.Notes, msgf("skipped %d edge(s) with unmapped endpoint", edgeSkip))
	}
	rep.Notes = append(rep.Notes, msgf("archmotif metricsRan=%v detectorsRan=%v", m.MetricsRan, m.DetectorsRan))
	return rep, nil
}

// ---------------------------------------------------------------------------
// fallback provider: own Newman modularity over the package import graph
// (variant A seed — keeps archlint self-sufficient if archmotif is absent)
// ---------------------------------------------------------------------------

// FallbackProvider computes a built-in modularity over package import edges via
// label-propagation communities. Deliberately small and dependency-free.
type FallbackProvider struct{}

func (FallbackProvider) Name() string { return "fallback" }

func (FallbackProvider) Compute(g *model.Graph) (Report, error) {
	rep := Report{Source: "fallback", GraphMetrics: map[string]float64{}}
	if g == nil {
		return rep, errNilGraph
	}
	// Undirected package adjacency from import edges between package nodes.
	isPkg := map[string]bool{}
	for _, n := range g.Nodes {
		if n.Entity == model.EntityPackage || n.Entity == model.EntityExternal {
			isPkg[n.ID] = true
		}
	}
	adj := map[string]map[string]float64{}
	deg := map[string]float64{}
	var m2 float64 // 2x total edge weight
	addEdge := func(a, c string) {
		if a == c {
			return
		}
		if adj[a] == nil {
			adj[a] = map[string]float64{}
		}
		if adj[c] == nil {
			adj[c] = map[string]float64{}
		}
		adj[a][c]++
		adj[c][a]++
		deg[a]++
		deg[c]++
		m2 += 2
	}
	for _, e := range g.Edges {
		if e.Type == model.EdgeImport && isPkg[e.From] && isPkg[e.To] {
			addEdge(e.From, e.To)
		}
	}
	if m2 == 0 {
		rep.Notes = append(rep.Notes, "no package import edges; modularity undefined")
		return rep, nil
	}

	// Label propagation: start each node in its own community, iterate to a
	// fixed point (bounded), assigning each node the most-weighted neighbour label.
	comm := map[string]string{}
	nodes := make([]string, 0, len(deg))
	for n := range deg {
		comm[n] = n
		nodes = append(nodes, n)
	}
	sort.Strings(nodes) // determinism
	for iter := 0; iter < 20; iter++ {
		changed := false
		for _, n := range nodes {
			best, bestW := comm[n], -1.0
			tally := map[string]float64{}
			for nb, w := range adj[n] {
				tally[comm[nb]] += w
			}
			labels := make([]string, 0, len(tally))
			for l := range tally {
				labels = append(labels, l)
			}
			sort.Strings(labels)
			for _, l := range labels {
				if tally[l] > bestW {
					bestW, best = tally[l], l
				}
			}
			if best != comm[n] {
				comm[n] = best
				changed = true
			}
		}
		if !changed {
			break
		}
	}

	// Newman Q = sum_ij [ A_ij/2m - (k_i k_j)/(2m)^2 ] * delta(c_i,c_j).
	var q float64
	for _, i := range nodes {
		for _, j := range nodes {
			if comm[i] != comm[j] {
				continue
			}
			a := adj[i][j]
			q += a/m2 - (deg[i]*deg[j])/(m2*m2)
		}
	}
	rep.Modularity = q
	rep.HasModularity = true
	rep.GraphMetrics["modularity"] = q
	rep.Notes = append(rep.Notes, "fallback modularity via label-propagation communities")
	return rep, nil
}
