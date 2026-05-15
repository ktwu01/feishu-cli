package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/riba2534/feishu-cli/internal/client"
	"github.com/riba2534/feishu-cli/internal/config"
	"github.com/spf13/cobra"
)

var sheetImportMDCmd = &cobra.Command{
	Use:   "import-md <file.md>",
	Short: "从 Markdown 表格创建电子表格",
	Long: `从 Markdown 文件中提取 GFM 表格，并以表格内容创建一个新的飞书电子表格。

工作流程:
  1. 读取本地 .md 文件，提取第 N 张 GFM 表格（默认第 0 张）
  2. POST /sheets/v3/spreadsheets         创建空电子表格
  3. GET  /sheets/v3/spreadsheets/{tok}/sheets/query   拿默认 sheet_id
  4. PUT  /sheets/v2/spreadsheets/{tok}/values         把表格数据写到 A1
  5. 打印新建电子表格的 URL

适用场景:
  把 Markdown 报告/笔记里的数据表格一键变成可在线编辑、筛选、排序的飞书电子表格。

支持的 Markdown 表格语法（GFM）:
  | 列1 | 列2 | 列3 |
  | --- | :---: | ---: |    ← 对齐符号会被忽略，只用作分隔行识别
  | 值A | 值B | 值C |
  | 值D | 值E | 值F |

  也兼容无前导/尾部竖线的写法：
  列1 | 列2
  --- | ---
  值A | 值B

特性:
  - 默认提取文件里第一张 GFM 表格；多表场景用 --table-index 选第几张
  - 单元格内 \| 会被正确识别为字面竖线
  - 不规则行长会按最长行 pad 空字符串

示例:
  # 用文件名作标题，导入第一张表
  feishu-cli sheet import-md report.md

  # 自定义标题 + 指定文件夹
  feishu-cli sheet import-md report.md --title "Q1 销售数据" -f fldcnxxxxxx

  # 文件里有多张表，导第二张（0-based）
  feishu-cli sheet import-md report.md --table-index 1

  # 用 user token 导入到个人空间
  feishu-cli sheet import-md report.md --user-access-token <token>`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.Validate(); err != nil {
			return err
		}

		mdPath := args[0]
		if !strings.HasSuffix(strings.ToLower(mdPath), ".md") && !strings.HasSuffix(strings.ToLower(mdPath), ".markdown") {
			fmt.Fprintf(os.Stderr, "提示: 文件扩展名不是 .md/.markdown，仍按 Markdown 解析\n")
		}

		raw, err := os.ReadFile(mdPath)
		if err != nil {
			return fmt.Errorf("读取文件失败: %w", err)
		}

		tableIndex, _ := cmd.Flags().GetInt("table-index")
		title, _ := cmd.Flags().GetString("title")
		folderToken, _ := cmd.Flags().GetString("folder-token")
		output, _ := cmd.Flags().GetString("output")

		if title == "" {
			base := filepath.Base(mdPath)
			ext := filepath.Ext(base)
			title = strings.TrimSuffix(base, ext)
			if title == "" {
				title = "Markdown 导入"
			}
		}

		// 1. 解析 markdown，挑出第 N 张 GFM 表格
		tables := extractGFMTables(string(raw))
		if len(tables) == 0 {
			return fmt.Errorf("文件里找不到任何 GFM 表格: %s", mdPath)
		}
		if tableIndex < 0 || tableIndex >= len(tables) {
			return fmt.Errorf("--table-index %d 超出范围（文件里共 %d 张表，0-based）", tableIndex, len(tables))
		}
		rows := tables[tableIndex]
		if len(rows) == 0 {
			return fmt.Errorf("第 %d 张表是空的", tableIndex)
		}

		// 客户端预检：飞书 sheets v2 PUT /values 单次最多 5000 行 × 100 列
		const (
			maxRows = 5000
			maxCols = 100
		)
		if len(rows) > maxRows {
			return fmt.Errorf("表格行数 %d 超过飞书 API 单次写入上限 %d 行，请拆分", len(rows), maxRows)
		}
		if len(rows[0]) > maxCols {
			return fmt.Errorf("表格列数 %d 超过飞书 API 单次写入上限 %d 列，请拆分", len(rows[0]), maxCols)
		}

		fmt.Printf("解析到表格 #%d：%d 行 × %d 列\n", tableIndex, len(rows), len(rows[0]))

		userAccessToken := resolveOptionalUserTokenWithFallback(cmd)
		ctx := client.Context()

		// 2. 创建空电子表格
		fmt.Printf("正在创建电子表格 %q ...\n", title)
		info, err := client.CreateSpreadsheet(ctx, title, folderToken, userAccessToken)
		if err != nil {
			return err
		}

		// 3. 拿默认 sheet_id
		sheets, err := client.QuerySheets(ctx, info.SpreadsheetToken, userAccessToken)
		if err != nil {
			return fmt.Errorf("查询工作表列表失败: %w", err)
		}
		if len(sheets) == 0 {
			return fmt.Errorf("新建电子表格没有默认工作表（不应该发生）")
		}
		sheetID := sheets[0].SheetID

		// 4. 写入数据到 A1
		colCount := len(rows[0])
		rowCount := len(rows)
		rangeStr := fmt.Sprintf("%s!A1:%s%d", sheetID, colIndexToLetter(colCount), rowCount)

		values := make([][]any, rowCount)
		for i, row := range rows {
			values[i] = make([]any, colCount)
			for j := 0; j < colCount; j++ {
				if j < len(row) {
					values[i][j] = row[j]
				} else {
					values[i][j] = ""
				}
			}
		}

		fmt.Printf("正在写入 %s ...\n", rangeStr)
		if _, err := client.WriteCells(ctx, info.SpreadsheetToken, rangeStr, values, userAccessToken); err != nil {
			return err
		}

		// 5. 输出
		if output == "json" {
			return printJSON(sheetImportMDResult{
				SpreadsheetToken: info.SpreadsheetToken,
				Title:            info.Title,
				URL:              info.URL,
				Rows:             rowCount,
				Cols:             colCount,
				TableIndex:       tableIndex,
				SourceFile:       mdPath,
			})
		}

		fmt.Println()
		fmt.Println("=== 导入完成 ===")
		fmt.Printf("  Token: %s\n", info.SpreadsheetToken)
		fmt.Printf("  标题: %s\n", info.Title)
		fmt.Printf("  URL: %s\n", info.URL)
		fmt.Printf("  数据: %d 行 × %d 列（来自 %s 第 %d 张表）\n", rowCount, colCount, mdPath, tableIndex)
		return nil
	},
}

