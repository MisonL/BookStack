package mcp

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/TruthHun/BookStack/models"
	"github.com/astaxie/beego/orm"
	"github.com/mark3labs/mcp-go/mcp"
)

// Pagination constants
const (
	DefaultListBooksPageSize  = 50
	DefaultSearchDocsPageSize = 30
	MaxDescriptionSnippetLen  = 100
)

// stripHTML removes HTML tags from a string
func stripHTML(s string) string {
	re := regexp.MustCompile(`<[^>]*>`)
	return re.ReplaceAllString(s, "")
}

// HandleListBooks handles the list_books tool
func HandleListBooks(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, ok := request.Params.Arguments.(map[string]interface{})
	// 实例化 Book Model
	if !ok {
		return mcp.NewToolResultError("Invalid arguments format"), nil
	}

	// 1. List Books
	token, _ := args["member_token"].(string)
	memberID, _ := ValidateToken(token)

	bookModel := models.NewBook()

	// 使用 FindForHomeToPager 获取带权限的书籍列表
	// 该方法逻辑：如果会员ID>0，返回 (私有且有权限 OR 公开) 的书籍；否则只返回公开书籍
	// orderType 空字符串默认 sort by order_index
	books, _, err := bookModel.FindForHomeToPager(1, DefaultListBooksPageSize, memberID, "")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list books: %v", err)), nil
	}

	var sb strings.Builder
	sb.WriteString("# Book List\n\n")
	for _, b := range books {
		// b 是 BookResult *
		vis := "Public"
		if b.PrivatelyOwned == 1 {
			vis = "Private"
		}
		sb.WriteString(fmt.Sprintf("- [ID: %d] **%s** (Identify: %s) - %s\n", b.BookId, b.BookName, b.Identify, vis))
		if b.Description != "" {
			desc := stripHTML(b.Description)
			if len(desc) > MaxDescriptionSnippetLen {
				desc = desc[:MaxDescriptionSnippetLen] + "..."
			}
			sb.WriteString(fmt.Sprintf("  %s\n", desc))
		}
		sb.WriteString("\n")
	}

	return mcp.NewToolResultText(sb.String()), nil
}

// HandleGetDoc handles the get_doc tool
func HandleGetDoc(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, ok := request.Params.Arguments.(map[string]interface{})
	if !ok {
		return mcp.NewToolResultError("Invalid arguments format"), nil
	}

	// 2. Get Doc
	docID, hasDocID := args["doc_id"].(float64)
	identify, _ := args["identify"].(string)
	bookID, hasBookID := args["book_id"].(float64)
	format, _ := args["format"].(string)
	token, _ := args["member_token"].(string)
	memberID, _ := ValidateToken(token)

	if format == "" {
		format = "markdown"
	}

	var doc *models.Document
	var err error
	docModel := models.NewDocument()

	// 1. 获取 Document 基础信息
	if hasDocID {
		doc, err = docModel.Find(int(docID))
	} else if identify != "" && hasBookID {
		doc, err = docModel.FindByBookIdAndDocIdentify(int(bookID), identify)
	} else {
		return mcp.NewToolResultError("Missing required arguments: doc_id OR (identify AND book_id)"), nil
	}

	if err != nil {
		if err == orm.ErrNoRows {
			return mcp.NewToolResultError("Document not found"), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("Error finding document: %v", err)), nil
	}

	// Permission Check
	bookModel := models.NewBook()
	book, err := bookModel.Find(doc.BookId)
	if err != nil {
		return mcp.NewToolResultError("Associated book not found"), nil
	}

	if book.PrivatelyOwned == 1 {
		// Private book: check access
		if memberID <= 0 {
			return mcp.NewToolResultError("Access denied: Private book requires valid member_token"), nil
		}
		// MinRole? Assuming Observer (3) is enough to read.
		// conf.BookObserver = 3? Need to check enumerate.go or just pass max int?
		// HasProjectAccess(identify string, memberId int, minRole int)
		// We have book.Identify
		// However, minRole 0 is Founder. We need MAX value for "Any Access".
		// Actually HasProjectAccess expects `role_id <= minRole`.
		// So passing a large number (like 3) should cover all roles.
		// Let's assume 100 or check conf.
		if !bookModel.HasProjectAccess(book.Identify, memberID, 100) {
			return mcp.NewToolResultError("Access denied: Insufficient permissions for this private book"), nil
		}
	}

	// 2. 获取内容
	var content string
	dsModel := models.NewDocumentStore()

	// 优先从 DocumentStore 获取 Markdown（这也是最适合 LLM 的格式）
	// models.DocumentStore.GetFiledById(docId interface{}, field string) string
	// 注意：GetFiledById 内部如果是 markdown 字段，如果为空可能会 fallback？看源码是直接返回字段。

	if format == "markdown" {
		// 尝试获取 Markdown
		md := dsModel.GetFiledById(doc.DocumentId, "markdown")
		if md != "" {
			content = md
		} else {
			// 如果 Markdown 为空（可能是旧数据或导入数据），降级使用 Release (HTML) 转换
			// 简单转义或直接提供 HTML (提示用户)
			content = fmt.Sprintf("Markdown source not available. HTML Release content:\n%s", doc.Release)
		}
	} else if format == "html" {
		content = doc.Release
	} else if format == "text" {
		// 简单的文本内容, 优先用 Content (纯文本?)，models里 content 是 "文本内容"
		txt := dsModel.GetFiledById(doc.DocumentId, "content")
		if txt != "" {
			content = txt
		} else {
			content = doc.Release // 降级
		}
	} else {
		return mcp.NewToolResultError("Unsupported format: " + format), nil
	}

	// 构造结果
	info := fmt.Sprintf("Title: %s\nID: %d\nBook ID: %d\nFormat: %s\n\n", doc.DocumentName, doc.DocumentId, doc.BookId, format)
	return mcp.NewToolResultText(info + content), nil
}

