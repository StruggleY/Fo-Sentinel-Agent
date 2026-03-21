package document

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// Chunk 根据配置的策略将文本切分为若干块，返回字符串切片。
//
// 适用于不需要父子结构或章节元数据的简单索引场景。
// 若需要完整元数据（ParentContent / SectionTitle / ChunkIndex），
// 请直接调用 HierarchicalChunk 或 HierarchicalChunkFromDocument。
//
// 策略选择建议：
//   - StrategyFixedSize：通用场景，计算开销最低
//   - StrategyStructureAware：Markdown 文档，保留标题语义边界
//   - StrategyHierarchical：知识库上传默认策略，兼顾检索精度与 LLM 上下文完整性
func Chunk(text string, cfg ChunkConfig) []string {
	switch cfg.Strategy {
	case StrategyStructureAware:
		return structureAwareChunk(text, cfg)
	case StrategyHierarchical:
		results := HierarchicalChunk(text, "", cfg)
		chunks := make([]string, len(results))
		for i, r := range results {
			chunks[i] = r.Content
		}
		return chunks
	default:
		return fixedSizeChunk(text, cfg)
	}
}

// HierarchicalChunk 父子分块：先将文本切分为父块（~ParentChunkSize），
// 再将每个父块细切为子块（~ChildChunkSize），子块携带所在父块的完整文本。
//
// 向量化与检索策略：
//   - 向量化：对子块文本进行嵌入，子块更短、语义更聚焦，向量质量更高
//   - 检索：命中子块向量 → 读取 ChunkResult.ParentContent → 注入父块完整上下文给 LLM
//   - 效果：兼顾检索精度（子块小）与上下文完整性（父块大）
//
// 参数说明：
//   - text: 待分块的原始文本
//   - sectionTitle: 章节标题，由 HierarchicalChunkFromDocument 注入；直接调用时可传空串
//   - cfg.ParentChunkSize: 父块大小（rune数），默认 1024
//   - cfg.ChildChunkSize:  子块大小（rune数），默认 256
//   - cfg.ChildOverlap:    子块间重叠（rune数），默认 0
//
// 返回的 ChunkResult.ChunkIndex 为本次调用内的连续编号（从 0 起），
// 跨章节的全局编号由 HierarchicalChunkFromDocument 负责重写。
func HierarchicalChunk(text, sectionTitle string, cfg ChunkConfig) []ChunkResult {
	parentSize := cfg.ParentChunkSize
	childSize := cfg.ChildChunkSize
	childOverlap := cfg.ChildOverlap
	if parentSize <= 0 {
		parentSize = 1024
	}
	if childSize <= 0 {
		childSize = 256
	}
	if childOverlap < 0 {
		childOverlap = 0
	}

	// 1. 切父块（固定大小，无重叠，保证每个子块只属于唯一父块）
	parentCfg := ChunkConfig{
		Strategy:    StrategyFixedSize,
		ChunkSize:   parentSize,
		OverlapSize: 0,
	}
	parents := fixedSizeChunk(text, parentCfg)

	// 2. 每个父块再切子块，预分配容量减少 GC（平均每父块 parentSize/childSize 个子块）
	childCfg := ChunkConfig{
		Strategy:    StrategyFixedSize,
		ChunkSize:   childSize,
		OverlapSize: childOverlap,
	}
	estimatedChildren := (parentSize / childSize) + 1
	results := make([]ChunkResult, 0, len(parents)*estimatedChildren)

	globalIdx := 0
	for _, parent := range parents {
		children := fixedSizeChunk(parent, childCfg)
		if len(children) == 0 {
			// 父块极短（≤ childSize）时直接作为唯一子块，避免丢失内容
			children = []string{parent}
		}
		for _, child := range children {
			results = append(results, ChunkResult{
				Content:       child,
				ParentContent: parent,
				SectionTitle:  sectionTitle,
				ChunkIndex:    globalIdx,
				CharCount:     utf8.RuneCountInString(child), // 子块字符数（rune），不含父块
			})
			globalIdx++
		}
	}
	return results
}

// HierarchicalChunkFromDocument 从结构化解析结果生成父子分块。
//
// 每个章节独立调用 HierarchicalChunk，子块不跨章节边界，
// SectionTitle 取当前章节标题（缺省时回退到文档标题）。
// 各章节分块完成后，ChunkIndex 被重写为文档级全局连续编号（0, 1, 2, ...），
// 供写入 Milvus / knowledge_chunks 时作为唯一定位键。
func HierarchicalChunkFromDocument(doc *ParsedDocument, cfg ChunkConfig) []ChunkResult {
	var results []ChunkResult
	globalIdx := 0
	for _, section := range doc.Sections {
		if strings.TrimSpace(section.Content) == "" {
			continue
		}
		// 章节标题：使用 "H1标题 > H2标题 > ..." 层级路径
		title := section.Title
		if title == "" {
			title = doc.Title
		}
		chunks := HierarchicalChunk(section.Content, title, cfg)
		for i := range chunks {
			chunks[i].ChunkIndex = globalIdx
			globalIdx++
		}
		results = append(results, chunks...)
	}
	return results
}

