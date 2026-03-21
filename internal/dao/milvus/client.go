package milvus

import (
	"context"
	"fmt"
	"sync"

	cli "github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

var (
	globalClient  cli.Client
	clientOnce    sync.Once
	clientInitErr error
)

// GetClient 返回全局单例 Milvus 客户端（懒初始化，线程安全）。
func GetClient(ctx context.Context) (cli.Client, error) {
	clientOnce.Do(func() {
		globalClient, clientInitErr = NewClient(ctx)
	})
	return globalClient, clientInitErr
}

// NewClient 连接 Milvus，确保目标数据库与集合存在并已加载到内存。
// 初始化顺序：连接 default DB → 创建 sentinel DB（不存在时）→ 建集合+索引（不存在时）→ 创建分区 → 加载集合。
func NewClient(ctx context.Context) (cli.Client, error) {
	// 必须先通过 default DB 创建目标数据库
	defaultClient, err := cli.NewClient(ctx, cli.Config{Address: "localhost:19530", DBName: "default"})
	if err != nil {
		return nil, fmt.Errorf("连接 Milvus default 库失败: %w", err)
	}

	databases, err := defaultClient.ListDatabases(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取数据库列表失败: %w", err)
	}
	dbExists := false
	for _, db := range databases {
		if db.Name == DBName {
			dbExists = true
			break
		}
	}
	if !dbExists {
		if err = defaultClient.CreateDatabase(ctx, DBName); err != nil {
			return nil, fmt.Errorf("创建数据库 %s 失败: %w", DBName, err)
		}
	}
	defaultClient.Close()

	sentinelClient, err := cli.NewClient(ctx, cli.Config{Address: "localhost:19530", DBName: DBName})
	if err != nil {
		return nil, fmt.Errorf("连接 Milvus %s 库失败: %w", DBName, err)
	}

	collections, err := sentinelClient.ListCollections(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取集合列表失败: %w", err)
	}
	collExists := false
	for _, c := range collections {
		if c.Name == CollectionName {
			collExists = true
			break
		}
	}

	if !collExists {
		schema := &entity.Schema{
			CollectionName: CollectionName,
			Description:    "Security sentinel knowledge collection",
			Fields:         CollectionFields,
		}
		if err = sentinelClient.CreateCollection(ctx, schema, entity.DefaultShardNumber); err != nil {
			return nil, fmt.Errorf("创建集合 %s 失败: %w", CollectionName, err)
		}

		// 稠密向量索引：COSINE 余弦度量
		denseIndex, err := entity.NewIndexAUTOINDEX(entity.COSINE)
		if err != nil {
			return nil, fmt.Errorf("创建稠密向量索引失败: %w", err)
		}
		if err = sentinelClient.CreateIndex(ctx, CollectionName, "vector", denseIndex, false); err != nil {
			return nil, fmt.Errorf("写入稠密向量索引失败: %w", err)
		}

		// 稀疏向量索引：SPARSE_INVERTED_INDEX + IP 度量（BM25 使用内积）
		sparseIndex, err := entity.NewIndexSparseInverted(entity.IP, 0.0)
		if err != nil {
			return nil, fmt.Errorf("创建稀疏向量索引失败: %w", err)
		}
		if err = sentinelClient.CreateIndex(ctx, CollectionName, "sparse_vector", sparseIndex, false); err != nil {
			return nil, fmt.Errorf("写入稀疏向量索引失败: %w", err)
		}

		// 创建两个分区：事件向量 / 知识文档向量
		if err = sentinelClient.CreatePartition(ctx, CollectionName, PartitionEvents); err != nil {
			return nil, fmt.Errorf("创建分区 %s 失败: %w", PartitionEvents, err)
		}
		if err = sentinelClient.CreatePartition(ctx, CollectionName, PartitionDocuments); err != nil {
			return nil, fmt.Errorf("创建分区 %s 失败: %w", PartitionDocuments, err)
		}
	}

	// 集合必须加载到内存才能查询；重复加载 Milvus 会忽略
	if err = sentinelClient.LoadCollection(ctx, CollectionName, false); err != nil {
		return nil, fmt.Errorf("加载集合 %s 失败: %w", CollectionName, err)
	}
	return sentinelClient, nil
}
