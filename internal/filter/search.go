package filter

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"zai-proxy/internal/model"
)

var searchRefPattern = regexp.MustCompile(`【turn\d+search(\d+)】`)
var searchRefPrefixPattern = regexp.MustCompile(`【(t(u(r(n(\d+(s(e(a(r(c(h(\d+)?)?)?)?)?)?)?)?)?)?)?)?$`)

type SearchRefFilter struct {
	buffer        string
	searchResults map[string]model.SearchResult
}

func NewSearchRefFilter() *SearchRefFilter {
	return &SearchRefFilter{
		searchResults: make(map[string]model.SearchResult),
	}
}

func (f *SearchRefFilter) AddSearchResults(results []model.SearchResult) {
	for _, r := range results {
		f.searchResults[r.RefID] = r
	}
}

func escapeMarkdownTitle(title string) string {
	title = strings.ReplaceAll(title, `\`, `\\`)
	title = strings.ReplaceAll(title, `[`, `\[`)
	title = strings.ReplaceAll(title, `]`, `\]`)
	return title
}

func (f *SearchRefFilter) Process(content string) string {
	content = f.buffer + content
	f.buffer = ""

	if content == "" {
		return ""
	}

	content = searchRefPattern.ReplaceAllStringFunc(content, func(match string) string {
		runes := []rune(match)
		refID := string(runes[1 : len(runes)-1])
		if result, ok := f.searchResults[refID]; ok {
			return fmt.Sprintf(`[\[%d\]](%s)`, result.Index, result.URL)
		}
		return ""
	})

	if content == "" {
		return ""
	}

	maxPrefixLen := 20
	if len(content) < maxPrefixLen {
		maxPrefixLen = len(content)
	}

	for i := 1; i <= maxPrefixLen; i++ {
		suffix := content[len(content)-i:]
		if searchRefPrefixPattern.MatchString(suffix) {
			f.buffer = suffix
			return content[:len(content)-i]
		}
	}

	return content
}

func (f *SearchRefFilter) Flush() string {
	result := f.buffer
	f.buffer = ""
	if result != "" {
		result = searchRefPattern.ReplaceAllStringFunc(result, func(match string) string {
			runes := []rune(match)
			refID := string(runes[1 : len(runes)-1])
			if r, ok := f.searchResults[refID]; ok {
				return fmt.Sprintf(`[\[%d\]](%s)`, r.Index, r.URL)
			}
			return ""
		})
	}
	return result
}

func (f *SearchRefFilter) GetSearchResultsMarkdown() string {
	if len(f.searchResults) == 0 {
		return ""
	}

	var results []model.SearchResult
	for _, r := range f.searchResults {
		results = append(results, r)
	}
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[i].Index > results[j].Index {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	var sb strings.Builder
	for _, r := range results {
		escapedTitle := escapeMarkdownTitle(r.Title)
		sb.WriteString(fmt.Sprintf("[\\[%d\\] %s](%s)\n", r.Index, escapedTitle, r.URL))
	}
	sb.WriteString("\n")
	return sb.String()
}

func IsSearchResultContent(editContent string) bool {
	return strings.Contains(editContent, `"search_result"`)
}

func ParseSearchResults(editContent string) []model.SearchResult {
	searchResultKey := `"search_result":`
	idx := strings.Index(editContent, searchResultKey)
	if idx == -1 {
		return nil
	}

	startIdx := idx + len(searchResultKey)
	for startIdx < len(editContent) && editContent[startIdx] != '[' {
		startIdx++
	}
	if startIdx >= len(editContent) {
		return nil
	}

	bracketCount := 0
	endIdx := startIdx
	for endIdx < len(editContent) {
		if editContent[endIdx] == '[' {
			bracketCount++
		} else if editContent[endIdx] == ']' {
			bracketCount--
			if bracketCount == 0 {
				endIdx++
				break
			}
		}
		endIdx++
	}

	if bracketCount != 0 {
		return nil
	}

	jsonStr := editContent[startIdx:endIdx]
	var rawResults []struct {
		Title string `json:"title"`
		URL   string `json:"url"`
		Index int    `json:"index"`
		RefID string `json:"ref_id"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &rawResults); err != nil {
		return nil
	}

	var results []model.SearchResult
	for _, r := range rawResults {
		results = append(results, model.SearchResult{
			Title: r.Title,
			URL:   r.URL,
			Index: r.Index,
			RefID: r.RefID,
		})
	}

	return results
}

