package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/riba2534/feishu-cli/internal/client"
	"github.com/spf13/cobra"
)

var sheetReadCmd = &cobra.Command{
	Use:   "read <spreadsheet_token> <range>",
	Short: "读取单元格数据",
	Long: `读取电子表格中指定范围的单元格数据。

范围格式:
  SheetID!A1:B2    - 指定工作表的范围
  A1:B2            - 当前工作表的范围（需要配合 --sheet-id）
  Sheet1!A:C       - 整列
  Sheet1!1:3       - 整行

示例:
  feishu-cli sheet read shtcnxxxxxx "Sheet1!A1:C10"
  feishu-cli sheet read shtcnxxxxxx "A1:C10" --sheet-id 0b12`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		spreadsheetToken := args[0]
		rangeStr := args[1]
		sheetID, _ := cmd.Flags().GetString("sheet-id")
		valueRenderOption, _ := cmd.Flags().GetString("value-render")
		dateTimeRenderOption, _ := cmd.Flags().GetString("datetime-render")
		outputFormat, _ := cmd.Flags().GetString("output")

		// 处理 shell 转义
		rangeStr = unescapeSheetRange(rangeStr)

		// 如果指定了 sheet-id 且范围中没有 !，则添加 sheet-id
		if sheetID != "" && !strings.Contains(rangeStr, "!") {
			rangeStr = sheetID + "!" + rangeStr
		}

		cellRange, err := client.ReadCells(client.Context(), spreadsheetToken, rangeStr, valueRenderOption, dateTimeRenderOption)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			output, _ := json.MarshalIndent(cellRange, "", "  ")
			fmt.Println(string(output))
		} else {
			fmt.Printf("范围: %s\n", cellRange.Range)
			if len(cellRange.Values) == 0 {
				fmt.Println("（空数据）")
				return nil
			}
			fmt.Println("数据:")
			for i, row := range cellRange.Values {
				rowStrs := make([]string, len(row))
				for j, cell := range row {
					rowStrs[j] = fmt.Sprintf("%v", cell)
				}
				fmt.Printf("  [%d] %s\n", i+1, strings.Join(rowStrs, " | "))
			}
		}

		return nil
	},
}

func init() {
	sheetCmd.AddCommand(sheetReadCmd)

	sheetReadCmd.Flags().String("sheet-id", "", "工作表 ID（如果范围中未指定）")
	sheetReadCmd.Flags().String("value-render", "", "值渲染选项: ToString, FormattedValue, Formula, UnformattedValue")
	sheetReadCmd.Flags().String("datetime-render", "", "日期时间渲染选项: FormattedString")
	sheetReadCmd.Flags().StringP("output", "o", "text", "输出格式: text, json")
}
