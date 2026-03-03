package chat

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	v1 "Fo-Sentinel-Agent/api/chat/v1"
	"Fo-Sentinel-Agent/internal/ai/agent/knowledge_index_pipeline"
	"Fo-Sentinel-Agent/utility/client"
	"Fo-Sentinel-Agent/utility/common"
	"Fo-Sentinel-Agent/utility/log_call_back"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/compose"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gfile"
)

// maxUploadBytes 单次上传文件大小上限（50 MB）
// 在 HTTP 层拦截超大请求
const maxUploadBytes int64 = 50 << 20 // 50 MB

// FileUpload 处理文件上传并构建知识库索引
func (c *ControllerV1) FileUpload(ctx context.Context, req *v1.FileUploadReq) (res *v1.FileUploadRes, err error) {
	// 步骤1: 从请求上下文中获取上传的文件对象
	r := g.RequestFromCtx(ctx)
	uploadFile := r.GetUploadFile("file")
	if uploadFile == nil {
		return nil, gerror.New("请上传文件")
	}

	// 步骤1.1: 校验文件大小，拒绝超过上限的文件，防止大文件 OOM
	if uploadFile.Size > maxUploadBytes {
		return nil, gerror.Newf("文件过大（%.1f MB），单次上传上限为 50 MB", float64(uploadFile.Size)/(1<<20))
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
	if err != nil {
		return fmt.Errorf("build knowledge indexing failed: %w", err)
	}

	// 步骤2: 获取全局单例 Milvus 客户端（进程内复用，无重复建连开销）
	cli, err := client.GetMilvusClient(ctx)
	if err != nil {
		return err
	}

	// 步骤3: 查询并删除相同源文件的旧索引数据（避免重复索引）
	// _source 即文件路径，eino FileLoader 写入 metadata 时以 URI 作为 _source 值。
	expr := fmt.Sprintf(`metadata["_source"] == "%s"`, path)

	// 3.1 只返回 id 字段，减少网络传输量
	queryResult, err := cli.Query(ctx, common.MilvusCollectionName, []string{}, expr, []string{"id"})
	if err != nil {
		return err
	} else if len(queryResult) > 0 {
		// 3.2 从列式结果中提取所有待删除的 ID
		var idsToDelete []string
		for _, column := range queryResult {
			if column.Name() == "id" {
				for i := 0; i < column.Len(); i++ {
					id, err := column.GetAsString(i)
					if err == nil {
						idsToDelete = append(idsToDelete, id)
					}
				}
			}
		}

		// 3.3 批量删除旧记录，使用 IN 操作符比逐条删除高效
		if len(idsToDelete) > 0 {
			deleteExpr := fmt.Sprintf(`id in ["%s"]`, strings.Join(idsToDelete, `","`))
			err = cli.Delete(ctx, common.MilvusCollectionName, "", deleteExpr)
			if err != nil {
				fmt.Printf("[warn] delete existing data failed: %v\n", err)
			} else {
				fmt.Printf("[info] deleted %d existing records with _source: %s\n", len(idsToDelete), path)
			}
		}
	}

	// 步骤4: 执行索引构建流水线（FileLoader → MarkdownSplitter → MilvusIndexer）
	ids, err := r.Invoke(ctx, document.Source{URI: path}, compose.WithCallbacks(log_call_back.LogCallback(nil)))
	if err != nil {
		return fmt.Errorf("invoke index graph failed: %w", err)
	}

	// 索引构建成功，打印统计信息
	// ids是返回的所有向量记录的ID列表，len(ids)表示文档被切分成了多少个块
	fmt.Printf("[done] indexing file: %s, len of parts: %d\n", path, len(ids))
	return nil
}
