package visualize

import (
	"crypto/sha1"
	"encoding/hex"
	"sort"
	"strings"
)

func BuildPathGraph(lanes []BehaviorLane) *PathGraph {
	if len(lanes) == 0 {
		return &PathGraph{}
	}

	nodeByKey := map[string]int{}
	edgeByKey := map[string]int{}
	graph := &PathGraph{}

	for _, lane := range lanes {
		var previousNodeID string
		for i := range lane.Steps {
			step := lane.Steps[i]
			key := StepKey(step)
			if strings.TrimSpace(key) == "" {
				key = lane.RunID + ":" + step.StepID
			}
			nodeID := pathNodeID(key)
			if pos, ok := nodeByKey[key]; ok {
				graph.Nodes[pos].RunIDs = addRunID(graph.Nodes[pos].RunIDs, lane.RunID)
			} else {
				nodeByKey[key] = len(graph.Nodes)
				graph.Nodes = append(graph.Nodes, PathNode{
					ID:     nodeID,
					Label:  pathNodeLabel(step),
					Kind:   string(step.Kind),
					RunIDs: []string{lane.RunID},
				})
			}
			if previousNodeID != "" {
				edgeKey := previousNodeID + ">" + nodeID
				if pos, ok := edgeByKey[edgeKey]; ok {
					graph.Edges[pos].RunIDs = addRunID(graph.Edges[pos].RunIDs, lane.RunID)
				} else {
					edgeByKey[edgeKey] = len(graph.Edges)
					graph.Edges = append(graph.Edges, PathEdge{
						From:   previousNodeID,
						To:     nodeID,
						RunIDs: []string{lane.RunID},
					})
				}
			}
			previousNodeID = nodeID
		}
	}

	return graph
}

func pathNodeID(key string) string {
	sum := sha1.Sum([]byte(key))
	return "step-" + hex.EncodeToString(sum[:])[:12]
}

func pathNodeLabel(step VisualStep) string {
	if value := strings.TrimSpace(step.Summary); value != "" {
		return value
	}
	if value := strings.TrimSpace(step.Target); value != "" {
		return value
	}
	return string(step.Kind)
}

func addRunID(runIDs []string, runID string) []string {
	if strings.TrimSpace(runID) == "" {
		return runIDs
	}
	for _, existing := range runIDs {
		if existing == runID {
			return runIDs
		}
	}
	runIDs = append(runIDs, runID)
	sort.Strings(runIDs)
	return runIDs
}
