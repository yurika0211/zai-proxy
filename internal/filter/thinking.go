package filter

import "strings"

type ThinkingFilter struct {
	HasSeenFirstThinking bool
	Buffer               string
	LastOutputChunk      string
	LastPhase            string
	ThinkingRoundCount   int
}

func (f *ThinkingFilter) ProcessThinking(deltaContent string) string {
	if !f.HasSeenFirstThinking {
		f.HasSeenFirstThinking = true
		if idx := strings.Index(deltaContent, "> "); idx != -1 {
			deltaContent = deltaContent[idx+2:]
		} else {
			return ""
		}
	}

	content := f.Buffer + deltaContent
	f.Buffer = ""

	content = strings.ReplaceAll(content, "\n> ", "\n")

	if strings.HasSuffix(content, "\n>") {
		f.Buffer = "\n>"
		return content[:len(content)-2]
	}
	if strings.HasSuffix(content, "\n") {
		f.Buffer = "\n"
		return content[:len(content)-1]
	}

	return content
}

func (f *ThinkingFilter) Flush() string {
	result := f.Buffer
	f.Buffer = ""
	return result
}

func (f *ThinkingFilter) ExtractCompleteThinking(editContent string) string {
	startIdx := strings.Index(editContent, "> ")
	if startIdx == -1 {
		return ""
	}
	startIdx += 2

	endIdx := strings.Index(editContent, "\n</details>")
	if endIdx == -1 {
		return ""
	}

	content := editContent[startIdx:endIdx]
	content = strings.ReplaceAll(content, "\n> ", "\n")
	return content
}

func (f *ThinkingFilter) ExtractIncrementalThinking(editContent string) string {
	completeThinking := f.ExtractCompleteThinking(editContent)
	if completeThinking == "" {
		return ""
	}

	if f.LastOutputChunk == "" {
		return completeThinking
	}

	idx := strings.Index(completeThinking, f.LastOutputChunk)
	if idx == -1 {
		return completeThinking
	}

	incrementalPart := completeThinking[idx+len(f.LastOutputChunk):]
	return incrementalPart
}

func (f *ThinkingFilter) ResetForNewRound() {
	f.LastOutputChunk = ""
	f.HasSeenFirstThinking = false
}