// ── Fixed Size Chunker ────────────────────────────────────────────────────────

var reURLBreak = regexp.MustCompile(`([a-zA-Z0-9])\.\n([a-zA-Z])`)

// normalizeText 修复 PDF/DOCX 解析时 URL 因换行被错误断开的情况。
// 处理形如 "github.com\ngoogles" → "github.com googles" 的断行场景。
// 注意：仅匹配 "字母数字.换行字母" 的模式，对 "https://\n" 等前缀断行无效。
func normalizeText(text string) string {
	return reURLBreak.ReplaceAllString(text, "$1.$2")
}

// fixedSizeChunk 滑动窗口分块，在优先边界（\n > 中文句末 > 英文句末）处对齐。
//
// 算法：
//  1. normalizeText：修复解析产物中 URL 换行断开
//  2. 计算 [start, end) 窗口，end = start + chunkSize
//  3. 当 end 未到文本末尾时，adjustToBoundary 向前寻找最优切分点
//  4. 下一个 start = end - overlapSize（确保相邻块有 overlapSize 个 rune 的重叠上下文）
//
// 边界条件：
//   - chunkSize <= 0 时使用默认值 512
//   - overlapSize < 0 时修正为 0
//   - nextStart <= start 时强制 +1，防止无限循环（极端配置下的兜底）
func fixedSizeChunk(text string, cfg ChunkConfig) []string {
	text = normalizeText(text)
	runes := []rune(text)
	total := len(runes)
	if total == 0 {
		return nil
	}

	chunkSize := cfg.ChunkSize
	overlapSize := cfg.OverlapSize
	if chunkSize <= 0 {
		chunkSize = 512
	}
	if overlapSize < 0 {
		overlapSize = 0
	}

	var chunks []string
	start := 0
	for start < total {
		end := start + chunkSize
		if end > total {
			end = total
		} else {
			end = adjustToBoundary(runes, end, overlapSize)
		}

		chunk := strings.TrimSpace(string(runes[start:end]))
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
		if end >= total {
			break
		}
		nextStart := end - overlapSize
		if nextStart <= start {
			nextStart = start + 1 // 防死循环
		}
		start = nextStart
	}
	return chunks
}

// adjustToBoundary 在 [targetEnd-lookback, targetEnd] 范围内向前查找最优切分边界，
// 优先级：换行符 > 中文句末标点（。！？）> 英文句末（.!? 后跟空白）。
// 三级均未命中时返回原始 targetEnd，即直接按字符数截断。
//
// 安全保证：
//   - lookback = min(maxLookback, targetEnd)，确保下界 >= 0
//   - 前两级循环访问 runes[i-1]，下界限制为 i >= 1（lo = max(lo, 1)）
//   - 第三级循环访问 runes[i-2]，下界限制为 i >= 2（lo3 = max(lo3, 2)）
func adjustToBoundary(runes []rune, targetEnd, maxLookback int) int {
	lookback := maxLookback
	if lookback > targetEnd {
		lookback = targetEnd
	}

	// 计算通用下界（>=1 保证 runes[i-1] 合法）
	lo := targetEnd - lookback
	if lo < 1 {
		lo = 1
	}

	// 优先：换行符
	for i := targetEnd; i >= lo; i-- {
		if runes[i-1] == '\n' {
			return i
		}
	}
	// 次优：中文句末标点
	for i := targetEnd; i >= lo; i-- {
		r := runes[i-1]
		if r == '。' || r == '！' || r == '？' {
			return i
		}
	}
	// 再次：英文句末（.!? 后跟空白），需同时访问 runes[i-2]，下界提升至 2
	lo3 := lo
	if lo3 < 2 {
		lo3 = 2
	}
	for i := targetEnd; i >= lo3; i-- {
		r := runes[i-2]
		next := runes[i-1]
		if (r == '.' || r == '!' || r == '?') && (next == ' ' || next == '\t' || next == '\n') {
			return i
		}
	}
	return targetEnd
}

// ── Structure Aware Chunker ───────────────────────────────────────────────────

type blockType int

const (
	blockHeading   blockType = iota // Markdown 标题行（# ~ ######）
	blockCodeFence                  // 代码围栏开/闭行（```），触发 inCode 状态翻转
	blockAtomic                     // 代码块内容行（inCode=true 时），不可在此处切分
	blockPara                       // 普通段落行（默认类型）
)

