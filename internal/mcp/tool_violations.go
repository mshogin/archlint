package mcp

import (
	"encoding/json"
	"fmt"
	"path/filepath"
)

// handleCheckViolations implements the check_violations tool.
func handleCheckViolations(state *State, args json.RawMessage) (*ViolationReport, error) {
	var params struct {
		Path string `json:"path"`
	}

	if args != nil {
		if err := json.Unmarshal(args, &params); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
	}

	graph := state.GetGraph()
	a := state.GetAnalyzer()

	report := &ViolationReport{}

	if params.Path != "" {
		if a == nil {
			return nil, fmt.Errorf("analyzer not initialized")
		}

		// Determine package from path.
		target := params.Path

		absPath, err := filepath.Abs(params.Path)
		if err == nil {
			for _, typeInfo := range a.AllTypes() {
				if matchesFile(typeInfo.File, absPath) {
					target = typeInfo.Package

					break
				}
			}
		}

		// Classic violations (coupling, cycles).
		report.Violations = DetectViolationsForPackage(graph, target)

		// Rich per-file metrics including SOLID, smells.
		metrics := ComputeFileMetrics(params.Path, a, graph)
		report.FileMetrics = metrics

		// Merge metrics-derived violations into the report.
		report.Violations = append(report.Violations, metrics.SRPViolations...)
		report.Violations = append(report.Violations, metrics.DIPViolations...)
		report.Violations = append(report.Violations, metrics.ISPViolations...)

		for _, gc := range metrics.GodClasses {
			report.Violations = append(report.Violations, Violation{
				Kind:    "god-class",
				Message: fmt.Sprintf("God class detected: %s", gc),
				Target:  gc,
			})
		}

		for _, hub := range metrics.HubNodes {
			report.Violations = append(report.Violations, Violation{
				Kind:    "hub-node",
				Message: fmt.Sprintf("Hub node detected (fan-in + fan-out > %d): %s", hubThreshold, hub),
				Target:  hub,
			})
		}

		for _, fe := range metrics.FeatureEnvy {
			report.Violations = append(report.Violations, Violation{
				Kind:    "feature-envy",
				Message: fmt.Sprintf("Feature envy detected: %s calls more methods on other types than its own receiver", fe),
				Target:  fe,
			})
		}

		for _, ss := range metrics.ShotgunSurgery {
			report.Violations = append(report.Violations, Violation{
				Kind:    "shotgun-surgery",
				Message: fmt.Sprintf("Shotgun surgery risk: changes to %s would affect >%d files", ss, shotgunThreshold),
				Target:  ss,
			})
		}

		return report, nil
	}

	// Check all packages.
	report.Violations = DetectAllViolations(graph)

	return report, nil
}
