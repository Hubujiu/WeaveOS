package flowgraph

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appquery"
)

var ErrInvalid = errors.New("invalid workflow graph")

type Graph struct {
	Version int
	Nodes   []Node
	Edges   []Edge
}
type Node struct {
	ID        string
	Kind      string
	Approval  *Approval
	Condition json.RawMessage
}
type Approval struct {
	Mode             string
	AssigneeIDs      []string
	EditableFieldIDs []string
}
type Edge struct{ From, To, Branch string }
type Validated struct{ Order, ReferencedFields []string }

var canonicalUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

const zeroUUID = "00000000-0000-0000-0000-000000000000"

func Validate(g Graph, fields []appquery.Field) (Validated, error) {
	if g.Version != 1 || len(g.Nodes) < 2 || len(g.Nodes) > 100 || len(g.Edges) < 1 || len(g.Edges) > 200 {
		return Validated{}, invalid("unsupported version or graph size")
	}
	if len(fields) > 200 {
		return Validated{}, invalid("field context exceeds limit")
	}

	fieldByID := make(map[string]appquery.Field, len(fields))
	for _, field := range fields {
		if !validUUID(field.ID) || !validFieldKind(field.Kind) {
			return Validated{}, invalid("invalid field context")
		}
		if _, exists := fieldByID[field.ID]; exists {
			return Validated{}, invalid("duplicate field context identity")
		}
		fieldByID[field.ID] = field
	}

	nodeIndex := make(map[string]int, len(g.Nodes))
	start := -1
	endCount := 0
	dependencies := make(map[string]struct{})
	for i, node := range g.Nodes {
		if !validUUID(node.ID) {
			return Validated{}, invalid("invalid node identity")
		}
		if _, exists := nodeIndex[node.ID]; exists {
			return Validated{}, invalid("duplicate node identity")
		}
		nodeIndex[node.ID] = i

		switch node.Kind {
		case "start":
			if start >= 0 || node.Approval != nil || hasCondition(node.Condition) {
				return Validated{}, invalid("invalid start node configuration")
			}
			start = i
		case "approval":
			if node.Approval == nil || hasCondition(node.Condition) {
				return Validated{}, invalid("invalid approval node configuration")
			}
			approval := node.Approval
			if approval.Mode != "all" && approval.Mode != "any" {
				return Validated{}, invalid("invalid approval mode")
			}
			if len(approval.AssigneeIDs) < 1 || len(approval.AssigneeIDs) > 50 {
				return Validated{}, invalid("approval assignee count exceeds bounds")
			}
			seenAssignees := make(map[string]struct{}, len(approval.AssigneeIDs))
			for _, id := range approval.AssigneeIDs {
				if !validUUID(id) {
					return Validated{}, invalid("invalid approval assignee identity")
				}
				if _, exists := seenAssignees[id]; exists {
					return Validated{}, invalid("duplicate approval assignee")
				}
				seenAssignees[id] = struct{}{}
			}
			if len(approval.EditableFieldIDs) > 200 {
				return Validated{}, invalid("editable field count exceeds limit")
			}
			seenFields := make(map[string]struct{}, len(approval.EditableFieldIDs))
			for _, id := range approval.EditableFieldIDs {
				if !validUUID(id) {
					return Validated{}, invalid("invalid editable field identity")
				}
				if _, exists := seenFields[id]; exists {
					return Validated{}, invalid("duplicate editable field")
				}
				if _, exists := fieldByID[id]; !exists {
					return Validated{}, invalid("editable field is not in field context")
				}
				seenFields[id] = struct{}{}
				dependencies[id] = struct{}{}
			}
		case "condition":
			if node.Approval != nil || !hasCondition(node.Condition) || bytes.Equal(bytes.TrimSpace(node.Condition), []byte("null")) {
				return Validated{}, invalid("invalid condition node configuration")
			}
			plan, err := appquery.Compile(node.Condition, nil, fields, 1)
			if err != nil {
				return Validated{}, invalid("invalid condition filter")
			}
			for _, id := range plan.ReferencedFields {
				dependencies[id] = struct{}{}
			}
		case "end":
			if node.Approval != nil || hasCondition(node.Condition) {
				return Validated{}, invalid("invalid end node configuration")
			}
			endCount++
		default:
			return Validated{}, invalid("unsupported node kind")
		}
	}
	if start < 0 || endCount == 0 {
		return Validated{}, invalid("graph requires one start and at least one end")
	}

	adjacency := make([][]int, len(g.Nodes))
	indegree := make([]int, len(g.Nodes))
	outdegree := make([]int, len(g.Nodes))
	branchCounts := make([]map[string]int, len(g.Nodes))
	seenEdges := make(map[edgeKey]struct{}, len(g.Edges))
	for _, edge := range g.Edges {
		from, fromExists := nodeIndex[edge.From]
		to, toExists := nodeIndex[edge.To]
		if !fromExists || !toExists || from == to {
			return Validated{}, invalid("edge endpoint is missing or self-referential")
		}
		key := edgeKey{from: edge.From, to: edge.To, branch: edge.Branch}
		if _, exists := seenEdges[key]; exists {
			return Validated{}, invalid("duplicate edge")
		}
		seenEdges[key] = struct{}{}
		if g.Nodes[from].Kind == "condition" {
			if edge.Branch != "true" && edge.Branch != "false" {
				return Validated{}, invalid("condition edge must select true or false")
			}
			if branchCounts[from] == nil {
				branchCounts[from] = make(map[string]int, 2)
			}
			branchCounts[from][edge.Branch]++
		} else if edge.Branch != "" {
			return Validated{}, invalid("non-condition edge has branch")
		}
		adjacency[from] = append(adjacency[from], to)
		outdegree[from]++
		indegree[to]++
	}

	for i, node := range g.Nodes {
		switch node.Kind {
		case "start":
			if indegree[i] != 0 || outdegree[i] != 1 {
				return Validated{}, invalid("invalid start node degree")
			}
		case "approval":
			if indegree[i] < 1 || outdegree[i] != 1 {
				return Validated{}, invalid("invalid approval node degree")
			}
		case "condition":
			counts := branchCounts[i]
			if indegree[i] < 1 || outdegree[i] != 2 || counts["true"] != 1 || counts["false"] != 1 {
				return Validated{}, invalid("condition requires one true and one false edge")
			}
		case "end":
			if indegree[i] < 1 || outdegree[i] != 0 {
				return Validated{}, invalid("invalid end node degree")
			}
		}
	}

	visited := make([]bool, len(g.Nodes))
	work := []int{start}
	visited[start] = true
	for head := 0; head < len(work); head++ {
		for _, next := range adjacency[work[head]] {
			if !visited[next] {
				visited[next] = true
				work = append(work, next)
			}
		}
	}
	for _, reached := range visited {
		if !reached {
			return Validated{}, invalid("graph contains unreachable nodes")
		}
	}

	remaining := append([]int(nil), indegree...)
	queue := make([]int, 0, len(g.Nodes))
	for i, degree := range remaining {
		if degree == 0 {
			queue = append(queue, i)
		}
	}
	order := make([]string, 0, len(g.Nodes))
	for head := 0; head < len(queue); head++ {
		current := queue[head]
		order = append(order, g.Nodes[current].ID)
		for _, next := range adjacency[current] {
			remaining[next]--
			if remaining[next] == 0 {
				queue = append(queue, next)
			}
		}
	}
	if len(order) != len(g.Nodes) {
		return Validated{}, invalid("graph contains a cycle")
	}

	referencedFields := make([]string, 0, len(dependencies))
	for id := range dependencies {
		referencedFields = append(referencedFields, id)
	}
	sort.Strings(referencedFields)
	return Validated{Order: order, ReferencedFields: referencedFields}, nil
}

type edgeKey struct {
	from   string
	to     string
	branch string
}

func invalid(reason string) error {
	return fmt.Errorf("%w: %s", ErrInvalid, reason)
}

func hasCondition(raw json.RawMessage) bool {
	return len(bytes.TrimSpace(raw)) > 0
}

func validUUID(id string) bool {
	return id != zeroUUID && canonicalUUID.MatchString(id)
}

func validFieldKind(kind appquery.FieldKind) bool {
	switch kind {
	case appquery.Text, appquery.Multiline, appquery.Number, appquery.Money,
		appquery.Date, appquery.Datetime, appquery.Boolean, appquery.SingleSelect,
		appquery.MultiSelect, appquery.Member, appquery.Department:
		return true
	default:
		return false
	}
}
