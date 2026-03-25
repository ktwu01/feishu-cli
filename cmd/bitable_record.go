package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/riba2534/feishu-cli/internal/client"
	"github.com/spf13/cobra"
)

var bitableRecordsCmd = &cobra.Command{
	Use:   "records <app_token> <table_id>",
	Short: "搜索/列出记录",
	Long: `搜索或列出数据表中的记录。

支持过滤、排序和分页。不指定过滤条件时列出所有记录。

过滤条件 JSON 格式:
  {"conjunction":"and","conditions":[{"field_name":"状态","operator":"is","value":["进行中"]}]}

排序 JSON 格式:
  [{"field_name":"创建时间","desc":true}]

运算符: is, isNot, contains, doesNotContain, isEmpty, isNotEmpty, isGreater, isLess`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		appToken := args[0]
		tableID := args[1]
		output, _ := cmd.Flags().GetString("output")
		pageSize, _ := cmd.Flags().GetInt("page-size")
		pageToken, _ := cmd.Flags().GetString("page-token")
		filterJSON, _ := cmd.Flags().GetString("filter")
		sortJSON, _ := cmd.Flags().GetString("sort")
		fieldNamesStr, _ := cmd.Flags().GetString("field-names")
		userToken := resolveOptionalUserToken(cmd)

		opts := client.BitableSearchOptions{
			PageSize:  pageSize,
			PageToken: pageToken,
		}

		if filterJSON != "" {
			var filter client.BitableFilter
			if err := json.Unmarshal([]byte(filterJSON), &filter); err != nil {
				return fmt.Errorf("解析过滤条件 JSON 失败: %w", err)
			}
			opts.Filter = &filter
		}

		if sortJSON != "" {
			var sort []client.BitableSortItem
			if err := json.Unmarshal([]byte(sortJSON), &sort); err != nil {
				return fmt.Errorf("解析排序条件 JSON 失败: %w", err)
			}
			opts.Sort = sort
		}

		if fieldNamesStr != "" {
			opts.FieldNames = splitAndTrim(fieldNamesStr)
		}

		records, nextPageToken, total, err := client.SearchBitableRecords(appToken, tableID, opts, userToken)
		if err != nil {
			return err
		}

		if output == "json" {
			result := map[string]any{
				"total":   total,
				"records": records,
			}
			if nextPageToken != "" {
				result["page_token"] = nextPageToken
				result["has_more"] = true
			}
			return printJSON(result)
		}

		fmt.Printf("共 %d 条记录", total)
		if nextPageToken != "" {
			fmt.Printf("（还有更多，page_token: %s）", nextPageToken)
		}
		fmt.Println()

		for i, r := range records {
			fmt.Printf("\n--- 记录 %d (ID: %s) ---\n", i+1, r.RecordID)
			for k, v := range r.Fields {
				fmt.Printf("  %s: %v\n", k, formatFieldValue(v))
			}
		}
		return nil
	},
}

var bitableGetRecordCmd = &cobra.Command{
	Use:   "get-record <app_token> <table_id> <record_id>",
	Short: "获取单条记录",
	Long:  "获取数据表中的指定记录",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		appToken := args[0]
		tableID := args[1]
		recordID := args[2]
		output, _ := cmd.Flags().GetString("output")
		userToken := resolveOptionalUserToken(cmd)

		record, err := client.GetBitableRecord(appToken, tableID, recordID, userToken)
		if err != nil {
			return err
		}

		if output == "json" {
			return printJSON(record)
		}

		fmt.Printf("Record ID: %s\n", record.RecordID)
		for k, v := range record.Fields {
			fmt.Printf("  %s: %v\n", k, formatFieldValue(v))
		}
		return nil
	},
}

var bitableAddRecordCmd = &cobra.Command{
	Use:   "add-record <app_token> <table_id>",
	Short: "创建记录",
	Long: `创建单条记录。

字段值 JSON 格式:
  {"名称": "测试", "金额": 100, "状态": "进行中"}

注意:
  - 数值不要传字符串
  - 日期使用 13 位毫秒时间戳
  - 单选直接写选项文本（自动创建选项）
  - 多选使用字符串数组 ["A","B"]
  - 超链接使用对象 {"text":"名称","link":"https://..."}`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		appToken := args[0]
		tableID := args[1]
		fieldsJSON, _ := cmd.Flags().GetString("fields")
		fieldsFile, _ := cmd.Flags().GetString("fields-file")
		output, _ := cmd.Flags().GetString("output")
		userToken := resolveOptionalUserToken(cmd)

		fieldsJSON, err := loadJSONInput(fieldsJSON, fieldsFile, "fields", "fields-file", "字段值 JSON")
		if err != nil {
			return err
		}

		var fields map[string]any
		if err := json.Unmarshal([]byte(fieldsJSON), &fields); err != nil {
			return fmt.Errorf("解析字段值 JSON 失败: %w", err)
		}

		record, err := client.CreateBitableRecord(appToken, tableID, fields, userToken)
		if err != nil {
			return err
		}

		if output == "json" {
			return printJSON(record)
		}

		fmt.Printf("创建成功！Record ID: %s\n", record.RecordID)
		return nil
	},
}

