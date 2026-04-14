package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/riba2534/feishu-cli/internal/client"
	"github.com/riba2534/feishu-cli/internal/config"
	"github.com/spf13/cobra"
)

var boardUpdateCmd = &cobra.Command{
	Use:   "update <whiteboard_id> [nodes_json_file]",
	Short: "更新画板内容（支持覆盖）",
	Long: `更新画板节点内容。支持从文件或 stdin 读取节点 JSON。

--overwrite 模式会先创建新节点，再删除旧节点（先写后删，保证不会出现空画板）。
--dry-run 模式仅预览，输出将要删除的节点数量，不实际执行。

示例:
  # 从文件更新
  feishu-cli board update BOARD_ID nodes.json

  # 从 stdin 管道更新（覆盖模式）
  cat nodes.json | feishu-cli board update BOARD_ID --stdin --overwrite

  # 预览覆盖操作
  feishu-cli board update BOARD_ID nodes.json --overwrite --dry-run`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.Validate(); err != nil {
			return err
		}

		whiteboardID := args[0]
		useStdin, _ := cmd.Flags().GetBool("stdin")
		overwrite, _ := cmd.Flags().GetBool("overwrite")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		output, _ := cmd.Flags().GetString("output")
		userAccessToken := resolveOptionalUserToken(cmd)

		// 1. 读取节点 JSON（从文件或 stdin）
		var nodesJSON string
		if useStdin {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("读取标准输入失败: %w", err)
			}
			nodesJSON = string(data)
		} else if len(args) >= 2 {
			data, err := os.ReadFile(args[1])
			if err != nil {
				return fmt.Errorf("读取节点文件失败: %w", err)
			}
			nodesJSON = string(data)
		} else {
			return fmt.Errorf("请提供节点 JSON 文件路径，或使用 --stdin 从标准输入读取")
		}

		// 2. dry-run 模式：只统计旧节点数
		if dryRun && overwrite {
			ids, err := extractBoardNodeIDs(whiteboardID, userAccessToken)
			if err != nil {
				return fmt.Errorf("获取画板节点失败: %w", err)
			}
			fmt.Fprintf(os.Stderr, "当前画板有 %d 个节点\n", len(ids))
			fmt.Fprintf(os.Stderr, "覆盖模式将删除这些节点并写入新内容\n")
			return nil
		}

		// 3. 如果 overwrite，先获取旧节点 ID 列表
		var oldNodeIDs []string
		if overwrite {
			ids, err := extractBoardNodeIDs(whiteboardID, userAccessToken)
			if err != nil {
				return fmt.Errorf("获取旧节点失败: %w", err)
			}
			oldNodeIDs = ids
		}

		// 4. 创建新节点
		newNodeIDs, err := client.CreateBoardNodes(whiteboardID, nodesJSON, client.CreateBoardNotesOptions{
			UserAccessToken: userAccessToken,
		})
		if err != nil {
			return fmt.Errorf("创建节点失败: %w", err)
		}
		fmt.Fprintf(os.Stderr, "已创建 %d 个新节点\n", len(newNodeIDs))

		// 5. 如果 overwrite，删除旧节点（不在 newNodeIDs 中的）
		var deletedCount int
		if overwrite && len(oldNodeIDs) > 0 {
			// 构建新节点 ID 集合
			newIDSet := make(map[string]struct{}, len(newNodeIDs))
			for _, id := range newNodeIDs {
				newIDSet[id] = struct{}{}
			}

			// 过滤出需要删除的旧节点
			var toDelete []string
			for _, id := range oldNodeIDs {
				if _, exists := newIDSet[id]; !exists {
					toDelete = append(toDelete, id)
				}
			}

			if len(toDelete) > 0 {
				if err := client.DeleteBoardNodes(whiteboardID, toDelete, userAccessToken); err != nil {
					fmt.Fprintf(os.Stderr, "警告: 删除旧节点失败: %v\n", err)
				} else {
					deletedCount = len(toDelete)
					fmt.Fprintf(os.Stderr, "已删除 %d 个旧节点\n", deletedCount)
				}
			}
		}

		// 6. 输出结果
		if output == "json" {
			result := map[string]any{
				"whiteboard_id": whiteboardID,
				"new_node_ids":  newNodeIDs,
				"created_count": len(newNodeIDs),
			}
			if overwrite {
				result["deleted_count"] = deletedCount
			}
			return printJSON(result)
		}

		fmt.Printf("画板更新成功！\n")
		fmt.Printf("  画板 ID: %s\n", whiteboardID)
		fmt.Printf("  创建节点数: %d\n", len(newNodeIDs))
		if overwrite {
			fmt.Printf("  删除旧节点数: %d\n", deletedCount)
		}
		for i, id := range newNodeIDs {
			fmt.Printf("  [%d] 节点 ID: %s\n", i+1, id)
		}

		return nil
	},
}

// extractBoardNodeIDs 从画板获取所有节点 ID
func extractBoardNodeIDs(whiteboardID, userAccessToken string) ([]string, error) {
	rawJSON, err := client.GetBoardNodes(whiteboardID, userAccessToken)
	if err != nil {
		return nil, err
	}

	// 解析响应，提取节点 ID
	// API 返回格式: {"code":0,"data":{"nodes":{"id1":{...},"id2":{...}}}}
	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Nodes json.RawMessage `json:"nodes"`
		} `json:"data"`
	}

	if err := json.Unmarshal(rawJSON, &resp); err != nil {
		return nil, fmt.Errorf("解析节点响应失败: %w", err)
	}

	if resp.Code != 0 {
		return nil, fmt.Errorf("获取节点失败: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	// nodes 可能是 map[string]any 格式（key 就是 node ID）
	var nodesMap map[string]json.RawMessage
	if err := json.Unmarshal(resp.Data.Nodes, &nodesMap); err != nil {
		// 也可能是数组格式，尝试解析为数组
		var nodesArray []struct {
			ID string `json:"id"`
		}
		if err2 := json.Unmarshal(resp.Data.Nodes, &nodesArray); err2 != nil {
			return nil, fmt.Errorf("解析节点数据失败: map 解析=%w, 数组解析=%v", err, err2)
		}
		ids := make([]string, 0, len(nodesArray))
		for _, n := range nodesArray {
			if n.ID != "" {
				ids = append(ids, n.ID)
			}
		}
		return ids, nil
	}

	ids := make([]string, 0, len(nodesMap))
	for id := range nodesMap {
		ids = append(ids, id)
	}
	return ids, nil
}

func init() {
	boardCmd.AddCommand(boardUpdateCmd)
	boardUpdateCmd.Flags().Bool("stdin", false, "从标准输入读取节点 JSON")
	boardUpdateCmd.Flags().Bool("overwrite", false, "覆盖模式（先写后删）")
	boardUpdateCmd.Flags().Bool("dry-run", false, "仅预览，不实际执行")
	boardUpdateCmd.Flags().StringP("output", "o", "", "输出格式 (json)")
	boardUpdateCmd.Flags().String("user-access-token", "", "User Access Token")
}