// sheetImportMDResult 是 --output json 模式下的稳定输出 schema。
type sheetImportMDResult struct {
	SpreadsheetToken string `json:"spreadsheet_token"`
	Title            string `json:"title"`
	URL              string `json:"url"`
	Rows             int    `json:"rows"`
	Cols             int    `json:"cols"`
	TableIndex       int    `json:"table_index"`
	SourceFile       string `json:"source_file"`
}

// extractGFMTables 从 Markdown 文本中解析所有 GFM 表格，按出现顺序返回。
//
// GFM 表格的判定规则：
//   - 表头行（含至少一个 "|"）+ 紧跟一行分隔线（每个 cell 是 :?-+:?，可有对齐冒号）
//   - 之后连续的「也含 | 的行」是数据行，遇到非表格行停止
//
// 兼容两种写法：
//   - 标准 |...| 双边竖线
//   - 无前导/尾部竖线（GitHub 也支持）
func extractGFMTables(text string) [][][]string {
	lines := strings.Split(text, "\n")
	var tables [][][]string

	i := 0
	for i < len(lines) {
		// 候选表头：至少 1 个 "|" 且不是分隔线本身
		if !looksLikeTableLine(lines[i]) || isSeparatorLine(lines[i]) {
			i++
			continue
		}
		// 下一行必须是分隔线
		if i+1 >= len(lines) || !isSeparatorLine(lines[i+1]) {
			i++
			continue
		}

		// 找到一张表的开头：解析表头 + 分隔线 + 数据行
		headerCells := splitTableRow(lines[i])
		// 分隔线决定列数（GFM 严格按分隔线列数）
		sepCells := splitTableRow(lines[i+1])
		colCount := len(sepCells)
		if colCount == 0 {
			i++
			continue
		}

		var rows [][]string
		rows = append(rows, padRow(headerCells, colCount))

		j := i + 2
		for j < len(lines) && looksLikeTableLine(lines[j]) && !isSeparatorLine(lines[j]) {
			rows = append(rows, padRow(splitTableRow(lines[j]), colCount))
			j++
		}

		tables = append(tables, rows)
		i = j
	}

	return tables
}

