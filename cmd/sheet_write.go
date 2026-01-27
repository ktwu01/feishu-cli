package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/riba2534/feishu-cli/internal/client"
	"github.com/spf13/cobra"
)

var sheetWriteCmd = &cobra.Command{
	Use:   "write <spreadsheet_token> <range>",
	Short: "写入单元格数据",
	Long: `写入数据到电子表格的指定范围。

数据格式（JSON 二维数组）:
  [["A1值", "B1值"], ["A2值", "B2值"]]

示例:
  # 通过命令行参数传入数据
  feishu-cli sheet write shtcnxxxxxx "Sheet1!A1:B2" --data '[["姓名", "年龄"], ["张三", 25]]'

  # 从文件读取数据
  feishu-cli sheet write shtcnxxxxxx "Sheet1!A1:B2" --data-file data.json`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		spreadsheetToken := args[0]
		rangeStr := args[1]
		sheetID, _ := cmd.Flags().GetString("sheet-id")
		dataStr, _ := cmd.Flags().GetString("data")
		dataFile, _ := cmd.Flags().GetString("data-file")
		outputFormat, _ := cmd.Flags().GetString("output")

		// 处理 shell 转义
		rangeStr = unescapeSheetRange(rangeStr)

		// 如果指定了 sheet-id 且范围中没有 !，则添加 sheet-id
		if sheetID != "" && !strings.Contains(rangeStr, "!") {
			rangeStr = sheetID + "!" + rangeStr
		}

		// 获取数据
		var jsonData string
		if dataFile != "" {
			data, err := os.ReadFile(dataFile)
			if err != nil {
				return fmt.Errorf("读取数据文件失败: %w", err)
			}
			jsonData = string(data)
		} else if dataStr != "" {
			jsonData = dataStr
		} else {
			return fmt.Errorf("请通过 --data 或 --data-file 指定数据")
		}

		// 解析 JSON 数据
		var values [][]interface{}
		if err := json.Unmarshal([]byte(jsonData), &values); err != nil {
			return fmt.Errorf("解析数据失败（需要 JSON 二维数组）: %w", err)
		}

		result, err := client.WriteCells(client.Context(), spreadsheetToken, rangeStr, values)
		if err != nil {
			return err
		}

		if outputFormat == "json" {
			output, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(output))
		} else {
			fmt.Printf("写入成功！\n")
			fmt.Printf("  更新范围: %s\n", result.Range)
			fmt.Printf("  写入行数: %d\n", len(values))
		}

		return nil
	},
}

func init() {
	sheetCmd.AddCommand(sheetWriteCmd)

	sheetWriteCmd.Flags().String("sheet-id", "", "工作表 ID（如果范围中未指定）")
	sheetWriteCmd.Flags().StringP("data", "d", "", "要写入的数据（JSON 二维数组）")
	sheetWriteCmd.Flags().String("data-file", "", "数据文件路径")
	sheetWriteCmd.Flags().StringP("output", "o", "text", "输出格式: text, json")
}