type block struct {
	kind    blockType
	content string
	size    int // rune 数（含行尾 \n）
}

var (
	reHeading   = regexp.MustCompile(`^#{1,6}\s`)
	reCodeFence = regexp.MustCompile("^```")
)

// buildBlocks 逐行扫描文本，按 Markdown 语义识别并标注块类型。
//
// 状态机：inCode 标志跟踪当前是否在代码围栏内。
// 遇到 ``` 行时切换 inCode；inCode=true 期间所有内容标注为 blockAtomic（不可分割）。
// 注意：文档末尾仍处于 inCode=true（代码围栏未闭合）属于合法降级，
// 末尾代码内容会被保留在最后一块中，不影响分块完整性。
func buildBlocks(text string) []block {
	lines := strings.Split(text, "\n")
	var blocks []block
	inCode := false

	for _, line := range lines {
		size := utf8.RuneCountInString(line) + 1 // +1 for \n
		switch {
		case reCodeFence.MatchString(line):
			blocks = append(blocks, block{kind: blockCodeFence, content: line + "\n", size: size})
			inCode = !inCode
		case inCode:
			blocks = append(blocks, block{kind: blockAtomic, content: line + "\n", size: size})
		case reHeading.MatchString(line):
			blocks = append(blocks, block{kind: blockHeading, content: line + "\n", size: size})
		default:
			blocks = append(blocks, block{kind: blockPara, content: line + "\n", size: size})
		}
	}
	return blocks
}

// structureAwareChunk 结构感知分块。
//
// 分块触发条件（优先级从高到低）：
//  1. 遇到新标题（blockHeading）且当前块 >= minChars → 提交当前块，新标题开启新块
//  2. 累计字符超过 maxChars 且 >= minChars → 强制提交后追加当前行
//  3. 累计字符达到 targetChars 且当前行非代码（安全边界）→ 提交当前块
//
// 后处理：
//   - 尾部碎片（< minChars/2）并入前一块，减少极小分块
//   - 单块超过 maxChars*1.5 时退化为 fixedSizeChunk 二次分块（处理无标题长段落）
//
// 注意：fallback 二次分块使用独立的 FixedSize 配置（ChunkSize=512），
// 不继承 cfg.TargetChars/MaxChars/MinChars，以避免递归调用 structureAwareChunk。
func structureAwareChunk(text string, cfg ChunkConfig) []string {
	targetChars := cfg.TargetChars
	maxChars := cfg.MaxChars
	minChars := cfg.MinChars
	if targetChars <= 0 {
		targetChars = 1400
	}
	if maxChars <= 0 {
		maxChars = 1800
	}
	if minChars <= 0 {
		minChars = 600
	}

	blocks := buildBlocks(text)
	var chunks []string
	var curBuilder strings.Builder
	curSize := 0

	flush := func() {
		s := strings.TrimSpace(curBuilder.String())
		if s != "" {
			chunks = append(chunks, s)
		}
		curBuilder.Reset()
		curSize = 0
	}

	for _, b := range blocks {
		projected := curSize + b.size
		switch b.kind {
		case blockHeading:
			// 新标题前提交当前块（若已足够大）
			if curSize >= minChars {
				flush()
			}
			curBuilder.WriteString(b.content)
			curSize += b.size
		default:
			if projected > maxChars && curSize >= minChars {
				flush()
			}
			curBuilder.WriteString(b.content)
			curSize += b.size
		}

		// 达到目标大小后在安全边界（非代码围栏/代码行）提交
		if curSize >= targetChars && b.kind != blockCodeFence && b.kind != blockAtomic {
			flush()
		}
	}
	flush()

	// 尾部过小的 chunk（< minChars/2）并入前一个，减少碎片
	if len(chunks) >= 2 {
		last := chunks[len(chunks)-1]
		if utf8.RuneCountInString(last) < minChars/2 {
			chunks[len(chunks)-2] += "\n" + last
			chunks = chunks[:len(chunks)-1]
		}
	}

	// fallback：单 chunk 超过 maxChars*1.5 时退化为 fixedSize 二次分块。
	// 使用独立配置（ChunkSize=512, OverlapSize=0），不依赖 cfg 的 StructureAware 参数，
	// 避免因 cfg.ChunkSize=0 而触发 fixedSizeChunk 内部默认值的隐式依赖。
	threshold := maxChars * 3 / 2
	fallbackCfg := ChunkConfig{Strategy: StrategyFixedSize, ChunkSize: 512, OverlapSize: 0}
	var result []string
	for _, c := range chunks {
		if utf8.RuneCountInString(c) > threshold {
			result = append(result, fixedSizeChunk(c, fallbackCfg)...)
		} else {
			result = append(result, c)
		}
	}
	return result
}
