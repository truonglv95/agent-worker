package integration

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// FigmaClient handles fetching and parsing design data from Figma.
type FigmaClient struct {
	Token      string
	HTTPClient *http.Client
}

func NewFigmaClient(token string) *FigmaClient {
	return &FigmaClient{
		Token: token,
		HTTPClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// ExtractDesignTokens parses a Figma URL, fetches the node, and returns a markdown summary.
func (c *FigmaClient) ExtractDesignTokens(figmaURL string) (string, error) {
	if c.Token == "" {
		return "", fmt.Errorf("FIGMA_TOKEN is not configured")
	}

	// Example URL: https://www.figma.com/file/abc123DEF456/Design?node-id=1-2
	// Or: https://www.figma.com/design/abc123DEF456/Design?node-id=1-2
	re := regexp.MustCompile(`figma\.com/(?:file|design)/([^/]+)/.*?(?:node-id=([^&]+))`)
	matches := re.FindStringSubmatch(figmaURL)
	if len(matches) < 3 {
		return "", fmt.Errorf("invalid figma URL or missing node-id")
	}

	fileKey := matches[1]
	nodeID := matches[2]
	
	// Figma API requires ':' instead of '-' in node ids sometimes, but usually the URL has '-', let's replace URL encoded '%3A' or '-' with ':'
	nodeID = strings.ReplaceAll(nodeID, "-", ":")
	nodeID = strings.ReplaceAll(nodeID, "%3A", ":")

	apiURL := fmt.Sprintf("https://api.figma.com/v1/files/%s/nodes?ids=%s", fileKey, nodeID)
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("X-Figma-Token", c.Token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch figma API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("figma API error (status %d): %s", resp.StatusCode, string(b))
	}

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", fmt.Errorf("failed to decode figma JSON: %w", err)
	}

	nodes, ok := data["nodes"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid figma response structure")
	}

	nodeData, ok := nodes[nodeID].(map[string]interface{})
	if !ok {
		// Sometimes the node ID in the response doesn't exactly match if it was URL encoded, try getting the first node
		for _, v := range nodes {
			nodeData = v.(map[string]interface{})
			break
		}
	}
	
	if nodeData == nil {
		return "", fmt.Errorf("node not found in response")
	}

	doc, ok := nodeData["document"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("document not found in node")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Figma Design (File: %s, Node: %s)\n\n", fileKey, nodeID))
	sb.WriteString("Below is the hierarchical structure and design tokens extracted from Figma. Use these exact colors, texts, and structural hierarchies.\n\n")
	
	parseFigmaNode(&sb, doc, 0)

	return sb.String(), nil
}

func parseFigmaNode(sb *strings.Builder, node map[string]interface{}, depth int) {
	indent := strings.Repeat("  ", depth)
	
	nodeType, _ := node["type"].(string)
	name, _ := node["name"].(string)
	
	sb.WriteString(fmt.Sprintf("%s- **[%s]** %s\n", indent, nodeType, name))
	
	// Print Characters (for text nodes)
	if chars, ok := node["characters"].(string); ok {
		// remove newlines to prevent markdown breaking
		chars = strings.ReplaceAll(chars, "\n", " ")
		sb.WriteString(fmt.Sprintf("%s  *Text*: \"%s\"\n", indent, chars))
	}

	// Print Style/Typography (for text nodes)
	if style, ok := node["style"].(map[string]interface{}); ok {
		if fw, ok := style["fontWeight"].(float64); ok {
			sb.WriteString(fmt.Sprintf("%s  *Font Weight*: %v\n", indent, fw))
		}
		if fs, ok := style["fontSize"].(float64); ok {
			sb.WriteString(fmt.Sprintf("%s  *Font Size*: %vpx\n", indent, fs))
		}
	}

	// Print Colors (Fills)
	if fills, ok := node["fills"].([]interface{}); ok && len(fills) > 0 {
		for _, fillInt := range fills {
			if fill, ok := fillInt.(map[string]interface{}); ok {
				if fillType, _ := fill["type"].(string); fillType == "SOLID" {
					if color, ok := fill["color"].(map[string]interface{}); ok {
						r, _ := color["r"].(float64)
						g, _ := color["g"].(float64)
						b, _ := color["b"].(float64)
						hex := rgbToHex(r, g, b)
						sb.WriteString(fmt.Sprintf("%s  *Fill Color*: %s\n", indent, hex))
					}
				}
			}
		}
	}

	// Recurse on children
	if children, ok := node["children"].([]interface{}); ok {
		for _, childInt := range children {
			if child, ok := childInt.(map[string]interface{}); ok {
				parseFigmaNode(sb, child, depth+1)
			}
		}
	}
}

func rgbToHex(r, g, b float64) string {
	return fmt.Sprintf("#%02x%02x%02x", int(r*255), int(g*255), int(b*255))
}
