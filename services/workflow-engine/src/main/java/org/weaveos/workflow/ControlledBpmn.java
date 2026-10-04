package org.weaveos.workflow;

import java.io.ByteArrayInputStream;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.regex.Pattern;
import javax.xml.XMLConstants;
import javax.xml.parsers.DocumentBuilderFactory;
import org.w3c.dom.Element;
import org.w3c.dom.Node;
import org.xml.sax.SAXException;
import org.xml.sax.SAXParseException;
import org.xml.sax.helpers.DefaultHandler;
import org.weaveos.workflow.DeploymentRegistry.InvalidDeployment;

/** Allowlist for services/bff/internal/flowgraph/compiler.go; no user executable XML. */
final class ControlledBpmn {
    private static final String BPMN = "http://www.omg.org/spec/BPMN/20100524/MODEL";
    private static final String FLOWABLE = "http://flowable.org/bpmn";
    private static final String XSI = XMLConstants.W3C_XML_SCHEMA_INSTANCE_NS_URI;
    private static final Pattern UUID = Pattern.compile("[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}");
    private static final Pattern COMPACT = Pattern.compile("[0-9a-f]{32}");
    private static final Pattern EDGE = Pattern.compile("e_(0|[1-9][0-9]*)");
    private static final int MAX_BYTES = 1024 * 1024;
    private ControlledBpmn() {}

    static void requireUuid(String id) {
        if (id == null || !UUID.matcher(id).matches() || id.equals("00000000-0000-0000-0000-000000000000")) invalid();
    }

    static byte[] validate(String xml, String versionId) {
        if (xml == null || xml.isEmpty() || xml.length() > MAX_BYTES) invalid();
        byte[] bytes = xml.getBytes(StandardCharsets.UTF_8);
        if (bytes.length > MAX_BYTES || !new String(bytes, StandardCharsets.UTF_8).equals(xml)) invalid();
        try {
            var factory = DocumentBuilderFactory.newInstance();
            factory.setNamespaceAware(true);
            factory.setFeature(XMLConstants.FEATURE_SECURE_PROCESSING, true);
            factory.setFeature("http://apache.org/xml/features/disallow-doctype-decl", true);
            factory.setFeature("http://xml.org/sax/features/external-general-entities", false);
            factory.setFeature("http://xml.org/sax/features/external-parameter-entities", false);
            factory.setFeature("http://apache.org/xml/features/nonvalidating/load-external-dtd", false);
            factory.setAttribute(XMLConstants.ACCESS_EXTERNAL_DTD, "");
            factory.setAttribute(XMLConstants.ACCESS_EXTERNAL_SCHEMA, "");
            factory.setXIncludeAware(false);
            factory.setExpandEntityReferences(false);
            // Compiler trees have at most six levels. Bound parser work before building the DOM.
            factory.setAttribute("http://www.oracle.com/xml/jaxp/properties/maxElementDepth", "8");
            var builder = factory.newDocumentBuilder();
            builder.setEntityResolver((publicId, systemId) -> { throw new SAXException("external entity forbidden"); });
            builder.setErrorHandler(new DefaultHandler() {
                @Override public void error(SAXParseException e) throws SAXException { throw e; }
                @Override public void fatalError(SAXParseException e) throws SAXException { throw e; }
            });
            var document = builder.parse(new ByteArrayInputStream(bytes));
            var roots = children(document, false);
            if (roots.size() != 1) invalid();
            Element root = roots.get(0);
            name(root, "definitions");
            attrs(root, Map.of("targetNamespace", "urn:weaveos:workflow"), true);
            var processes = children(root, false);
            if (processes.size() != 1) invalid();
            Element process = processes.get(0);
            name(process, "process");
            attrs(process, Map.of("id", "p_" + versionId.replace("-", ""), "isExecutable", "true"), false);
            validateProcess(process);
            return bytes;
        } catch (InvalidDeployment e) {
            throw e;
        } catch (Exception e) {
            // Never echo untrusted XML or parser diagnostics into the caller's response/logs.
            throw new InvalidDeployment();
        }
    }