var bitableAddRecordsCmd = &cobra.Command{
	Use:   "add-records <app_token> <table_id>",
	Short: "批量创建记录",
	Long: `批量创建记录（最多 500 条）。

数据格式为 JSON 数组，每个元素是字段值对象:
  [{"名称":"A","金额":100},{"名称":"B","金额":200}]`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		appToken := args[0]
		tableID := args[1]
		dataJSON, _ := cmd.Flags().GetString("data")
		dataFile, _ := cmd.Flags().GetString("data-file")
		output, _ := cmd.Flags().GetString("output")
		userToken := resolveOptionalUserToken(cmd)

		dataJSON, err := loadJSONInput(dataJSON, dataFile, "data", "data-file", "记录数据 JSON")
		if err != nil {
			return err
		}

		var records []map[string]any
		if err := json.Unmarshal([]byte(dataJSON), &records); err != nil {
			return fmt.Errorf("解析数据 JSON 失败: %w", err)
		}

		results, err := client.BatchCreateBitableRecords(appToken, tableID, records, userToken)
		if err != nil {
			return err
		}

		if output == "json" {
			return printJSON(results)
		}

		fmt.Printf("批量创建成功！共 %d 条记录\n", len(results))
		for _, r := range results {
			fmt.Printf("  Record ID: %s\n", r.RecordID)
		}
		return nil
	},
}

var bitableUpdateRecordCmd = &cobra.Command{
	Use:   "update-record <app_token> <table_id> <record_id>",
	Short: "更新记录",
	Long:  "更新单条记录的字段值",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		appToken := args[0]
		tableID := args[1]
		recordID := args[2]
		fieldsJSON, _ := cmd.Flags().GetString("fields")
		fieldsFile, _ := cmd.Flags().GetString("fields-file")
		output, _ := cmd.Flags().GetString("output")
		userToken := resolveOptionalUserToken(cmd)

		fieldsJSON, err := loadJSONInput(fieldsJSON, fieldsFile, "fields", "fields-file", "字段值 JSON")
		if err != nil {
			return err
		}

		var fields map[string]any
		if err := json.Unmarshal([]byte(fieldsJSON), &fields); err != nil {
			return fmt.Errorf("解析字段值 JSON 失败: %w", err)
		}

		record, err := client.UpdateBitableRecord(appToken, tableID, recordID, fields, userToken)
		if err != nil {
			return err
		}

		if output == "json" {
			return printJSON(record)
		}

		fmt.Printf("更新成功！Record ID: %s\n", record.RecordID)
		return nil
	},
}

var bitableUpdateRecordsCmd = &cobra.Command{
	Use:   "update-records <app_token> <table_id>",
	Short: "批量更新记录",
	Long: `批量更新记录（最多 500 条）。

数据格式为 JSON 数组，每个元素需包含 record_id 和 fields:
  [{"record_id":"recxxx","fields":{"状态":"已完成"}},{"record_id":"recyyy","fields":{"金额":300}}]`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		appToken := args[0]
		tableID := args[1]
		dataJSON, _ := cmd.Flags().GetString("data")
		dataFile, _ := cmd.Flags().GetString("data-file")
		output, _ := cmd.Flags().GetString("output")
		userToken := resolveOptionalUserToken(cmd)

		dataJSON, err := loadJSONInput(dataJSON, dataFile, "data", "data-file", "记录数据 JSON")
		if err != nil {
			return err
		}

		var records []map[string]any
		if err := json.Unmarshal([]byte(dataJSON), &records); err != nil {
			return fmt.Errorf("解析数据 JSON 失败: %w", err)
		}

		results, err := client.BatchUpdateBitableRecords(appToken, tableID, records, userToken)
		if err != nil {
			return err
		}

		if output == "json" {
			return printJSON(results)
		}

		fmt.Printf("批量更新成功！共 %d 条记录\n", len(results))
		for _, r := range results {
			fmt.Printf("  Record ID: %s\n", r.RecordID)
		}
		return nil
	},
}

var bitableDeleteRecordsCmd = &cobra.Command{
	Use:   "delete-records <app_token> <table_id>",
	Short: "批量删除记录",
	Long: `批量删除记录（最多 500 条）。

通过 --record-ids 指定要删除的记录 ID（逗号分隔）。`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		appToken := args[0]
		tableID := args[1]
		recordIDsStr, _ := cmd.Flags().GetString("record-ids")
		userToken := resolveOptionalUserToken(cmd)

		recordIDs := splitAndTrim(recordIDsStr)
		if len(recordIDs) == 0 {
			return fmt.Errorf("请指定要删除的记录 ID（--record-ids）")
		}

		if err := client.BatchDeleteBitableRecords(appToken, tableID, recordIDs, userToken); err != nil {
			return err
		}

		fmt.Printf("删除成功！共删除 %d 条记录\n", len(recordIDs))
		return nil
	},
}

