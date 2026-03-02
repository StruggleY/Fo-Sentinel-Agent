package common

import "github.com/milvus-io/milvus-sdk-go/v2/entity"

const (
	MilvusDBName         = "agent"
	MilvusCollectionName = "biz"

	// EmbeddingDim 是 text-embedding-v4 的 float32 向量维度。
	// 该值同时被 embedder（配置请求维度）和 Milvus Schema（FloatVector dim）引用，
	// 保证两端严格一致，改动时只需修改此处。
	EmbeddingDim = 2048
)

var FileDir = "./docs/"

// CollectionFields 是 biz 集合的唯一 Schema 定义（Single Source of Truth）。
// client.go（建表）和 indexer.go（写入）共同引用此变量，杜绝两处独立维护导致的字段漂移。
//
// 向量类型选择说明：
//   - 使用 FloatVector(2048) 而非 BinaryVector(65536)：
//     text-embedding-v4 输出 float32 向量，BinaryVector 存储后 Milvus 按 HAMMING 距离
//     对 float 原始二进制位求异或，与语义相似度无关，检索准确率极低。
//   - 配合 COSINE 度量，float 向量的余弦相似度才能真实反映语义距离。
var CollectionFields = []*entity.Field{
	{
		Name:       "id",
		DataType:   entity.FieldTypeVarChar,
		TypeParams: map[string]string{"max_length": "256"},
		PrimaryKey: true,
	},
	{
		Name:       "vector",
		DataType:   entity.FieldTypeFloatVector,
		TypeParams: map[string]string{"dim": "2048"},
	},
	{
		Name:       "content",
		DataType:   entity.FieldTypeVarChar,
		TypeParams: map[string]string{"max_length": "8192"},
	},
	{
		Name:     "metadata",
		DataType: entity.FieldTypeJSON,
	},
}
