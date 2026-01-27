package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/riba2534/feishu-cli/internal/client"
	"github.com/spf13/cobra"
)

var sheetFindCmd = &cobra.Command{
	Use:   "find <spreadsheet_token> <sheet_id> <keyword>",
	Short: "查找单元格内容",
	Long: `在工作表中查找指定内容。

示例:
  # 普通查找
  feishu-cli sheet find shtcnxxxxxx 0b12 "关键词"

  # 正则表达式查找
  feishu-cli sheet find shtcnxxxxxx 0b12 "[A-Z]+" --regex

  # 在指定范围内查找
  feishu-cli sheet find shtcnxxxxxx 0b12 "关键词" --range "A1:C10"`,
	Args: cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		spreadsheetToken := args[0]
		sheetID := args[1]
		keyword := args[2]
		rangeStr, _ := cmd.Flags().GetString("range")
		matchCase, _ := cmd.Flags().GetBool("match-case")
		matchEntireCell, _ := cmd.Flags().GetBool("match-entire-cell")
		searchByRegex, _ := cmd.Flags().GetBool("regex")
		outputFormat, _ := cmd.Flags().GetString("output")

		result, err := client.FindCells(client.Context(), spreadsheetToken, sheetID, keyword, matchCase, matchEntireCell, searchByRegex, rangeStr)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			output, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(output))
		} else {
			fmt.Printf("查找结果:\n")
			fmt.Printf("  匹配单元格: %d 个\n", len(result.MatchedCells))
			fmt.Printf("  匹配行数: %d\n", result.RowsCount)
			if len(result.MatchedCells) > 0 {
				fmt.Printf("  单元格列表: %v\n", result.MatchedCells)
			}
			if len(result.MatchedFormulaCells) > 0 {
				fmt.Printf("  公式单元格: %v\n", result.MatchedFormulaCells)
			}
		}

		return nil
	},
}

func init() {
	sheetCmd.AddCommand(sheetFindCmd)

	sheetFindCmd.Flags().String("range", "", "查找范围（如 A1:C10）")
	sheetFindCmd.Flags().Bool("match-case", false, "区分大小写")
	sheetFindCmd.Flags().Bool("match-entire-cell", false, "完全匹配单元格")
	sheetFindCmd.Flags().Bool("regex", false, "使用正则表达式")
	sheetFindCmd.Flags().StringP("output", "o", "text", "输出格式: text, json")
}
