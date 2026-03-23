package mcp

import (
	"sync"

	"github.com/mshogin/archlint/internal/analyzer"
	"github.com/mshogin/archlint/internal/model"
)

// DegradationReport describes the health change of a file between two snapshots.
type DegradationReport struct {
	FilePath        string      `json:"filePath"`
	HealthBefore    int         `json:"healthBefore"`
	HealthAfter     int         `json:"healthAfter"`
	Delta           int         `json:"delta"` // positive = improved
	NewViolations   []Violation `json:"newViolations,omitempty"`
	FixedViolations []Violation `json:"fixedViolations,omitempty"`
	Status          string      `json:"status"` // "improved", "stable", "degraded", "critical"
}

// DegradationDetector tracks metrics snapshots and detects degradation.
type DegradationDetector struct {
	mu        sync.RWMutex
	baselines map[string]*FileMetrics
}

// NewDegradationDetector creates a new degradation detector.
func NewDegradationDetector() *DegradationDetector {
	return &DegradationDetector{
		baselines: make(map[string]*FileMetrics),
	}
}

// SetBaselines stores metric baselines for all files.
func (d *DegradationDetector) SetBaselines(metrics map[string]*FileMetrics) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.baselines = make(map[string]*FileMetrics, len(metrics))
	for k, v := range metrics {
		d.baselines[k] = v
	}
}

// SetBaseline stores a baseline for a single file.
func (d *DegradationDetector) SetBaseline(path string, m *FileMetrics) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.baselines[path] = m
}

// GetBaseline returns the stored baseline for a file.
func (d *DegradationDetector) GetBaseline(path string) *FileMetrics {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.baselines[path]
}

// Check computes the degradation report for a file by comparing the current
// metrics against the stored baseline.
func (d *DegradationDetector) Check(path string, a *analyzer.GoAnalyzer, graph *model.Graph) *DegradationReport {
	current := ComputeFileMetrics(path, a, graph)

	d.mu.RLock()
	baseline := d.baselines[path]
	d.mu.RUnlock()

	report := &DegradationReport{
		FilePath:    path,
		HealthAfter: current.HealthScore,
	}

	if baseline == nil {
		report.HealthBefore = current.HealthScore
		report.Delta = 0
		report.Status = "stable"
	} else {
		report.HealthBefore = baseline.HealthScore
		report.Delta = current.HealthScore - baseline.HealthScore
		report.NewViolations = findNewViolations(baseline, current)
		report.FixedViolations = findNewViolations(current, baseline)
		report.Status = classifyDelta(report.Delta)
	}

	// Update the baseline to the current snapshot.
	d.mu.Lock()
	d.baselines[path] = current
	d.mu.Unlock()

	return report
}

// CheckWithoutUpdate computes the degradation report without updating the baseline.
func (d *DegradationDetector) CheckWithoutUpdate(path string, a *analyzer.GoAnalyzer, graph *model.Graph) *DegradationReport {
	current := ComputeFileMetrics(path, a, graph)

	d.mu.RLock()
	baseline := d.baselines[path]
	d.mu.RUnlock()

	report := &DegradationReport{
		FilePath:    path,
		HealthAfter: current.HealthScore,
	}

	if baseline == nil {
		report.HealthBefore = current.HealthScore
		report.Delta = 0
		report.Status = "stable"
	} else {
		report.HealthBefore = baseline.HealthScore
		report.Delta = current.HealthScore - baseline.HealthScore
		report.NewViolations = findNewViolations(baseline, current)
		report.FixedViolations = findNewViolations(current, baseline)
		report.Status = classifyDelta(report.Delta)
	}

	return report
}

func classifyDelta(delta int) string {
	switch {
	case delta > 5:
		return "improved"
	case delta >= -5:
		return "stable"
	case delta >= -15:
		return "degraded"
	default:
		return "critical"
	}
}

// findNewViolations returns violations present in 'after' but not in 'before'.
func findNewViolations(before, after *FileMetrics) []Violation {
	beforeSet := make(map[string]bool)

	for _, v := range allViolations(before) {
		beforeSet[violationKey(v)] = true
	}

	var newOnes []Violation

	for _, v := range allViolations(after) {
		if !beforeSet[violationKey(v)] {
			newOnes = append(newOnes, v)
		}
	}

	return newOnes
}

func allViolations(m *FileMetrics) []Violation {
	var all []Violation
	all = append(all, m.SRPViolations...)
	all = append(all, m.DIPViolations...)
	all = append(all, m.ISPViolations...)

	for _, name := range m.GodClasses {
		all = append(all, Violation{Kind: "god-class", Target: name})
	}

	for _, name := range m.HubNodes {
		all = append(all, Violation{Kind: "hub-node", Target: name})
	}

	for _, name := range m.FeatureEnvy {
		all = append(all, Violation{Kind: "feature-envy", Target: name})
	}

	for _, name := range m.ShotgunSurgery {
		all = append(all, Violation{Kind: "shotgun-surgery", Target: name})
	}

	for _, cycle := range m.CyclicDeps {
		all = append(all, Violation{Kind: "circular-dependency", Message: cycle})
	}

	return all
}

func violationKey(v Violation) string {
	return v.Kind + "|" + v.Target + "|" + v.Message
}
