package text

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/xerrors"
)

const (
	symbolAdded     = "[+]"
	symbolRemoved   = "[-]"
	symbolModified  = "[~]"
	symbolUnchanged = "[ ]"
	indentSize      = 2
)

type DOMDiff struct {
	parser     *htmlParser
	builder    *treeBuilder
	comparator *nodeComparator
	formatter  *diffFormatter
}

func NewDOMDiff() *DOMDiff {
	return &DOMDiff{
		parser:     &htmlParser{},
		builder:    &treeBuilder{},
		comparator: &nodeComparator{},
		formatter:  &diffFormatter{},
	}
}

func (d *DOMDiff) Calculate(baseline []byte, target []byte) (*DiffResult, error) {
	baselineDoc, err := d.parser.parse(baseline)
	if err != nil {
		return nil, xerrors.Errorf("failed to parse baseline HTML: %w", err)
	}

	targetDoc, err := d.parser.parse(target)
	if err != nil {
		return nil, xerrors.Errorf("failed to parse target HTML: %w", err)
	}

	baselineTree := d.builder.buildTree(baselineDoc)
	targetTree := d.builder.buildTree(targetDoc)

	comparison := d.comparator.compare(baselineTree, targetTree)
	formattedDiff := d.formatter.format(comparison)

	return &DiffResult{
		Diff:       formattedDiff,
		DiffAmount: comparison.diffRatio(),
	}, nil
}

type treeNode struct {
	path     string
	depth    int
	nodeType html.NodeType
	tag      string
	attrs    map[string]string
	text     string
	children []*treeNode
}

type htmlParser struct{}

func (p *htmlParser) parse(content []byte) (*html.Node, error) {
	return html.Parse(bytes.NewReader(content))
}

type treeBuilder struct{}

func (b *treeBuilder) buildTree(doc *html.Node) *treeNode {
	root := &treeNode{path: "", depth: -1}
	b.buildTreeRecursive(doc, root, "", -1)
	return root
}

func (b *treeBuilder) buildTreeRecursive(n *html.Node, parent *treeNode, path string, depth int) {
	if n == nil {
		return
	}

	node := b.createTreeNode(n, path, depth)
	if node == nil {
		node = parent
	} else if b.shouldAddNode(n, node) {
		parent.children = append(parent.children, node)
	}

	b.processChildren(n, node, path, depth)
}

func (b *treeBuilder) createTreeNode(n *html.Node, path string, depth int) *treeNode {
	node := &treeNode{
		path:     path,
		depth:    depth,
		nodeType: n.Type,
		children: []*treeNode{},
	}

	switch n.Type {
	case html.ElementNode:
		node.tag = n.Data
		node.attrs = b.extractAttributes(n)
		return node
	case html.TextNode:
		text := strings.TrimSpace(n.Data)
		if text != "" {
			node.text = text
			return node
		}
		return nil
	default:
		return nil
	}
}

func (b *treeBuilder) extractAttributes(n *html.Node) map[string]string {
	attrs := make(map[string]string)
	for _, attr := range n.Attr {
		attrs[attr.Key] = attr.Val
	}
	return attrs
}

func (b *treeBuilder) shouldAddNode(n *html.Node, node *treeNode) bool {
	return node != nil && (n.Type == html.ElementNode ||
		(n.Type == html.TextNode && node.text != ""))
}

func (b *treeBuilder) processChildren(n *html.Node, parent *treeNode, path string, depth int) {
	childIndex := 0
	textIndex := 0
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		childPath := b.buildChildPath(c, path, &childIndex, &textIndex)
		b.buildTreeRecursive(c, parent, childPath, depth+1)
	}
}

func (b *treeBuilder) buildChildPath(n *html.Node, parentPath string, childIndex, textIndex *int) string {
	switch n.Type {
	case html.ElementNode:
		path := fmt.Sprintf("%s/%s[%d]", parentPath, n.Data, *childIndex)
		*childIndex++
		return path
	case html.TextNode:
		if strings.TrimSpace(n.Data) != "" {
			path := fmt.Sprintf("%s/text[%d]", parentPath, *textIndex)
			*textIndex++
			return path
		}
	}
	return parentPath
}

type comparisonResult struct {
	changes    []change
	totalNodes int
}

type change struct {
	changeType changeType
	path       string
	baseline   *treeNode
	target     *treeNode
	indent     int
}

type changeType int

const (
	changeAdded changeType = iota
	changeRemoved
	changeModified
	changeUnchanged
)

func (r *comparisonResult) diffRatio() float64 {
	if r.totalNodes == 0 {
		return 0.0
	}

	changedCount := 0
	for _, c := range r.changes {
		if c.changeType != changeUnchanged {
			changedCount++
		}
	}

	ratio := float64(changedCount) / float64(r.totalNodes)
	if ratio > 1.0 {
		return 1.0
	}
	return ratio
}

type nodeComparator struct{}

func (c *nodeComparator) compare(baseline, target *treeNode) *comparisonResult {
	result := &comparisonResult{
		changes: []change{},
	}
	c.compareNodes(baseline, target, 0, result)
	return result
}

func (c *nodeComparator) compareNodes(baseline, target *treeNode, indent int, result *comparisonResult) {
	baselineMap := c.buildNodeMap(baseline.children)
	targetMap := c.buildNodeMap(target.children)

	allPaths := c.collectAllPaths(baselineMap, targetMap)

	for _, path := range allPaths {
		c.processNodeComparison(path, baselineMap, targetMap, indent, result)
	}
}