func init() {
	bitableCmd.AddCommand(bitableRecordsCmd)
	bitableCmd.AddCommand(bitableGetRecordCmd)
	bitableCmd.AddCommand(bitableAddRecordCmd)
	bitableCmd.AddCommand(bitableAddRecordsCmd)
	bitableCmd.AddCommand(bitableUpdateRecordCmd)
	bitableCmd.AddCommand(bitableUpdateRecordsCmd)
	bitableCmd.AddCommand(bitableDeleteRecordsCmd)

	// records (search)
	bitableRecordsCmd.Flags().StringP("output", "o", "text", "输出格式: text, json")
	bitableRecordsCmd.Flags().Int("page-size", 20, "每页记录数（最大 500）")
	bitableRecordsCmd.Flags().String("page-token", "", "分页标记")
	bitableRecordsCmd.Flags().String("filter", "", "过滤条件 JSON")
	bitableRecordsCmd.Flags().String("sort", "", "排序条件 JSON")
	bitableRecordsCmd.Flags().String("field-names", "", "指定返回的字段名（逗号分隔）")
	bitableRecordsCmd.Flags().String("user-access-token", "", "User Access Token（可选）")

	// get-record
	bitableGetRecordCmd.Flags().StringP("output", "o", "text", "输出格式: text, json")
	bitableGetRecordCmd.Flags().String("user-access-token", "", "User Access Token（可选）")

	// add-record
	bitableAddRecordCmd.Flags().String("fields", "", "字段值 JSON")
	bitableAddRecordCmd.Flags().String("fields-file", "", "字段值 JSON 文件")
	bitableAddRecordCmd.Flags().StringP("output", "o", "text", "输出格式: text, json")
	bitableAddRecordCmd.Flags().String("user-access-token", "", "User Access Token（可选）")
	bitableAddRecordCmd.MarkFlagsOneRequired("fields", "fields-file")
	bitableAddRecordCmd.MarkFlagsMutuallyExclusive("fields", "fields-file")

	// add-records
	bitableAddRecordsCmd.Flags().String("data", "", "记录数据 JSON 数组")
	bitableAddRecordsCmd.Flags().String("data-file", "", "记录数据 JSON 文件")
	bitableAddRecordsCmd.Flags().StringP("output", "o", "text", "输出格式: text, json")
	bitableAddRecordsCmd.Flags().String("user-access-token", "", "User Access Token（可选）")
	bitableAddRecordsCmd.MarkFlagsOneRequired("data", "data-file")
	bitableAddRecordsCmd.MarkFlagsMutuallyExclusive("data", "data-file")

	// update-record
	bitableUpdateRecordCmd.Flags().String("fields", "", "字段值 JSON")
	bitableUpdateRecordCmd.Flags().String("fields-file", "", "字段值 JSON 文件")
	bitableUpdateRecordCmd.Flags().StringP("output", "o", "text", "输出格式: text, json")
	bitableUpdateRecordCmd.Flags().String("user-access-token", "", "User Access Token（可选）")
	bitableUpdateRecordCmd.MarkFlagsOneRequired("fields", "fields-file")
	bitableUpdateRecordCmd.MarkFlagsMutuallyExclusive("fields", "fields-file")

	// update-records
	bitableUpdateRecordsCmd.Flags().String("data", "", "记录数据 JSON 数组")
	bitableUpdateRecordsCmd.Flags().String("data-file", "", "记录数据 JSON 文件")
	bitableUpdateRecordsCmd.Flags().StringP("output", "o", "text", "输出格式: text, json")
	bitableUpdateRecordsCmd.Flags().String("user-access-token", "", "User Access Token（可选）")
	bitableUpdateRecordsCmd.MarkFlagsOneRequired("data", "data-file")
	bitableUpdateRecordsCmd.MarkFlagsMutuallyExclusive("data", "data-file")

	// delete-records
	bitableDeleteRecordsCmd.Flags().String("record-ids", "", "记录 ID 列表（逗号分隔）")
	bitableDeleteRecordsCmd.Flags().String("user-access-token", "", "User Access Token（可选）")
	mustMarkFlagRequired(bitableDeleteRecordsCmd, "record-ids")
}

// formatFieldValue 格式化字段值用于文本输出
func formatFieldValue(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%g", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	case []any:
		parts := make([]string, len(val))
		for i, item := range val {
			parts[i] = formatFieldValue(item)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]any:
		// 超链接或人员等复合类型
		if text, ok := val["text"]; ok {
			if link, ok := val["link"]; ok {
				return fmt.Sprintf("%v (%v)", text, link)
			}
			return fmt.Sprintf("%v", text)
		}
		if name, ok := val["name"]; ok {
			return fmt.Sprintf("%v", name)
		}
		data, _ := json.Marshal(val)
		return string(data)
	case nil:
		return "<空>"
	default:
		return fmt.Sprintf("%v", val)
	}
}