    private static void validateProcess(Element process) {
        Map<String, Element> nodes = new HashMap<>();
        Map<String, Element> edges = new HashMap<>();
        Set<String> ids = new HashSet<>();
        int starts = 0, ends = 0;
        for (Element element : children(process, false)) {
            String type = element.getLocalName();
            String id = element.getAttribute("id");
            if (!BPMN.equals(element.getNamespaceURI()) || !ids.add(id)) invalid();
            switch (type) {
                case "startEvent", "endEvent" -> {
                    if (!(compactId(id, "n_") || (type.equals("endEvent") && id.equals("reject_end")))) invalid();
                    attrs(element, Map.of("id", id), false);
                    empty(element);
                    if (type.equals("startEvent")) starts++; else ends++;
                    nodes.put(id, element);
                }
                case "exclusiveGateway" -> {
                    if (!compactId(id, "n_") && !compactId(id, "g_")) invalid();
                    String defaultEdge = element.getAttribute("default");
                    if (!EDGE.matcher(defaultEdge).matches()) invalid();
                    attrs(element, Map.of("id", id, "default", defaultEdge), false);
                    empty(element);
                    nodes.put(id, element);
                }
                case "userTask" -> {
                    if (!compactId(id, "n_")) invalid();
                    attrs(element, Map.of("id", id, key(FLOWABLE, "assignee"), "${approver}"), false);
                    var loops = children(element, false);
                    if (loops.size() != 1) invalid();
                    Element loop = loops.get(0);
                    name(loop, "multiInstanceLoopCharacteristics");
                    attrs(loop, Map.of("isSequential", "false", key(FLOWABLE, "collection"), "a_" + id.substring(2),
                        key(FLOWABLE, "elementVariable"), "approver"), false);
                    var conditions = children(loop, false);
                    if (conditions.size() != 1) invalid();
                    Element condition = conditions.get(0);
                    name(condition, "completionCondition");
                    expressionAttrs(condition);
                    String text = expression(condition);
                    if (!text.equals("${wf_rejected || nrOfCompletedInstances == nrOfInstances}")
                        && !text.equals("${nrOfCompletedInstances > 0}")) invalid();
                    nodes.put(id, element);
                }
                case "sequenceFlow" -> {
                    if (!EDGE.matcher(id).matches() && !compactId(id, "u_") && !compactId(id, "r_")) invalid();
                    attrs(element, Map.of("id", id, "sourceRef", element.getAttribute("sourceRef"),
                        "targetRef", element.getAttribute("targetRef")), false);
                    var conditions = children(element, false);
                    if (conditions.size() > 1) invalid();
                    if (!conditions.isEmpty()) {
                        name(conditions.get(0), "conditionExpression");
                        expressionAttrs(conditions.get(0));
                        expression(conditions.get(0));
                    }
                    edges.put(id, element);
                }
                default -> invalid();
            }
        }
        if (starts != 1 || ends == 0) invalid();
        for (Element edge : edges.values()) {
            String id = edge.getAttribute("id"), source = edge.getAttribute("sourceRef"), target = edge.getAttribute("targetRef");
            if (!nodes.containsKey(source) || !nodes.containsKey(target)) invalid();
            var conditions = children(edge, false);
            if (id.startsWith("u_")) {
                if (!source.equals("n_" + id.substring(2)) || !target.equals("g_" + id.substring(2)) || !conditions.isEmpty()
                    || !nodes.get(source).getLocalName().equals("userTask")) invalid();
            } else if (id.startsWith("r_")) {
                if (!source.equals("g_" + id.substring(2)) || !target.equals("reject_end") || conditions.size() != 1
                    || !expression(conditions.get(0)).equals("${wf_rejected == true}")) invalid();
            } else if (!conditions.isEmpty()) {
                if (!compactId(source, "n_") || !nodes.get(source).getLocalName().equals("exclusiveGateway")
                    || !expression(conditions.get(0)).equals("${route_" + source.substring(2) + " == true}")) invalid();
            }
        }
        for (Element node : nodes.values()) {
            if (node.getLocalName().equals("exclusiveGateway")) {
                Element edge = edges.get(node.getAttribute("default"));
                if (edge == null || !edge.getAttribute("sourceRef").equals(node.getAttribute("id")) || !children(edge, false).isEmpty()) invalid();
                String id = node.getAttribute("id");
                if (id.startsWith("g_") && (!nodes.containsKey("n_" + id.substring(2))
                    || !nodes.get("n_" + id.substring(2)).getLocalName().equals("userTask"))) invalid();
            }
        }
    }

    private static boolean compactId(String id, String prefix) {
        return id.startsWith(prefix) && COMPACT.matcher(id.substring(prefix.length())).matches()
            && !id.substring(prefix.length()).equals("00000000000000000000000000000000");
    }
    private static String key(String namespace, String local) { return namespace + "|" + local; }
    private static void name(Element element, String local) {
        if (!local.equals(element.getLocalName()) || !BPMN.equals(element.getNamespaceURI())) invalid();
    }
    private static void attrs(Element element, Map<String, String> expected, boolean namespaces) {
        Set<String> found = new HashSet<>();
        var attributes = element.getAttributes();
        for (int i = 0; i < attributes.getLength(); i++) {
            Node attribute = attributes.item(i);
            String ns = attribute.getNamespaceURI();
            if (XMLConstants.XMLNS_ATTRIBUTE_NS_URI.equals(ns)) {
                if (!namespaces) invalid();
                String value = switch (attribute.getNodeName()) {
                    case "xmlns", "xmlns:bpmn" -> BPMN;
                    case "xmlns:flowable" -> FLOWABLE;
                    case "xmlns:xsi" -> XSI;
                    default -> null;
                };
                if (!attribute.getNodeValue().equals(value)) invalid();
                continue;
            }
            String attributeKey = ns == null ? attribute.getNodeName() : key(ns, attribute.getLocalName());
            if (!attribute.getNodeValue().equals(expected.get(attributeKey)) || !found.add(attributeKey)) invalid();
        }
        if (!found.equals(expected.keySet())) invalid();
    }
    private static void expressionAttrs(Element element) {
        attrs(element, Map.of(key(XSI, "type"), "bpmn:tFormalExpression"), false);
        if (!BPMN.equals(element.lookupNamespaceURI("bpmn"))) invalid();
    }
    private static String expression(Element element) {
        if (!children(element, true).isEmpty()) invalid();
        return element.getTextContent();
    }
    private static void empty(Element element) { if (!children(element, false).isEmpty()) invalid(); }
    private static List<Element> children(Node parent, boolean expression) {
        List<Element> elements = new ArrayList<>();
        for (Node child = parent.getFirstChild(); child != null; child = child.getNextSibling()) {
            switch (child.getNodeType()) {
                case Node.ELEMENT_NODE -> elements.add((Element) child);
                case Node.TEXT_NODE -> { if (!expression && !child.getNodeValue().isBlank()) invalid(); }
                // Comments, CDATA, PI, entity references and DTDs are not compiler output.
                default -> invalid();
            }
        }
        return elements;
    }
    private static void invalid() { throw new InvalidDeployment(); }
}
