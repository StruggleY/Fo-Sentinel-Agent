package client

import (
	"Fo-Sentinel-Agent/utility/common"
	"context"
	"fmt"

	cli "github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

// NewMilvusClient 创建并初始化一个连接到Milvus向量数据库的客户端
// 该函数会完成以下操作:
// 1. 检查并创建所需的数据库(如果不存在)
// 2. 检查并创建所需的集合(collection)及其schema
// 3. 为集合字段创建索引以优化查询性能
// 4. 加载集合到内存中以提供快速访问
// 参数:
//   - ctx: 上下文对象,用于控制请求的生命周期
//
// 返回值:
//   - cli.Client: 已配置好的Milvus客户端实例
//   - error: 如果初始化过程中出现错误则返回
func NewMilvusClient(ctx context.Context) (cli.Client, error) {
	// 步骤1: 先连接到Milvus的默认数据库(default)
	// 因为创建新数据库需要通过default数据库的连接来操作
	defaultClient, err := cli.NewClient(ctx, cli.Config{
		Address: "localhost:19530", // Milvus服务器地址和端口
		DBName:  "default",         // 连接到默认数据库
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to default database: %w", err)
	}

	// 步骤2: 检查目标数据库(agent数据库)是否存在，不存在则创建
	// 获取所有已存在的数据库列表
	databases, err := defaultClient.ListDatabases(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list databases: %w", err)
	}

	// 遍历数据库列表,检查是否已存在agent数据库
	agentDBExists := false
	for _, db := range databases {
		if db.Name == common.MilvusDBName { // common.MilvusDBName定义了agent数据库的名称
			agentDBExists = true
			break
		}
	}

	// 如果agent数据库不存在,则创建它
	if !agentDBExists {
		err = defaultClient.CreateDatabase(ctx, common.MilvusDBName)
		if err != nil {
			return nil, fmt.Errorf("failed to create agent database: %w", err)
		}
	}

	// 步骤3: 创建一个新的客户端连接,这次连接到agent数据库
	// 关闭之前的default数据库连接,使用agent数据库进行后续操作
	agentClient, err := cli.NewClient(ctx, cli.Config{
		Address: "localhost:19530",   // 相同的Milvus服务器地址
		DBName:  common.MilvusDBName, // 连接到刚创建或已存在的agent数据库
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to agent database: %w", err)
	}

	// 步骤4: 检查业务知识集合(biz collection)是否存在,不存在则创建
	// 集合(collection)相当于关系数据库中的表,用于存储向量数据
	collections, err := agentClient.ListCollections(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list collections: %w", err)
	}

	// 遍历集合列表,检查目标集合是否已存在
	bizCollectionExists := false
	for _, collection := range collections {
		if collection.Name == common.MilvusCollectionName { // 业务知识集合的名称
			bizCollectionExists = true
			break
		}
	}

	if !bizCollectionExists {
		// 如果集合不存在，则创建它。
		// Schema 字段统一使用 common.CollectionFields（单一事实来源），
		// 与 indexer.go 共享同一份定义，避免两处独立维护导致字段漂移。
		schema := &entity.Schema{
			CollectionName: common.MilvusCollectionName,
			Description:    "Business knowledge collection",
			Fields:         common.CollectionFields,
		}

		// 创建集合,使用默认的分片数量
		err = agentClient.CreateCollection(ctx, schema, entity.DefaultShardNumber)
		if err != nil {
			return nil, fmt.Errorf("failed to create biz collection: %w", err)
		}

		// 只为 vector 字段创建索引。
		// 使用 COSINE 度量：FloatVector 的余弦相似度能真实反映语义距离，
		// 比 HAMMING（二进制异或）更适合文本 Embedding 的相似度检索。
		vectorIndex, err := entity.NewIndexAUTOINDEX(entity.COSINE)
		if err != nil {
			return nil, fmt.Errorf("failed to create vector index: %w", err)
		}
		err = agentClient.CreateIndex(ctx, common.MilvusCollectionName, "vector", vectorIndex, false)
		if err != nil {
			return nil, fmt.Errorf("failed to create vector index: %w", err)
		}

		// 将集合加载到内存中,这样才能进行查询操作
		// false参数表示非异步加载,会等待加载完成
		err = agentClient.LoadCollection(ctx, common.MilvusCollectionName, false)
		if err != nil {
			return nil, fmt.Errorf("failed to load collection: %w", err)
		}
	} else {
		// 如果集合已存在,确保它已被加载到内存
		// 重复加载不会造成问题,Milvus会忽略已加载的集合
		err = agentClient.LoadCollection(ctx, common.MilvusCollectionName, false)
		if err != nil {
			return nil, fmt.Errorf("failed to load existing collection: %w", err)
		}
	}

	// 清理资源:关闭default数据库的连接
	// 因为我们已经有了agent数据库的连接,不再需要default连接
	defaultClient.Close()

	// 返回已配置好的agent数据库客户端
	return agentClient, nil
}
