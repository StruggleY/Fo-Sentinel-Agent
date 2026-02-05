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
		// 如果集合不存在,需要创建它
		// 首先定义集合的schema(结构定义),包括字段信息
		schema := &entity.Schema{
			CollectionName: common.MilvusCollectionName,     // 集合名称
			Description:    "Business knowledge collection", // 集合描述:业务知识集合
			Fields:         fields,                          // 字段定义(在文件底部定义)
		}

		// 创建集合,使用默认的分片数量
		err = agentClient.CreateCollection(ctx, schema, entity.DefaultShardNumber)
		if err != nil {
			return nil, fmt.Errorf("failed to create biz collection: %w", err)
		}

		// 只为vector字段创建索引
		// 注意:
		// - id字段是主键,Milvus会自动为其创建索引,无需手动创建
		// - content字段是VarChar类型,不是向量,不需要向量索引
		// - 只有vector字段才需要向量索引,因为它用于相似度搜索
		vectorIndex, err := entity.NewIndexAUTOINDEX(entity.HAMMING)
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

// fields 定义了业务知识集合(biz collection)的字段结构
// 这个schema定义了如何存储和检索业务知识向量数据
var fields = []*entity.Field{
	{
		// id字段: 主键字段,用于唯一标识每条记录
		Name:     "id",
		DataType: entity.FieldTypeVarChar, // 可变长度字符串类型
		TypeParams: map[string]string{
			"max_length": "256", // 最大长度256个字符
		},
		PrimaryKey: true, // 标记为主键
	},
	{
		// vector字段: 存储文档的向量表示(embeddings)
		Name:     "vector",
		DataType: entity.FieldTypeBinaryVector, // 二进制向量类型,节省存储空间
		TypeParams: map[string]string{
			"dim": "65536", // 向量维度为65536维(相当于8192字节的二进制向量)
		},
	},
	{
		// content字段: 存储原始文本内容
		Name:     "content",
		DataType: entity.FieldTypeVarChar, // 可变长度字符串类型
		TypeParams: map[string]string{
			"max_length": "8192", // 最大长度8192个字符
		},
	},
	{
		// metadata字段: 存储额外的元数据信息(如来源、时间戳等)
		Name:     "metadata",
		DataType: entity.FieldTypeJSON, // JSON类型,可以存储任意结构化数据
	},
}
