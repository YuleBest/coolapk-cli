package render

import (
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// htmlToText 把动态正文里的 HTML 片段转成纯文本：
// 链接渲染成 "文本 <地址>"，<br> 与块级标签换成换行，其余标签丢弃。
func htmlToText(source string) string {
	if source == "" || !strings.ContainsAny(source, "<&") {
		return source
	}
	nodes, err := html.ParseFragment(strings.NewReader(source), &html.Node{
		Type:     html.ElementNode,
		Data:     "div",
		DataAtom: atom.Div,
	})
	if err != nil {
		return source
	}
	var builder strings.Builder
	for _, node := range nodes {
		writeNode(&builder, node)
	}
	return strings.TrimSpace(trimBlankLines(builder.String()))
}

func writeNode(builder *strings.Builder, node *html.Node) {
	switch node.Type {
	case html.TextNode:
		builder.WriteString(node.Data)
		return
	case html.ElementNode:
	default:
		writeChildren(builder, node)
		return
	}

	switch node.Data {
	case "br":
		builder.WriteString("\n")
	case "a":
		label := strings.TrimSpace(nodeText(node))
		href := attribute(node, "href")
		switch {
		case label == "":
			builder.WriteString(href)
		case href == "" || href == label || !isWebURL(href):
			// 站内相对链接（话题、动态）只保留文字，避免噪声。
			builder.WriteString(label)
		default:
			builder.WriteString(label + " <" + href + ">")
		}
	case "img":
		if src := attribute(node, "src"); src != "" {
			builder.WriteString("图片 " + src)
		}
	case "p", "div", "li", "blockquote":
		newline(builder)
		writeChildren(builder, node)
		newline(builder)
	default:
		writeChildren(builder, node)
	}
}

func writeChildren(builder *strings.Builder, node *html.Node) {
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		writeNode(builder, child)
	}
}

func nodeText(node *html.Node) string {
	var builder strings.Builder
	writeChildren(&builder, node)
	return builder.String()
}

func attribute(node *html.Node, key string) string {
	for _, attr := range node.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func isWebURL(value string) bool {
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")
}

func newline(builder *strings.Builder) {
	text := builder.String()
	if text == "" || strings.HasSuffix(text, "\n") {
		return
	}
	builder.WriteString("\n")
}

// trimBlankLines 把连续两个以上空行压成一个空行。
func trimBlankLines(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	blank := false
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			if blank {
				continue
			}
			blank = true
			out = append(out, "")
			continue
		}
		blank = false
		out = append(out, strings.TrimRight(line, " \t"))
	}
	return strings.Join(out, "\n")
}
