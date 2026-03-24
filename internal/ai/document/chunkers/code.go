package chunkers

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// codePatterns 各语言的顶层声明边界正则，用于识别函数/类/接口定义。
var codePatterns = map[string][]*regexp.Regexp{
	"go": {
		regexp.MustCompile(`(?m)^func\s+`),
		regexp.MustCompile(`(?m)^type\s+\w+\s+struct`),
		regexp.MustCompile(`(?m)^type\s+\w+\s+interface`),
	},
	"python": {
		regexp.MustCompile(`(?m)^def\s+\w+`),
		regexp.MustCompile(`(?m)^class\s+\w+`),
		regexp.MustCompile(`(?m)^async\s+def\s+\w+`),
	},
	"java": {
		regexp.MustCompile(`(?m)^\s*(public|private|protected)?\s*(static)?\s+\w+\s+\w+\s*\(`),
		regexp.MustCompile(`(?m)^\s*(public|private|protected)?\s+class\s+\w+`),
		regexp.MustCompile(`(?m)^\s*(public|private|protected)?\s+interface\s+\w+`),
	},
	"javascript": {
		regexp.MustCompile(`(?m)^function\s+\w+`),
		regexp.MustCompile(`(?m)^class\s+\w+`),
		regexp.MustCompile(`(?m)^const\s+\w+\s*=\s*\(`),
		regexp.MustCompile(`(?m)^export\s+(function|class|const)`),
	},
	"typescript": {
		regexp.MustCompile(`(?m)^function\s+\w+`),
		regexp.MustCompile(`(?m)^class\s+\w+`),
		regexp.MustCompile(`(?m)^const\s+\w+\s*=\s*\(`),
		regexp.MustCompile(`(?m)^export\s+(function|class|const|interface)`),
		regexp.MustCompile(`(?m)^interface\s+\w+`),
	},
}

// Code 按函数/类/接口边界切分代码文件，超过 maxSize 时强制截断。
// language 支持：go / python / java / javascript / typescript。
// 未识别语言降级为 SlidingWindow 分块。
func Code(text, language string, maxSize int) []ChunkResult {
	// maxSize 默认值必须在首次使用前设置（含未识别语言的降级路径）
	if maxSize <= 0 {
		maxSize = 1024
	}

	patterns, ok := codePatterns[strings.ToLower(language)]
	if !ok {
		// 未知语言：降级为滑动窗口分块
		raw := SlidingWindow(text, maxSize, 0)
		results := make([]ChunkResult, len(raw))
		for i, chunk := range raw {
			results[i] = ChunkResult{
				Content:    chunk,
				ChunkIndex: i,
				CharCount:  utf8.RuneCountInString(chunk),
			}
		}
		return results
	}

	lines := strings.Split(text, "\n")
	var results []ChunkResult
	var cur strings.Builder
	chunkIndex := 0

	// flush 提交当前缓冲区为一个 ChunkResult，非空时才追加
	flush := func() {
		content := strings.TrimSpace(cur.String())
		if content != "" {
			results = append(results, ChunkResult{
				Content:    content,
				ChunkIndex: chunkIndex,
				CharCount:  utf8.RuneCountInString(content),
			})
			chunkIndex++
		}
		cur.Reset()
	}

	for i, line := range lines {
		// 检测是否命中边界模式（函数/类/接口声明行）
		isBoundary := false
		for _, pattern := range patterns {
			if pattern.MatchString(line) {
				isBoundary = true
				break
			}
		}

		// 遇到边界且缓冲区非空时，先提交前一段代码
		if isBoundary && cur.Len() > 0 {
			flush()
		}

		cur.WriteString(line)
		if i < len(lines)-1 {
			cur.WriteString("\n")
		}

		// 单块超过 maxSize 时强制截断，防止 LLM 上下文溢出
		if utf8.RuneCountInString(cur.String()) > maxSize {
			flush()
		}
	}

	// 提交最后一段
	if cur.Len() > 0 {
		flush()
	}

	return results
}
