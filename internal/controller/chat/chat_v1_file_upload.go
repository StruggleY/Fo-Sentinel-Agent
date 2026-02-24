package chat

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"Fo-Sentinel-Agent/api/chat/v1"
	"Fo-Sentinel-Agent/internal/ai/agent/knowledge_index_pipeline"
	loader2 "Fo-Sentinel-Agent/internal/ai/loader"
	"Fo-Sentinel-Agent/utility/client"
	"Fo-Sentinel-Agent/utility/common"
	"Fo-Sentinel-Agent/utility/log_call_back"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/compose"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gfile"
)

// FileUpload 处理文件上传并构建知识库索引
func (c *ControllerV1) FileUpload(ctx context.Context, req *v1.FileUploadReq) (res *v1.FileUploadRes, err error) {
	// 步骤1: 从请求上下文中获取上传的文件对象
	r := g.RequestFromCtx(ctx)
	uploadFile := r.GetUploadFile("file")
	if uploadFile == nil {
		return nil, gerror.New("请上传文件")
	}

	// 步骤2: 检查并创建文件保存目录
	if !gfile.Exists(common.FileDir) {
		if err := gfile.Mkdir(common.FileDir); err != nil {
			return nil, gerror.Wrapf(err, "创建目录失败: %s", common.FileDir)
		}
	}

	// 步骤3: 构造文件保存路径
	newFileName := uploadFile.Filename
	savePath := filepath.Join(common.FileDir)

	// 步骤4: 将上传文件保存到磁盘
	_, err = uploadFile.Save(savePath, false)
	if err != nil {
		return nil, gerror.Wrapf(err, "保存文件失败")
	}

	// 步骤5: 读取文件元信息(大小等)
	fileInfo, err := os.Stat(savePath)
	if err != nil {
		return nil, gerror.Wrapf(err, "获取文件信息失败")
	}

	// 步骤6: 构造响应数据
	res = &v1.FileUploadRes{
		FileName: newFileName,
		FilePath: savePath,
		FileSize: fileInfo.Size(),
	}

	// 步骤7: 将文件内容构建到知识库索引
	err = buildIntoIndex(ctx, common.FileDir+"/"+newFileName)
	if err != nil {
		return nil, gerror.Wrapf(err, "构建知识库失败")
	}
	return res, nil
}

// buildIntoIndex 构建文件索引到向量数据库
func buildIntoIndex(ctx context.Context, path string) error {
	// 步骤1: 构建知识库索引处理流水线
	r, err := knowledge_index_pipeline.BuildKnowledgeIndexing(ctx)

	// 步骤2: 创建文件加载器并加载文档
	loader, err := loader2.NewFileLoader(ctx)
	if err != nil {
		return err
	}
	docs, err := loader.Load(ctx, document.Source{URI: path})
	if err != nil {
		return err
	}

	// 步骤3: 连接Milvus向量数据库
	cli, err := client.NewMilvusClient(ctx)
	if err != nil {
		return err
	}

	// 步骤4: 查询并删除相同源文件的旧索引数据(避免重复索引)
	// 4.1 构造查询表达式：从metadata的JSON字段中查找_source字段匹配的记录
	// 例如：metadata["_source"] == "/data/files/document.pdf"
	// _source记录了文档的原始文件路径，同一个文件可能被多次上传
	expr := fmt.Sprintf(`metadata["_source"] == "%s"`, docs[0].MetaData["_source"])

	// 4.2 执行查询操作
	// 参数说明：
	//   - common.MilvusCollectionName: 集合名称(biz)
	//   - []: 不指定分区(partition)，查询整个集合
	//   - expr: 过滤表达式，只查找_source匹配的记录
	//   - []string{"id"}: 只返回id字段，减少数据传输量
	queryResult, err := cli.Query(ctx, common.MilvusCollectionName, []string{}, expr, []string{"id"})
	if err != nil {
		return err
	} else if len(queryResult) > 0 {
		// 查询结果不为空，说明该文件之前已经被索引过

		// 步骤4.3: 从查询结果中提取所有待删除记录的ID
		// Milvus查询返回的是列式数据结构(Column)，不是行式
		var idsToDelete []string
		for _, column := range queryResult {
			// 遍历所有返回的列，找到名为"id"的列
			if column.Name() == "id" {
				// 遍历该列的所有行，提取每个id值
				for i := 0; i < column.Len(); i++ {
					id, err := column.GetAsString(i)
					if err == nil {
						idsToDelete = append(idsToDelete, id)
					}
				}
			}
		}

		// 步骤4.4: 批量删除旧记录(清理历史数据)
		if len(idsToDelete) > 0 {
			// 构造删除表达式：id in ["uuid1","uuid2","uuid3"]
			// 使用IN操作符批量删除，比逐条删除高效
			deleteExpr := fmt.Sprintf(`id in ["%s"]`, strings.Join(idsToDelete, `","`))
			err = cli.Delete(ctx, common.MilvusCollectionName, "", deleteExpr)
			if err != nil {
				// 删除失败只警告，不中断流程(降级处理)
				fmt.Printf("[warn] delete existing data failed: %v\n", err)
			} else {
				// 删除成功，记录日志
				fmt.Printf("[info] deleted %d existing records with _source: %s\n", len(idsToDelete), docs[0].MetaData["_source"])
			}
		}
	}

	// 步骤5: 执行索引构建流水线，生成向量并存储到Milvus
	// 调用之前构建好的知识索引处理流水线
	// 流程：FileLoader(加载文档) -> MarkdownSplitter(分块) -> MilvusIndexer(向量化+存储)
	// 参数说明：
	//   - document.Source{URI: path}: 输入文档路径
	//   - compose.WithCallbacks: 添加日志回调，记录处理过程
	ids, err := r.Invoke(ctx, document.Source{URI: path}, compose.WithCallbacks(log_call_back.LogCallback(nil)))
	if err != nil {
		return fmt.Errorf("invoke index graph failed: %w", err)
	}

	// 索引构建成功，打印统计信息
	// ids是返回的所有向量记录的ID列表，len(ids)表示文档被切分成了多少个块
	fmt.Printf("[done] indexing file: %s, len of parts: %d\n", path, len(ids))
	return nil
}
