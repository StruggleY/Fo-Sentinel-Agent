package document

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/dslipak/pdf"
	pdfcpuapi "github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

// ParseFile 根据文件扩展名选择对应解析器，返回清理后的纯文本。
// 支持：.pdf、.docx、.md、.markdown
func ParseFile(filePath string) (string, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".pdf":
		return parsePDF(filePath)
	case ".docx":
		return parseDocx(filePath)
	case ".md", ".markdown":
		return parseText(filePath)
	default:
		return "", fmt.Errorf("unsupported file format: %s", ext)
	}
}

// ParseFileStructured 根据文件扩展名选择对应解析器，返回结构化解析结果（含标题层级）。
// 支持：.pdf、.docx、.md、.markdown
// 对于不支持标题识别的格式（PDF），退化为单节结果。
func ParseFileStructured(filePath string) (*ParsedDocument, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".md", ".markdown":
		return parseMarkdownStructured(filePath)
	case ".docx":
		return parseDocxStructured(filePath)
	case ".pdf":
		return parsePDFStructured(filePath)
	default:
		return nil, fmt.Errorf("unsupported file format: %s", ext)
	}
}

// parseMarkdownStructured 解析 Markdown 文件，按 #/##/### 标题层级切分节。
func parseMarkdownStructured(filePath string) (*ParsedDocument, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	text := cleanupText(string(data))
	return extractMarkdownSections(text, filepath.Base(filePath)), nil
}

// parseTxtStructured 已移除（不再支持 .txt 格式）

// parseDocxStructured 解析 DOCX 文件，识别 Heading1/2/3 段落样式划分章节。
func parseDocxStructured(filePath string) (*ParsedDocument, error) {
	text, err := parseDocx(filePath)
	if err != nil {
		return nil, err
	}
	// DOCX 当前实现提取纯文本，无法区分标题段落样式，退化为 Markdown 解析
	// （如果文档中有 Markdown 风格的标题 #/## 也能识别）
	return extractMarkdownSections(text, strings.TrimSuffix(filepath.Base(filePath), ".docx")), nil
}

// parsePDFStructured 解析 PDF，尝试按空行双换行识别段落，整体作为单节。
// PDF 结构化解析（标题识别）需专业库，当前退化为纯文本单节。
func parsePDFStructured(filePath string) (*ParsedDocument, error) {
	text, err := parsePDF(filePath)
	if err != nil {
		return nil, err
	}
	docTitle := strings.TrimSuffix(filepath.Base(filePath), ".pdf")
	// 尝试按 Markdown 标题识别（有些 PDF 导出工具会保留 # 标题）
	doc := extractMarkdownSections(text, docTitle)
	return doc, nil
}

// reMarkdownHeading 匹配 Markdown 标题行（# 到 ######）。
var reMarkdownHeading = regexp.MustCompile(`^(#{1,6})\s+(.+?)\s*$`)

