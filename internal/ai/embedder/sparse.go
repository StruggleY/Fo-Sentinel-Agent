// Package embedder 提供稀疏向量嵌入器（BM25 关键词检索）。
package embedder

import (
	"hash/fnv"
	"math"
	"regexp"
	"strings"
	"unicode"

	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

// BM25Params BM25 超参数。
var BM25Params = struct {
	K1 float32 // 词频饱和系数，通常 1.2~2.0
	B  float32 // 文档长度归一化系数，通常 0.75
}{
	K1: 1.5,
	B:  0.75,
}

// avgDocLen 全局平均文档长度估计（token 数），用于 BM25 长度归一化。
// 初始值设为 100，随实际文档近似。
const avgDocLen = 100

var reSplitter = regexp.MustCompile(`[\s\p{P}]+`)

// tokenize 将文本分词为 token 列表：
// 1. 转小写
// 2. 按空白 + 标点拆分
// 3. 过滤空串和纯标点串
func tokenize(text string) []string {
	text = strings.ToLower(text)
	parts := reSplitter.Split(text, -1)
	var tokens []string
	for _, p := range parts {
		if p == "" {
			continue
		}
		// 过滤纯标点 token
		hasLetter := false
		for _, r := range p {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				hasLetter = true
				break
			}
		}
		if hasLetter {
			tokens = append(tokens, p)
		}
	}
	return tokens
}

// hashToken 将 token 映射到 uint32 位置（使用 FNV-1a 哈希）。
// 使用 24 位空间（~16M 桶）减少碰撞。
func hashToken(token string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(token))
	return h.Sum32() & 0xFFFFFF // 24-bit
}

// BM25Embed 将文本编码为 BM25 稀疏向量。
//
// 算法说明：
//   - 对文本分词后计算词频（TF）
//   - 使用 BM25 公式：score(t,d) = TF(t,d)*(K1+1) / (TF(t,d) + K1*(1-B+B*|d|/avgDocLen))
//   - token 通过 FNV-1a 哈希映射到 uint32 位置（稀疏向量下标）
//   - 归一化：对所有分值做 L2 归一化
//
// 返回的稀疏向量可直接用于 Milvus HybridSearch sparse_vector 字段。
func BM25Embed(text string) (entity.SparseEmbedding, error) {
	tokens := tokenize(text)
	if len(tokens) == 0 {
		// 空文本：返回单一占位符稀疏向量
		positions := []uint32{0}
		values := []float32{0.0001}
		return entity.NewSliceSparseEmbedding(positions, values)
	}

	// 计算词频
	tf := make(map[string]float32)
	for _, t := range tokens {
		tf[t]++
	}

	docLen := float32(len(tokens))
	normFactor := 1.0 - BM25Params.B + BM25Params.B*docLen/avgDocLen

	// 计算 BM25 分值
	scores := make(map[uint32]float32)
	for token, freq := range tf {
		pos := hashToken(token)
		score := freq * (BM25Params.K1 + 1) / (freq + BM25Params.K1*normFactor)
		// 哈希碰撞时取最大值
		if existing, ok := scores[pos]; !ok || score > existing {
			scores[pos] = score
		}
	}

	// L2 归一化
	var sumSq float32
	for _, v := range scores {
		sumSq += v * v
	}
	norm := float32(math.Sqrt(float64(sumSq)))
	if norm == 0 {
		norm = 1
	}

	positions := make([]uint32, 0, len(scores))
	values := make([]float32, 0, len(scores))
	for pos, v := range scores {
		positions = append(positions, pos)
		values = append(values, v/norm)
	}

	return entity.NewSliceSparseEmbedding(positions, values)
}
