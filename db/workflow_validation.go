package db

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/impishMD/taskexec/pkg/common_errors"
	"github.com/impishMD/taskexec/pkg/task_logger"
)

func WorkflowConditionMatches(status task_logger.TaskStatus, condition WorkflowEdgeCondition) bool {
	if !status.IsFinished() {
		return false
	}
	switch condition {
	case WorkflowEdgeAlways:
		return true
	case WorkflowEdgeOnSuccess:
		return status == task_logger.TaskSuccessStatus
	case WorkflowEdgeOnFailure:
		return status != task_logger.TaskSuccessStatus
	default:
		return false
	}
}

// WorkflowNodeOrder returns a deterministic topological order, excluding notes.
// Validation belongs on the server too: the editor is not a trust boundary.
func WorkflowNodeOrder(workflow WorkflowTemplate) ([]WorkflowNode, error) {
	bad := common_errors.NewValidationError
	nodes := map[int]WorkflowNode{}
	incoming := map[int]int{}
	outgoing := map[int][]int{}
	for _, node := range workflow.Nodes {
		if _, exists := nodes[node.ID]; exists {
			return nil, bad("workflow node IDs must be unique")
		}
		nodes[node.ID] = node
		if node.EffectiveKind() != WorkflowNodeNoteKind {
			incoming[node.ID] = 0
		}
	}
	seen := map[[2]int]bool{}
	for _, edge := range workflow.Edges {
		from, okFrom := nodes[edge.SourceNodeID]
		to, okTo := nodes[edge.DestinationNodeID]
		pair := [2]int{edge.SourceNodeID, edge.DestinationNodeID}
		if !okFrom || !okTo || from.EffectiveKind() == WorkflowNodeNoteKind || to.EffectiveKind() == WorkflowNodeNoteKind || pair[0] == pair[1] || seen[pair] {
			return nil, bad("workflow edges must connect distinct executable nodes exactly once")
		}
		if err := edge.Condition.Validate(); err != nil {
			return nil, err
		}
		seen[pair] = true
		incoming[pair[1]]++
		outgoing[pair[0]] = append(outgoing[pair[0]], pair[1])
	}
	queue := []WorkflowNode{}
	for _, node := range workflow.Nodes {
		if n, ok := incoming[node.ID]; ok && n == 0 {
			queue = append(queue, node)
		}
	}
	if len(queue) != 1 {
		return nil, bad("workflow must have exactly one executable root")
	}
	order := []WorkflowNode{}
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		order = append(order, node)
		for _, dest := range outgoing[node.ID] {
			incoming[dest]--
			if incoming[dest] == 0 {
				queue = append(queue, nodes[dest])
			}
		}
	}
	if len(order) != len(incoming) {
		return nil, bad("workflow must not contain cycles")
	}
	return order, nil
}

func WorkflowRootNode(workflow WorkflowTemplate) (WorkflowNode, error) {
	order, err := WorkflowNodeOrder(workflow)
	if err != nil {
		return WorkflowNode{}, err
	}
	return order[0], nil
}

func ValidateWorkflowTemplate(store WorkflowTemplateValidationStore, workflow WorkflowTemplate) error {
	bad := common_errors.NewValidationError
	if strings.TrimSpace(workflow.Name) == "" || len(workflow.Name) > 255 {
		return bad("workflow name is required and must not exceed 255 bytes")
	}
	if len(workflow.Nodes) > 500 || len(workflow.Edges) > 2000 {
		return bad("workflow exceeds 500 nodes or 2000 edges")
	}
	if workflow.StartVersion != nil && len(*workflow.StartVersion) > 20 {
		return bad("workflow start version must not exceed 20 bytes")
	}
	if workflow.StartVersion != nil {
		if parts := buildVersionRE.FindStringSubmatch(*workflow.StartVersion); parts != nil {
			if _, err := strconv.Atoi(parts[2]); err != nil {
				return bad("workflow version number is too large")
			}
		}
	}
	if _, err := WorkflowNodeOrder(workflow); err != nil {
		return err
	}
	for _, node := range workflow.Nodes {
		if err := node.EffectiveKind().Validate(); err != nil {
			return err
		}
		if err := node.EffectiveConvergenceMode().Validate(); err != nil {
			return err
		}
		switch node.EffectiveKind() {
		case WorkflowNodeTaskKind:
			template, err := store.GetTemplate(workflow.ProjectID, node.TemplateID)
			if err != nil {
				return bad(fmt.Sprintf("workflow node %d must reference a template in this project", node.ID))
			}
			if node.TaskParams != nil {
				task := node.TaskParams.CreateTask(template.ID)
				if err = task.ValidateNewTask(template); err != nil {
					return err
				}
			}
		case WorkflowNodeApprovalKind:
			if node.ApprovalTimeout != nil && (*node.ApprovalTimeout <= 0 || *node.ApprovalTimeout > 2147483647) {
				return bad("approval timeout must be a positive number of seconds")
			}
		case WorkflowNodeDelayKind:
			if node.DelaySeconds == nil || *node.DelaySeconds <= 0 || *node.DelaySeconds > 2147483647 {
				return bad("workflow delay must be a positive number of seconds")
			}
		}
	}
	return nil
}