func (c *nodeComparator) buildNodeMap(nodes []*treeNode) map[string]*treeNode {
	nodeMap := make(map[string]*treeNode)
	for _, node := range nodes {
		nodeMap[node.path] = node
	}
	return nodeMap
}

func (c *nodeComparator) collectAllPaths(baselineMap, targetMap map[string]*treeNode) []string {
	pathSet := make(map[string]bool)
	for path := range baselineMap {
		pathSet[path] = true
	}
	for path := range targetMap {
		pathSet[path] = true
	}

	paths := make([]string, 0, len(pathSet))
	for path := range pathSet {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func (c *nodeComparator) processNodeComparison(path string, baselineMap, targetMap map[string]*treeNode, indent int, result *comparisonResult) {
	baseNode, inBase := baselineMap[path]
	targetNode, inTarget := targetMap[path]

	result.totalNodes++

	var change change
	change.path = path
	change.indent = indent

	switch {
	case !inBase && inTarget:
		change.changeType = changeAdded
		change.target = targetNode
		result.changes = append(result.changes, change)
		if len(targetNode.children) > 0 {
			c.compareNodes(&treeNode{children: []*treeNode{}}, targetNode, indent+1, result)
		}
	case inBase && !inTarget:
		change.changeType = changeRemoved
		change.baseline = baseNode
		result.changes = append(result.changes, change)
		if len(baseNode.children) > 0 {
			c.compareNodes(baseNode, &treeNode{children: []*treeNode{}}, indent+1, result)
		}
	case inBase && inTarget:
		if c.nodesEqual(baseNode, targetNode) {
			change.changeType = changeUnchanged
			change.baseline = baseNode
			result.changes = append(result.changes, change)
		} else {
			change.changeType = changeModified
			change.baseline = baseNode
			change.target = targetNode
			result.changes = append(result.changes, change)
		}
		c.compareNodes(baseNode, targetNode, indent+1, result)
	}
}

func (c *nodeComparator) nodesEqual(a, b *treeNode) bool {
	if a.nodeType != b.nodeType {
		return false
	}

	switch a.nodeType {
	case html.ElementNode:
		return c.elementNodesEqual(a, b)
	case html.TextNode:
		return a.text == b.text
	default:
		return true
	}
}

func (c *nodeComparator) elementNodesEqual(a, b *treeNode) bool {
	if a.tag != b.tag || len(a.attrs) != len(b.attrs) {
		return false
	}

	for k, v := range a.attrs {
		if b.attrs[k] != v {
			return false
		}
	}
	return true
}

type diffFormatter struct{}

func (f *diffFormatter) format(comparison *comparisonResult) []byte {
	var buf bytes.Buffer

	f.writeHeader(&buf)
	f.writeChanges(&buf, comparison.changes)
	f.writeLegend(&buf)

	return buf.Bytes()
}

func (f *diffFormatter) writeHeader(w *bytes.Buffer) {
	w.WriteString("DOM Tree Diff:\n")
	w.WriteString("==============\n\n")
}

func (f *diffFormatter) writeChanges(w *bytes.Buffer, changes []change) {
	for _, change := range changes {
		f.writeChange(w, change)
	}
}

func (f *diffFormatter) writeChange(w *bytes.Buffer, change change) {
	indent := strings.Repeat(" ", change.indent*indentSize)

	switch change.changeType {
	case changeAdded:
		fmt.Fprintf(w, "%s%s %s\n", indent, symbolAdded, f.formatNode(change.target))
	case changeRemoved:
		fmt.Fprintf(w, "%s%s %s\n", indent, symbolRemoved, f.formatNode(change.baseline))
	case changeModified:
		fmt.Fprintf(w, "%s%s %s → %s\n", indent, symbolModified,
			f.formatNode(change.baseline), f.formatNode(change.target))
	case changeUnchanged:
		fmt.Fprintf(w, "%s%s %s\n", indent, symbolUnchanged, f.formatNode(change.baseline))
	}
}

func (f *diffFormatter) formatNode(n *treeNode) string {
	if n.nodeType == html.TextNode {
		return fmt.Sprintf("text: \"%s\"", n.text)
	}

	if n.nodeType == html.ElementNode {
		if len(n.attrs) == 0 {
			return fmt.Sprintf("<%s>", n.tag)
		}

		attrs := f.formatAttributes(n.attrs)
		return fmt.Sprintf("<%s %s>", n.tag, attrs)
	}

	return "unknown"
}

func (f *diffFormatter) formatAttributes(attrs map[string]string) string {
	formatted := make([]string, 0, len(attrs))
	for k, v := range attrs {
		formatted = append(formatted, fmt.Sprintf("%s=\"%s\"", k, v))
	}
	sort.Strings(formatted)
	return strings.Join(formatted, " ")
}

func (f *diffFormatter) writeLegend(w *bytes.Buffer) {
	w.WriteString("\nLegend:\n")
	w.WriteString("  " + symbolAdded + " Added\n")
	w.WriteString("  " + symbolRemoved + " Removed\n")
	w.WriteString("  " + symbolModified + " Modified\n")
	w.WriteString("  " + symbolUnchanged + " Unchanged\n")
}

var _ Differ = (*DOMDiff)(nil)
