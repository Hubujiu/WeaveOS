package flowgraph

import (
	"bytes"
	"encoding/xml"
	"strconv"
	"strings"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appquery"
)

const (
	bpmnNamespace     = "http://www.omg.org/spec/BPMN/20100524/MODEL"
	flowableNamespace = "http://flowable.org/bpmn"
	xsiNamespace      = "http://www.w3.org/2001/XMLSchema-instance"
	workflowNamespace = "urn:weaveos:workflow"
)

// CompileBPMN renders a validated workflow into the fixed, server-owned BPMN
// subset. It never incorporates user predicates, labels, or assignee identities
// into executable XML.
func CompileBPMN(g Graph, fields []appquery.Field, definitionID string) ([]byte, error) {
	if !validUUID(definitionID) {
		return nil, ErrInvalid
	}
	return compileBPMN(g, fields, "p_"+uuidCompact(definitionID))
}

// compileBPMN is the single renderer for legacy and application-scoped keys.
// Both callers construct processKey solely from validated server identities.
func compileBPMN(g Graph, fields []appquery.Field, processKey string) ([]byte, error) {
	validated, err := Validate(g, fields)
	if err != nil {
		return nil, ErrInvalid
	}

	nodes := make(map[string]Node, len(g.Nodes))
	for _, node := range g.Nodes {
		nodes[node.ID] = node
	}

	approvalEdges := make(map[string]int)
	for i, edge := range g.Edges {
		if nodes[edge.From].Kind == "approval" {
			approvalEdges[edge.From] = i
		}
	}

	root := bpmnElement{
		name: "definitions",
		attrs: []xml.Attr{
			bpmnAttr("xmlns", bpmnNamespace),
			bpmnAttr("xmlns:bpmn", bpmnNamespace),
			bpmnAttr("xmlns:flowable", flowableNamespace),
			bpmnAttr("xmlns:xsi", xsiNamespace),
			bpmnAttr("targetNamespace", workflowNamespace),
		},
	}
	process := bpmnElement{
		name: "process",
		attrs: []xml.Attr{
			bpmnAttr("id", processKey),
			bpmnAttr("isExecutable", "true"),
		},
	}

	for _, id := range validated.Order {
		node := nodes[id]
		xmlID := bpmnNodeID(id)
		switch node.Kind {
		case "start":
			process.children = append(process.children, bpmnElement{name: "startEvent", attrs: []xml.Attr{bpmnAttr("id", xmlID)}})
		case "end":
			process.children = append(process.children, bpmnElement{name: "endEvent", attrs: []xml.Attr{bpmnAttr("id", xmlID)}})
		case "condition":
			falseFlow := ""
			for i, edge := range g.Edges {
				if edge.From == id && edge.Branch == "false" {
					falseFlow = bpmnEdgeID(i)
					break
				}
			}
			process.children = append(process.children, bpmnElement{
				name:  "exclusiveGateway",
				attrs: []xml.Attr{bpmnAttr("id", xmlID), bpmnAttr("default", falseFlow)},
			})
		case "approval":
			approvalID := uuidCompact(id)
			completion := "${wf_rejected || nrOfCompletedInstances == nrOfInstances}"
			if node.Approval.Mode == "any" {
				completion = "${nrOfCompletedInstances > 0}"
			}
			process.children = append(process.children,
				bpmnElement{
					name: "userTask",
					attrs: []xml.Attr{
						bpmnAttr("id", xmlID),
						bpmnAttr("flowable:assignee", "${approver}"),
					},
					children: []bpmnElement{{
						name: "multiInstanceLoopCharacteristics",
						attrs: []xml.Attr{
							bpmnAttr("isSequential", "false"),
							bpmnAttr("flowable:collection", "a_"+approvalID),
							bpmnAttr("flowable:elementVariable", "approver"),
						},
						children: []bpmnElement{{
							name:  "completionCondition",
							attrs: []xml.Attr{bpmnAttr("xsi:type", "bpmn:tFormalExpression")},
							text:  completion,
						}},
					}},
				},
				bpmnElement{
					name: "exclusiveGateway",
					attrs: []xml.Attr{
						bpmnAttr("id", bpmnApprovalGatewayID(id)),
						bpmnAttr("default", bpmnEdgeID(approvalEdges[id])),
					},
				},
			)
		}
	}

	hasApproval := len(approvalEdges) > 0
	if hasApproval {
		process.children = append(process.children, bpmnElement{
			name:  "endEvent",
			attrs: []xml.Attr{bpmnAttr("id", "reject_end")},
		})
	}

	for i, edge := range g.Edges {
		source := bpmnNodeID(edge.From)
		if nodes[edge.From].Kind == "approval" {
			source = bpmnApprovalGatewayID(edge.From)
		}
		flow := bpmnElement{
			name: "sequenceFlow",
			attrs: []xml.Attr{
				bpmnAttr("id", bpmnEdgeID(i)),
				bpmnAttr("sourceRef", source),
				bpmnAttr("targetRef", bpmnNodeID(edge.To)),
			},
		}
		if nodes[edge.From].Kind == "condition" && edge.Branch == "true" {
			flow.children = []bpmnElement{{
				name:  "conditionExpression",
				attrs: []xml.Attr{bpmnAttr("xsi:type", "bpmn:tFormalExpression")},
				text:  "${route_" + uuidCompact(edge.From) + " == true}",
			}}
		}
		process.children = append(process.children, flow)
	}

	for _, node := range g.Nodes {
		if node.Kind != "approval" {
			continue
		}
		compactID := uuidCompact(node.ID)
		process.children = append(process.children,
			bpmnElement{
				name: "sequenceFlow",
				attrs: []xml.Attr{
					bpmnAttr("id", "u_"+compactID),
					bpmnAttr("sourceRef", bpmnNodeID(node.ID)),
					bpmnAttr("targetRef", bpmnApprovalGatewayID(node.ID)),
				},
			},
		)
		if hasApproval {
			process.children = append(process.children, bpmnElement{
				name: "sequenceFlow",
				attrs: []xml.Attr{
					bpmnAttr("id", "r_"+compactID),
					bpmnAttr("sourceRef", bpmnApprovalGatewayID(node.ID)),
					bpmnAttr("targetRef", "reject_end"),
				},
				children: []bpmnElement{{
					name:  "conditionExpression",
					attrs: []xml.Attr{bpmnAttr("xsi:type", "bpmn:tFormalExpression")},
					text:  "${wf_rejected == true}",
				}},
			})
		}
	}

	root.children = []bpmnElement{process}
	var output bytes.Buffer
	encoder := xml.NewEncoder(&output)
	if err := encodeBPMNElement(encoder, root); err != nil {
		return nil, ErrInvalid
	}
	if err := encoder.Flush(); err != nil {
		return nil, ErrInvalid
	}
	return output.Bytes(), nil
}

type bpmnElement struct {
	name     string
	attrs    []xml.Attr
	children []bpmnElement
	text     string
}

func encodeBPMNElement(encoder *xml.Encoder, element bpmnElement) error {
	start := xml.StartElement{Name: xml.Name{Local: element.name}, Attr: element.attrs}
	if err := encoder.EncodeToken(start); err != nil {
		return err
	}
	if element.text != "" {
		if err := encoder.EncodeToken(xml.CharData([]byte(element.text))); err != nil {
			return err
		}
	}
	for _, child := range element.children {
		if err := encodeBPMNElement(encoder, child); err != nil {
			return err
		}
	}
	return encoder.EncodeToken(start.End())
}

func bpmnAttr(name, value string) xml.Attr {
	return xml.Attr{Name: xml.Name{Local: name}, Value: value}
}

func bpmnNodeID(id string) string            { return "n_" + uuidCompact(id) }
func bpmnApprovalGatewayID(id string) string { return "g_" + uuidCompact(id) }
func bpmnEdgeID(index int) string            { return "e_" + strconv.Itoa(index) }
func uuidCompact(id string) string           { return strings.ReplaceAll(id, "-", "") }