// extractMarkdownSections 从文本中提取 Markdown 标题层级，切分为 ParsedDocument。
// 标题行本身会写入所在节的 content 开头，确保分块后标题文本可被向量检索命中。
// SectionTitle 按层级路径拼接（如 "安全漏洞 > SQL注入 > 防御措施"），提供完整上下文。
func extractMarkdownSections(text, defaultTitle string) *ParsedDocument {
	// 统一换行符，消除 Windows \r\n 导致标题捕获到 \r 的问题
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	lines := strings.Split(text, "\n")
	doc := &ParsedDocument{}
	var currentSection *ParsedSection
	var contentBuilder strings.Builder

	// 层级路径：index 0=H1, 1=H2, ..., 5=H6
	titlePath := make([]string, 6)

	flushSection := func() {
		if currentSection != nil {
			currentSection.Content = strings.TrimSpace(contentBuilder.String())
			doc.Sections = append(doc.Sections, *currentSection)
			contentBuilder.Reset()
		}
	}

	for _, line := range lines {
		if m := reMarkdownHeading.FindStringSubmatch(line); m != nil {
			level := len(m[1])
			title := strings.TrimSpace(m[2])
			flushSection()

			if doc.Title == "" && level == 1 {
				doc.Title = title
			}

			// 更新层级路径：当前层级设为 title，清空所有子层级
			titlePath[level-1] = title
			for i := level; i < 6; i++ {
				titlePath[i] = ""
			}

			// 构建 "H1 > H2 > H3" 层级路径
			var pathParts []string
			for i := 0; i < level; i++ {
				if titlePath[i] != "" {
					pathParts = append(pathParts, titlePath[i])
				}
			}
			fullTitle := strings.Join(pathParts, " > ")

			currentSection = &ParsedSection{Level: level, Title: fullTitle}
			// 将标题行写入 content，确保分块后标题文本可被检索命中
			contentBuilder.WriteString(line)
			contentBuilder.WriteString("\n")
		} else {
			if currentSection == nil {
				currentSection = &ParsedSection{Level: 0, Title: ""}
			}
			contentBuilder.WriteString(line)
			contentBuilder.WriteString("\n")
		}
	}
	flushSection()

	if doc.Title == "" {
		doc.Title = defaultTitle
	}
	// 过滤掉内容为空且无标题的节
	var sections []ParsedSection
	for _, s := range doc.Sections {
		if s.Content != "" || s.Title != "" {
			sections = append(sections, s)
		}
	}
	if len(sections) == 0 {
		sections = []ParsedSection{{Level: 0, Title: "", Content: text}}
	}
	doc.Sections = sections
	return doc
}

// parsePDF 提取 PDF 文件的纯文本内容。
// 解析流程（三阶段 fallback）：
//  1. 直接用 ledongthuc/pdf 打开并提取（快速路径，覆盖大多数标准 PDF）
//  2. 若失败，用 pdfcpu Optimize 修复格式后重试（处理损坏/非标准 PDF）
//  3. 若仍失败，用 pdfcpu Decrypt 去加密后重试（处理权限加密 PDF）
//  4. 三者均无文字时返回明确错误（如纯图片 PDF）
func parsePDF(filePath string) (string, error) {
	// 阶段1：直接提取
	if text, err := parsePDFWithLib(filePath); err == nil {
		return text, nil
	} else {
		log.Printf("[parsePDF] stage1 failed (%s): %v", filepath.Base(filePath), err)
	}

	// 阶段2：pdfcpu Optimize 修复格式后重试
	if normalized, err := optimizePDFWithPdfcpu(filePath); err == nil {
		defer os.Remove(normalized)
		if text, err := parsePDFWithLib(normalized); err == nil {
			return text, nil
		} else {
			log.Printf("[parsePDF] stage2 failed (%s): %v", filepath.Base(filePath), err)
		}
	} else {
		log.Printf("[parsePDF] stage2 optimize failed (%s): %v", filepath.Base(filePath), err)
	}

	// 阶段3：pdfcpu Decrypt 去加密后重试
	decrypted, decErr := decryptPDFWithPdfcpu(filePath)
	if decErr != nil {
		log.Printf("[parsePDF] stage3 decrypt failed (%s): %v", filepath.Base(filePath), decErr)
	} else {
		defer os.Remove(decrypted)
		if text, err := parsePDFWithLib(decrypted); err == nil {
			return text, nil
		} else {
			log.Printf("[parsePDF] stage3 failed (%s): %v", filepath.Base(filePath), err)
		}

		// 阶段4：Decrypt 后强制转为传统 xref table 格式（ledongthuc/pdf 不支持 xref stream）
		if reopt, err := normalizePDFToLegacy(decrypted); err == nil {
			defer os.Remove(reopt)
			if text, err := parsePDFWithLib(reopt); err == nil {
				return text, nil
			} else {
				log.Printf("[parsePDF] stage4 failed (%s): %v", filepath.Base(filePath), err)
			}
		} else {
			log.Printf("[parsePDF] stage4 normalize failed (%s): %v", filepath.Base(filePath), err)
		}
	}

	return "", fmt.Errorf("parse pdf %s: unable to extract text (possibly image-only or severely malformed PDF)", filepath.Base(filePath))
}