func IsSearchToolCall(editContent string, phase string) bool {
	if phase != "tool_call" {
		return false
	}
	return strings.Contains(editContent, `"mcp"`) || strings.Contains(editContent, `mcp-server`)
}

func ParseImageSearchResults(editContent string) []model.ImageSearchResult {
	resultKey := `"result":`
	idx := strings.Index(editContent, resultKey)
	if idx == -1 {
		return nil
	}

	startIdx := idx + len(resultKey)
	for startIdx < len(editContent) && editContent[startIdx] != '[' {
		startIdx++
	}
	if startIdx >= len(editContent) {
		return nil
	}

	bracketCount := 0
	endIdx := startIdx
	inString := false
	escapeNext := false
	for endIdx < len(editContent) {
		ch := editContent[endIdx]

		if escapeNext {
			escapeNext = false
			endIdx++
			continue
		}

		if ch == '\\' {
			escapeNext = true
			endIdx++
			continue
		}

		if ch == '"' {
			inString = !inString
		}

		if !inString {
			if ch == '[' || ch == '{' {
				bracketCount++
			} else if ch == ']' || ch == '}' {
				bracketCount--
				if bracketCount == 0 && ch == ']' {
					endIdx++
					break
				}
			}
		}
		endIdx++
	}

	if bracketCount != 0 {
		return nil
	}

	jsonStr := editContent[startIdx:endIdx]

	var rawResults []map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &rawResults); err != nil {
		return nil
	}

	var results []model.ImageSearchResult
	for _, item := range rawResults {
		if itemType, ok := item["type"].(string); ok && itemType == "text" {
			if text, ok := item["text"].(string); ok {
				result := parseImageSearchText(text)
				if result.Title != "" && result.Link != "" {
					results = append(results, result)
				}
			}
		}
	}

	return results
}

func parseImageSearchText(text string) model.ImageSearchResult {
	result := model.ImageSearchResult{}

	if titleIdx := strings.Index(text, "Title: "); titleIdx != -1 {
		titleStart := titleIdx + len("Title: ")
		titleEnd := strings.Index(text[titleStart:], ";")
		if titleEnd != -1 {
			result.Title = strings.TrimSpace(text[titleStart : titleStart+titleEnd])
		}
	}

	if linkIdx := strings.Index(text, "Link: "); linkIdx != -1 {
		linkStart := linkIdx + len("Link: ")
		linkEnd := strings.Index(text[linkStart:], ";")
		if linkEnd != -1 {
			result.Link = strings.TrimSpace(text[linkStart : linkStart+linkEnd])
		} else {
			result.Link = strings.TrimSpace(text[linkStart:])
		}
	}

	if thumbnailIdx := strings.Index(text, "Thumbnail: "); thumbnailIdx != -1 {
		thumbnailStart := thumbnailIdx + len("Thumbnail: ")
		result.Thumbnail = strings.TrimSpace(text[thumbnailStart:])
	}

	return result
}

func FormatImageSearchResults(results []model.ImageSearchResult) string {
	if len(results) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, r := range results {
		escapedTitle := strings.ReplaceAll(r.Title, `[`, `\[`)
		escapedTitle = strings.ReplaceAll(escapedTitle, `]`, `\]`)
		sb.WriteString(fmt.Sprintf("\n![%s](%s)", escapedTitle, r.Link))
	}
	sb.WriteString("\n")
	return sb.String()
}

func ExtractTextBeforeGlmBlock(editContent string) string {
	if idx := strings.Index(editContent, "<glm_block"); idx != -1 {
		text := editContent[:idx]
		if strings.HasSuffix(text, "\n") {
			text = text[:len(text)-1]
		}
		return text
	}
	return ""
}
