package cmd

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/riba2534/feishu-cli/internal/client"
	"github.com/riba2534/feishu-cli/internal/config"
	"github.com/spf13/cobra"
)

var driveStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "本地目录 ↔ 云盘文件夹 SHA-256 内容对照",
	Long: `递归列举 --folder-token 下的所有 type=file 条目，遍历 --local-dir 的所有 regular file，
按 SHA-256 内容哈希对照得到四个桶：

  - new_local：仅本地存在
  - new_remote：仅远端存在
  - modified：双方都有但内容不同
  - unchanged：双方都有且哈希一致

仅 type=file 参与对照；docx/sheet/bitable/mindnote/slides 等在线文档没有可哈希的本地等价文件，跳过。

必填:
  --folder-token   云盘根文件夹 token
  --local-dir      本地根目录（必须在当前工作目录的子树内）

可选:
  --output / -o    输出格式（json，默认人读）
  --user-access-token  覆盖登录态

权限:
  - User Access Token 或 Tenant Token
  - drive:drive.metadata:readonly
  - drive:file:download

示例:
  feishu-cli drive status --folder-token fldxxx --local-dir ./mirror
  feishu-cli drive status --folder-token fldxxx --local-dir ./mirror -o json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.Validate(); err != nil {
			return err
		}

		folderToken, _ := cmd.Flags().GetString("folder-token")
		localDir, _ := cmd.Flags().GetString("local-dir")
		output, _ := cmd.Flags().GetString("output")
		if folderToken == "" {
			return fmt.Errorf("--folder-token 必填")
		}
		if localDir == "" {
			return fmt.Errorf("--local-dir 必填")
		}

		safeRoot, _, err := resolveSafeLocalDir(localDir)
		if err != nil {
			return err
		}

		userToken := resolveOptionalUserTokenWithFallback(cmd)

		fmt.Fprintf(cmd.ErrOrStderr(), "扫描本地: %s\n", safeRoot)
		localFiles, err := walkLocalRegularFiles(safeRoot)
		if err != nil {
			return err
		}
		localHashes := make(map[string]string, len(localFiles))
		for rel, abs := range localFiles {
			h, hErr := client.HashLocalFile(abs)
			if hErr != nil {
				return fmt.Errorf("计算本地哈希失败 (%s): %w", rel, hErr)
			}
			localHashes[rel] = h
		}

		fmt.Fprintf(cmd.ErrOrStderr(), "列举云盘文件夹: %s\n", folderToken)
		entries, err := client.ListFolderRecursive(folderToken, userToken)
		if err != nil {
			return err
		}
		remoteFiles := remoteFilesOnly(entries)

		// 合并 path 集合
		paths := map[string]struct{}{}
		for p := range localHashes {
			paths[p] = struct{}{}
		}
		for p := range remoteFiles {
			paths[p] = struct{}{}
		}
		sortedPaths := make([]string, 0, len(paths))
		for p := range paths {
			sortedPaths = append(sortedPaths, p)
		}
		sort.Strings(sortedPaths)

		type entry struct {
			RelPath   string `json:"rel_path"`
			FileToken string `json:"file_token,omitempty"`
		}
		var newLocal, newRemote, modified, unchanged []entry

		for _, rel := range sortedPaths {
			localHash, hasLocal := localHashes[rel]
			remoteToken, hasRemote := remoteFiles[rel]
			switch {
			case hasLocal && !hasRemote:
				newLocal = append(newLocal, entry{RelPath: rel})
			case !hasLocal && hasRemote:
				newRemote = append(newRemote, entry{RelPath: rel, FileToken: remoteToken})
			default:
				remoteHash, hErr := client.HashRemoteFile(remoteToken, userToken)
				if hErr != nil {
					return fmt.Errorf("计算远端哈希失败 (%s): %w", rel, hErr)
				}
				if localHash == remoteHash {
					unchanged = append(unchanged, entry{RelPath: rel, FileToken: remoteToken})
				} else {
					modified = append(modified, entry{RelPath: rel, FileToken: remoteToken})
				}
			}
		}

		result := map[string]any{
			"new_local":  emptyOrSlice(newLocal),
			"new_remote": emptyOrSlice(newRemote),
			"modified":   emptyOrSlice(modified),
			"unchanged":  emptyOrSlice(unchanged),
		}

		if output == "json" {
			return printJSON(result)
		}

		printBucket := func(label string, items []entry) {
			fmt.Printf("[%s] %d 项\n", label, len(items))
			for _, it := range items {
				fmt.Printf("  %s", it.RelPath)
				if it.FileToken != "" {
					fmt.Printf("  (token=%s)", it.FileToken)
				}
				fmt.Println()
			}
		}
		printBucket("仅本地 new_local", newLocal)
		printBucket("仅远端 new_remote", newRemote)
		printBucket("内容不同 modified", modified)
		fmt.Printf("[内容一致 unchanged] %d 项\n", len(unchanged))
		_ = filepath.ToSlash // keep import
		return nil
	},
}

// emptyOrSlice 把 nil 切片转为空切片，避免 JSON 出现 null。
func emptyOrSlice[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

func init() {
	driveCmd.AddCommand(driveStatusCmd)
	driveStatusCmd.Flags().String("folder-token", "", "云盘根文件夹 token（必填）")
	driveStatusCmd.Flags().String("local-dir", "", "本地根目录（必填，必须在 cwd 子树内）")
	driveStatusCmd.Flags().StringP("output", "o", "", "输出格式（json）")
	driveStatusCmd.Flags().String("user-access-token", "", "User Access Token（覆盖登录态）")
	mustMarkFlagRequired(driveStatusCmd, "folder-token")
	mustMarkFlagRequired(driveStatusCmd, "local-dir")
}