// optimizePDFWithPdfcpu 用 pdfcpu Optimize 修复 PDF 格式问题（压缩、修复交叉引用表等）。
// 适用于格式损坏、非标准结构的 PDF。调用方负责删除临时文件。
func optimizePDFWithPdfcpu(src string) (string, error) {
	tmp, err := os.CreateTemp("", "fo-pdf-opt-*.pdf")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	tmp.Close()

	conf := model.NewDefaultConfiguration()
	conf.UserPW = ""
	conf.OwnerPW = ""
	if err := pdfcpuapi.OptimizeFile(src, tmp.Name(), conf); err != nil {
		os.Remove(tmp.Name())
		return "", fmt.Errorf("pdfcpu optimize: %w", err)
	}
	return tmp.Name(), nil
}

// normalizePDFToLegacy 将 PDF 转换为传统 xref table 格式（禁用 xref stream 和 object stream）。
// ledongthuc/pdf 不支持 PDF 1.5+ 的 xref stream，pdfcpu Decrypt 输出的文件可能使用此格式，
// 需要通过此函数转换后才能被 ledongthuc/pdf 正确读取。调用方负责删除临时文件。
func normalizePDFToLegacy(src string) (string, error) {
	tmp, err := os.CreateTemp("", "fo-pdf-legacy-*.pdf")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	tmp.Close()

	conf := model.NewDefaultConfiguration()
	conf.UserPW = ""
	conf.OwnerPW = ""
	conf.WriteXRefStream = false   // 禁用 xref stream，输出传统 xref table
	conf.WriteObjectStream = false // 禁用 object stream
	if err := pdfcpuapi.OptimizeFile(src, tmp.Name(), conf); err != nil {
		os.Remove(tmp.Name())
		return "", fmt.Errorf("pdfcpu normalize legacy: %w", err)
	}
	return tmp.Name(), nil
}

// 适用于含 256-bit AES 权限加密但无内容密码的 PDF。调用方负责删除临时文件。
func decryptPDFWithPdfcpu(src string) (string, error) {
	f, err := os.Open(src)
	if err != nil {
		return "", fmt.Errorf("open src: %w", err)
	}
	defer f.Close()

	conf := model.NewDefaultConfiguration()
	conf.UserPW = ""
	conf.OwnerPW = ""
	conf.WriteXRefStream = false
	conf.WriteObjectStream = false

	ctx, err := pdfcpuapi.ReadAndValidate(f, conf)
	if err != nil {
		return "", fmt.Errorf("pdfcpu read: %w", err)
	}

	// 清除加密标记，写出时不重新加密
	ctx.Encrypt = nil
	ctx.EncKey = nil

	tmp, err := os.CreateTemp("", "fo-pdf-plain-*.pdf")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	if err := pdfcpuapi.Write(ctx, tmp, conf); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", fmt.Errorf("pdfcpu write plain: %w", err)
	}
	tmp.Close()
	return tmp.Name(), nil
}

// parsePDFWithLib 用 dslipak/pdf 提取文本：
// 优先 GetPlainText()；若失败则逐页读取（兼容部分非标格式）。
// 整体加 60s 超时，防止图片型 PDF 导致 goroutine 卡死。
func parsePDFWithLib(filePath string) (text string, err error) {
	type result struct {
		text string
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		t, e := parsePDFWithLibInner(filePath)
		ch <- result{t, e}
	}()
	select {
	case res := <-ch:
		return res.text, res.err
	case <-time.After(60 * time.Second):
		return "", fmt.Errorf("pdf text extraction timeout (possibly image-only PDF)")
	}
}