// HandleSearchDocs handles the search_docs tool
func HandleSearchDocs(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, ok := request.Params.Arguments.(map[string]interface{})
	if !ok {
		return mcp.NewToolResultError("Invalid arguments format"), nil
	}

	// 3. Search Docs
	query, _ := args["query"].(string)
	// bookID, _ := args["book_id"].(float64) // Optional
	token, _ := args["member_token"].(string)
	memberID, _ := ValidateToken(token)

	if query == "" {
		return mcp.NewToolResultError("Query is required"), nil
	}

	searchModel := models.NewDocumentSearchResult()

	// 使用 FindToPager 进行全局搜索，支持 memberId 过滤
	// FindToPager(keyword string, pageIndex, pageSize, memberId int)
	docs, totalCount, err := searchModel.FindToPager(query, 1, DefaultSearchDocsPageSize, memberID)

	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Search failed: %v", err)), nil
	}

	var resultText strings.Builder
	resultText.WriteString(fmt.Sprintf("Found %d documents (via Database):\n\n", totalCount))

	for _, doc := range docs {
		resultText.WriteString(fmt.Sprintf("- [ID: %d] **%s** (Book: %s)\n", doc.DocumentId, doc.DocumentName, doc.BookName))
		// MySQL 搜索不返回 snippet，需要额外查询?
		// SearchDocument 返回的 DocumentSearchResult 包含 Identify 和 Description 吗？
		// 看 models/document_search_result.go:
		// doc.Description 是空的 (doc.release as description)
		// 实际上 SearchDocument SQL: SELECT ... release as description ...
		// 所以我们有 HTML 片段 (Release)

		// 清理 HTML 标签作为 snippet
		snippet := stripHTML(doc.Description)
		if len(snippet) > MaxDescriptionSnippetLen {
			snippet = snippet[:MaxDescriptionSnippetLen] + "..."
		}
		resultText.WriteString(fmt.Sprintf("  %s\n\n", snippet))
	}

	return mcp.NewToolResultText(resultText.String()), nil
}

/*
func HandleResourceBooks(ctx context.Context, request mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	// 简单的 Resource 实现
	bookModel := models.NewBook()
	o := orm.NewOrm()
	var books []*models.Book
	// 只查 20 本
	o.QueryTable(bookModel.TableNameWithPrefix()).Filter("privately_owned", 0).Limit(20).All(&books)

	var sb strings.Builder
	sb.WriteString("# Book List\n\n")
	for _, b := range books {
		sb.WriteString(fmt.Sprintf("- [%d] %s (%s)\n", b.BookId, b.BookName, b.Identify))
	}

	return &mcp.ReadResourceResult{
		Contents: []mcp.ResourceContent{
			{
				URI:      request.Params.URI,
				MIMEType: "text/markdown",
				Text:     sb.String(),
			},
		},
	}, nil
}
*/