// looksLikeTableLine 行里至少一个 "|"（已 unescape \|）且非空。
func looksLikeTableLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	// 至少有一个非转义的 |
	for k := 0; k < len(trimmed); k++ {
		if trimmed[k] == '|' && (k == 0 || trimmed[k-1] != '\\') {
			return true
		}
	}
	return false
}

// isSeparatorLine GFM 分隔线：每个 cell 是 :?-+:?（允许对齐冒号），
// 至少一个 "-"，cell 之间用 | 分隔。
func isSeparatorLine(line string) bool {
	cells := splitTableRow(line)
	if len(cells) == 0 {
		return false
	}
	for _, c := range cells {
		c = strings.TrimSpace(c)
		if !isSeparatorCell(c) {
			return false
		}
	}
	return true
}

func isSeparatorCell(c string) bool {
	if c == "" {
		return false
	}
	// 形如 ---  :---  ---:  :---: 都允许
	start := 0
	end := len(c)
	if c[start] == ':' {
		start++
	}
	if end > start && c[end-1] == ':' {
		end--
	}
	if end-start < 1 {
		return false
	}
	for k := start; k < end; k++ {
		if c[k] != '-' {
			return false
		}
	}
	return true
}

// splitTableRow 把一行 GFM 表格按 "|" 切成 cell 数组，处理：
//   - 前导/尾部竖线可有可无
//   - 单元格内 \| 是字面竖线
//   - 单元格内容首尾空白会被 trim
func splitTableRow(line string) []string {
	trimmed := strings.TrimSpace(line)
	// 去前导/尾部 |（尾部要排除 \|，那是字面竖线不是分隔符）
	trimmed = strings.TrimPrefix(trimmed, "|")
	if strings.HasSuffix(trimmed, "|") && !strings.HasSuffix(trimmed, "\\|") {
		trimmed = trimmed[:len(trimmed)-1]
	}

	var cells []string
	var buf strings.Builder
	for k := 0; k < len(trimmed); k++ {
		ch := trimmed[k]
		if ch == '\\' && k+1 < len(trimmed) && trimmed[k+1] == '|' {
			buf.WriteByte('|')
			k++
			continue
		}
		if ch == '|' {
			cells = append(cells, strings.TrimSpace(buf.String()))
			buf.Reset()
			continue
		}
		buf.WriteByte(ch)
	}
	cells = append(cells, strings.TrimSpace(buf.String()))
	return cells
}

// padRow 把行扩展到 colCount 长（短的补 ""，长的截断）。
func padRow(row []string, colCount int) []string {
	out := make([]string, colCount)
	for i := 0; i < colCount; i++ {
		if i < len(row) {
			out[i] = row[i]
		}
	}
	return out
}

func init() {
	sheetCmd.AddCommand(sheetImportMDCmd)
	sheetImportMDCmd.Flags().StringP("title", "t", "", "电子表格标题（默认用文件名去后缀）")
	sheetImportMDCmd.Flags().StringP("folder-token", "f", "", "目标文件夹 Token（可选）")
	sheetImportMDCmd.Flags().Int("table-index", 0, "选第几张 GFM 表格（0-based，默认第 0 张）")
	sheetImportMDCmd.Flags().StringP("output", "o", "text", "输出格式: text, json")
	sheetImportMDCmd.Flags().String("user-access-token", "", "User Access Token（可选；默认优先使用 auth login 登录态，失败时回退 App Token）")
}