func parsePDFWithLibInner(filePath string) (text string, err error) {
	r, openErr := pdf.Open(filePath)
	if openErr != nil {
		return "", fmt.Errorf("open pdf: %w", openErr)
	}

	// 快速路径：一次性提取全文
	if reader, e := r.GetPlainText(); e == nil {
		if buf, e2 := io.ReadAll(reader); e2 == nil {
			if t := cleanupText(string(buf)); t != "" {
				return t, nil
			}
		}
	}

	// 慢速路径：逐页提取，用 recover 防止内部 panic
	text, err = parsePDFPageByPage(r)
	if err != nil {
		return "", err
	}
	return text, nil
}

// parsePDFPageByPage 逐页提取 PDF 文本，recover 捕获 ledongthuc/pdf 的内部 panic。
// 该库在处理大型/复杂/非标准 PDF 时可能在 p.Content() 内部 panic，
// recover 后将 panic 转为 error，由上层 parsePDF 触发 pdfcpu 规范化重试。
func parsePDFPageByPage(r *pdf.Reader) (result string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("pdf page parse panic: %v", r)
		}
	}()

	var sb strings.Builder
	numPages := r.NumPage()
	for i := 1; i <= numPages; i++ {
		p := r.Page(i)
		if p.V.IsNull() {
			continue
		}
		for _, t := range p.Content().Text {
			sb.WriteString(t.S)
		}
		sb.WriteByte('\n')
	}
	result = cleanupText(sb.String())
	if result == "" {
		return "", fmt.Errorf("no text extracted (possibly image-only PDF)")
	}
	return result, nil
}

// parseText 读取 TXT / Markdown 文件。
func parseText(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}
	return cleanupText(string(data)), nil
}

// parseDocx 从 .docx（ZIP+XML）中提取 <w:t> 正文文本，段落（<w:p>）间插入换行。
func parseDocx(filePath string) (string, error) {
	zr, err := zip.OpenReader(filePath)
	if err != nil {
		return "", fmt.Errorf("open docx: %w", err)
	}
	defer zr.Close()

	var docXML io.ReadCloser
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			docXML, err = f.Open()
			if err != nil {
				return "", fmt.Errorf("open word/document.xml: %w", err)
			}
			break
		}
	}
	if docXML == nil {
		return "", fmt.Errorf("word/document.xml not found in docx")
	}
	defer docXML.Close()

	text, err := extractDocxText(docXML)
	if err != nil {
		return "", err
	}
	return cleanupText(text), nil
}

// extractDocxText 逐 token 解析 word/document.xml，仅提取 <w:t> 元素文本。
// 段落 <w:p> 结束时插入换行，忽略批注、修订、嵌入对象等复杂结构。
func extractDocxText(r io.Reader) (string, error) {
	decoder := xml.NewDecoder(r)
	var sb strings.Builder
	inPara := false

	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("decode docx xml: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "p": // <w:p> 段落开始
				inPara = true
			case "t": // <w:t> 文本节点
				if inPara {
					var text string
					if err := decoder.DecodeElement(&text, &t); err == nil {
						sb.WriteString(text)
					}
				}
			}
		case xml.EndElement:
			if t.Name.Local == "p" && inPara {
				sb.WriteString("\n")
				inPara = false
			}
		}
	}
	return sb.String(), nil
}

var (
	reTrailingSpace = regexp.MustCompile(`[ \t]+\n`)
	reExcessNewline = regexp.MustCompile(`\n{3,}`)
)

// cleanupText：
// 1. 移除 UTF-8 BOM
// 2. 清除行尾空格/制表符
// 3. 压缩连续 3+ 空行为双空行
// 4. 去除首尾空白
func cleanupText(text string) string {
	text = strings.TrimPrefix(text, "\uFEFF")
	text = reTrailingSpace.ReplaceAllString(text, "\n")
	text = reExcessNewline.ReplaceAllString(text, "\n\n")
	return strings.TrimSpace(text)
}

// TruncateToMaxBytes 在 UTF-8 字符边界处截断字符串，确保不超过 maxBytes 字节。
// 用于 Milvus varchar(8192) 安全边界保护。
func TruncateToMaxBytes(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	for i := maxBytes; i > 0; i-- {
		if (s[i] & 0xC0) != 0x80 {
			return s[:i]
		}
	}
	return ""
}
