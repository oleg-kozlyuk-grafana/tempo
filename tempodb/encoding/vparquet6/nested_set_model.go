package vparquet6

import (
	"sort"

	v1 "github.com/grafana/tempo/pkg/tempopb/trace/v1"

	"github.com/grafana/tempo/pkg/util"
)

// nestedSetRootParent is used for the root span's ParentID field. this allows the fetch layer (and
// other code) to distinguish between situations in which a span's parent is unknown due to a broken trace
// or simply known to not exist.
const nestedSetRootParent = -1

// spanNode is a wrapper around a flat span that is used to build and traverse spans as a tree.
type spanNode struct {
	parent    *spanNode
	span      *FlatSpan
	children  []*spanNode
	nextChild int
}

// assignNestedSetModelBounds calculates and assigns the values NestedSetLeft, NestedSetRight,
// ParentID, and ChildCount for all flat spans.
// Returns true if the trace tree is a connected graph.
func assignNestedSetModelBounds(spans []FlatSpan) bool {
	if len(spans) == 0 {
		return false
	}

	// find root spans and map span IDs to tree nodes
	var (
		undoAssignment bool
		allNodes       = make([]spanNode, 0, len(spans))
		nodesByID      = make(map[uint64][]*spanNode, len(spans))
		rootNodes      []*spanNode
	)

	// Compute per-service statistics while iterating spans
	serviceStatsMap := map[string]*ServiceStats{}

	for i := range spans {
		s := &spans[i]
		allNodes = append(allNodes, spanNode{span: s})
		node := &allNodes[len(allNodes)-1]

		if s.IsRoot() {
			rootNodes = append(rootNodes, node)
		}

		id := util.SpanIDToUint64(s.SpanID)
		if nodes, ok := nodesByID[id]; ok {
			// zipkin traces may contain client/server spans with the same IDs
			nodes = append(nodes, node)
			nodesByID[id] = nodes
			if len(nodes) > 2 {
				undoAssignment = true
			}
		} else {
			nodesByID[id] = []*spanNode{node}
		}

		// Accumulate service stats
		svcName := s.ResourceServiceName
		ss, ok := serviceStatsMap[svcName]
		if !ok {
			ss = &ServiceStats{ServiceName: svcName}
			serviceStatsMap[svcName] = ss
		}
		ss.SpanCount++
		if s.StatusCode == int(v1.Status_STATUS_CODE_ERROR) {
			ss.ErrorCount++
		}
	}

	// Build sorted ServiceStats slice and assign to all spans
	serviceStats := make([]ServiceStats, 0, len(serviceStatsMap))
	for _, ss := range serviceStatsMap {
		serviceStats = append(serviceStats, *ss)
	}
	sort.Slice(serviceStats, func(i, j int) bool {
		return serviceStats[i].ServiceName < serviceStats[j].ServiceName
	})
	for i := range spans {
		spans[i].ServiceStats = serviceStats
	}

	// check preconditions before assignment
	if len(rootNodes) == 0 {
		return false
	}
	if undoAssignment {
		for _, nodes := range nodesByID {
			for _, n := range nodes {
				n.span.NestedSetLeft = 0
				n.span.NestedSetRight = 0
				n.span.ParentID = 0
				n.span.ChildCount = 0
			}
		}
		return false
	}

	connected := true
	// build the tree
	for i := range allNodes {
		node := &allNodes[i]
		parent := findParentNodeInMap(nodesByID, node)
		if parent == nil {
			if !node.span.IsRoot() {
				connected = false
			}
			continue
		}
		node.parent = parent
		parent.children = append(parent.children, node)
	}

	// assign child count for each node
	for i := range allNodes {
		allNodes[i].span.ChildCount = int32(len(allNodes[i].children))
	}

	// traverse the tree depth first. When going down the tree, assign NestedSetLeft
	// and assign NestedSetRight when going up.
	nestedSetBound := int32(1)
	for _, root := range rootNodes {
		node := root
		node.span.NestedSetLeft = nestedSetBound
		node.span.ParentID = nestedSetRootParent
		nestedSetBound++

		for node != nil {
			if node.nextChild < len(node.children) {
				next := node.children[node.nextChild]
				node.nextChild++

				next.span.NestedSetLeft = nestedSetBound
				next.span.ParentID = node.span.NestedSetLeft
				nestedSetBound++
				node = next
			} else {
				node.span.NestedSetRight = nestedSetBound
				nestedSetBound++

				node = node.parent
			}
		}
	}

	return connected
}

// findParentNodeInMap finds the tree node containing the parent span for another node.
func findParentNodeInMap(nodesByID map[uint64][]*spanNode, node *spanNode) *spanNode {
	if node.span.IsRoot() {
		return nil
	}

	parentID := util.SpanIDToUint64(node.span.ParentSpanID)
	nodes := nodesByID[parentID]

	switch len(nodes) {
	case 0:
		return nil
	case 1:
		return nodes[0]
	case 2:
		// handle client/server spans with the same span ID
		kindWant := int(v1.Span_SPAN_KIND_SERVER)
		if node.span.Kind == int(v1.Span_SPAN_KIND_SERVER) {
			kindWant = int(v1.Span_SPAN_KIND_CLIENT)
		}

		if nodes[0].span.Kind == kindWant {
			return nodes[0]
		}
		if nodes[1].span.Kind == kindWant {
			return nodes[1]
		}
	}

	return nil
}
