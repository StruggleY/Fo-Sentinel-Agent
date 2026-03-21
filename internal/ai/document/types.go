package document

// ChunkStrategy 文档分块策略。
type ChunkStrategy string

const (
	// StrategyFixedSize 固定大小分块（滑动窗口 + 重叠），适合大多数场景。
	StrategyFixedSize ChunkStrategy = "fixed_size"
	// StrategyStructureAware 结构感知分块（按 Markdown 标题层级 + 段落贪心合并）。
	StrategyStructureAware ChunkStrategy = "structure_aware"
	// StrategyHierarchical 父子分块（父块~512tokens，子块~128tokens），
	// 子块存 Milvus 向量，metadata 携带父块完整文本供 LLM 使用。
	StrategyHierarchical ChunkStrategy = "hierarchical"
)

// ChunkConfig 分块配置，通过 IndexInput 在运行时传入，无需重建 Graph。
type ChunkConfig struct {
	Strategy        ChunkStrategy // 分块策略，默认 fixed_size
	ChunkSize       int           // fixed_size/子块 用：目标块大小（rune），默认 512
	OverlapSize     int           // fixed_size/子块 用：重叠大小（rune），默认 128
	TargetChars     int           // structure_aware 用：目标块大小（rune），默认 1400
	MaxChars        int           // structure_aware 用：最大块大小（rune），默认 1800
	MinChars        int           // structure_aware 用：最小块大小（rune），默认 600
	ParentChunkSize int           // hierarchical 用：父块大小（rune），默认 1024
	ChildChunkSize  int           // hierarchical 用：子块大小（rune），默认 256
	ChildOverlap    int           // hierarchical 用：子块重叠（rune），默认 40
}

// DefaultChunkConfig 返回推荐的默认分块配置（fixed_size 512/128）。
func DefaultChunkConfig() ChunkConfig {
	return ChunkConfig{
		Strategy:        StrategyFixedSize,
		ChunkSize:       512,
		OverlapSize:     128,
		TargetChars:     1400,
		MaxChars:        1800,
		MinChars:        600,
		ParentChunkSize: 1024,
		ChildChunkSize:  256,
		ChildOverlap:    40,
	}
}

// DefaultHierarchicalConfig 返回推荐的父子分块配置。
func DefaultHierarchicalConfig() ChunkConfig {
	return ChunkConfig{
		Strategy:        StrategyHierarchical,
		ParentChunkSize: 1024,
		ChildChunkSize:  256,
		ChildOverlap:    40,
	}
}

// ParsedSection 文档中的一个章节，包含标题层级和正文内容。
type ParsedSection struct {
	Level   int    // 标题层级：1=H1, 2=H2, 3=H3, 0=无标题（文档开头无标题的内容）
	Title   string // 章节标题（不含 # 前缀）
	Content string // 章节正文（不含标题行本身）
}

// ParsedDocument 结构化解析结果，包含文档主标题和按章节划分的内容。
type ParsedDocument struct {
	Title    string          // 文档主标题（首个 H1 标题，或文件名）
	Sections []ParsedSection // 按标题层级划分的节
}

// ChunkResult 父子分块的单个子块结果。
// 子块存入 Milvus 做向量检索，命中时通过 ParentContent 返回更完整的上下文给 LLM。
type ChunkResult struct {
	ID            string // 子块ID（由外层赋值，对应 Milvus ID）
	Content       string // 子块文本（用于向量化，长度 ~ChildChunkSize）
	ParentContent string // 父块文本（检索命中后返回给 LLM，长度 ~ParentChunkSize）
	SectionTitle  string // 所在章节标题（metadata 注入，辅助 LLM 理解上下文）
	ChunkIndex    int    // 子块在文档中的全局序号（从 0 开始）
	CharCount     int    // 子块字符数
}
