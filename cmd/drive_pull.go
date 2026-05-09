package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/riba2534/feishu-cli/internal/client"
	"github.com/riba2534/feishu-cli/internal/config"
	"github.com/spf13/cobra"
)

const (
	driveMirrorIfExistsOverwrite = "overwrite"
	driveMirrorIfExistsSkip      = "skip"
)

var drivePullCmd = &cobra.Command{
	Use:   "pull",
	Short: "把云盘文件夹镜像到本地（Drive → 本地，单向 file-level 镜像）",
	Long: `递归列举 --folder-token 下的所有 type=file 条目，下载到 --local-dir 的对应路径。
type=folder/docx/sheet/bitable/mindnote/slides/shortcut 不会作为可下载条目（在线文档没有等价本地文件）。

可选 --delete-local --yes 同时清理本地不存在于远端的 regular file（高危，必须双确认）。
失败时不会触发删除阶段，避免「半同步」状态。

必填:
  --folder-token   云盘根文件夹 token
  --local-dir      本地根目录（必须在 cwd 子树内）

可选:
  --if-exists       overwrite（默认）/ skip：本地同路径已存在时如何处理
  --delete-local    清理本地不存在于远端的 regular file（高危）
  --yes             与 --delete-local 配套，确认删除
  --output / -o     输出格式（json）
  --user-access-token  覆盖登录态

权限:
  - User Access Token 或 Tenant Token
  - drive:drive.metadata:readonly
  - drive:file:download

示例:
  feishu-cli drive pull --folder-token fldxxx --local-dir ./mirror
  feishu-cli drive pull --folder-token fldxxx --local-dir ./mirror --if-exists skip
  feishu-cli drive pull --folder-token fldxxx --local-dir ./mirror --delete-local --yes`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.Validate(); err != nil {
			return err
		}

		folderToken, _ := cmd.Flags().GetString("folder-token")
		localDir, _ := cmd.Flags().GetString("local-dir")
		ifExists, _ := cmd.Flags().GetString("if-exists")
		deleteLocal, _ := cmd.Flags().GetBool("delete-local")
		yes, _ := cmd.Flags().GetBool("yes")
		output, _ := cmd.Flags().GetString("output")

		if folderToken == "" {
			return fmt.Errorf("--folder-token 必填")
		}
		if localDir == "" {
			return fmt.Errorf("--local-dir 必填")
		}
		if ifExists == "" {
			ifExists = driveMirrorIfExistsOverwrite
		}
		if ifExists != driveMirrorIfExistsOverwrite && ifExists != driveMirrorIfExistsSkip {
			return fmt.Errorf("--if-exists 只能是 overwrite 或 skip")
		}
		if deleteLocal && !yes {
			return fmt.Errorf("--delete-local 是高危操作，必须同时加 --yes 才执行")
		}

		safeRoot, _, err := resolveSafeLocalDir(localDir)
		if err != nil {
			return err
		}

		userToken := resolveOptionalUserTokenWithFallback(cmd)

		fmt.Fprintf(cmd.ErrOrStderr(), "列举云盘文件夹: %s\n", folderToken)
		entries, err := client.ListFolderRecursive(folderToken, userToken)
		if err != nil {
			return err
		}
		remoteFiles := remoteFilesOnly(entries)
		// remotePaths 含 folder/docx/sheet 等所有条目，用于 --delete-local 守门
		remotePaths := make(map[string]struct{}, len(entries))
		for rel := range entries {
			remotePaths[rel] = struct{}{}
		}

		type item struct {
			RelPath   string `json:"rel_path"`
			FileToken string `json:"file_token,omitempty"`
			Action    string `json:"action"` // downloaded / skipped / failed / deleted_local / delete_failed
			Error     string `json:"error,omitempty"`
		}
		var items []item
		var downloaded, skipped, failed, deletedLocal, downloadFailed int

		// 稳定顺序
		sortedRels := make([]string, 0, len(remoteFiles))
		for rel := range remoteFiles {
			sortedRels = append(sortedRels, rel)
		}
		sort.Strings(sortedRels)

		for _, rel := range sortedRels {
			token := remoteFiles[rel]
			target := filepath.Join(safeRoot, filepath.FromSlash(rel))

			if info, statErr := os.Stat(target); statErr == nil {
				if info.IsDir() {
					items = append(items, item{
						RelPath:   rel,
						FileToken: token,
						Action:    "failed",
						Error:     "本地同路径是目录，远端是文件",
					})
					failed++
					downloadFailed++
					continue
				}
				if ifExists == driveMirrorIfExistsSkip {
					items = append(items, item{RelPath: rel, FileToken: token, Action: "skipped"})
					skipped++
					continue
				}
			}

			// 确保父目录存在
			if mkErr := os.MkdirAll(filepath.Dir(target), 0755); mkErr != nil {
				items = append(items, item{RelPath: rel, FileToken: token, Action: "failed", Error: mkErr.Error()})
				failed++
				downloadFailed++
				continue
			}
			if dlErr := client.DownloadFileWithToken(token, target, userToken); dlErr != nil {
				items = append(items, item{RelPath: rel, FileToken: token, Action: "failed", Error: dlErr.Error()})
				failed++
				downloadFailed++
				continue
			}
			items = append(items, item{RelPath: rel, FileToken: token, Action: "downloaded"})
			downloaded++
		}

		// --delete-local 在下载阶段无失败时才执行，避免半同步状态
		if deleteLocal && downloadFailed == 0 {
			localFiles, walkErr := walkLocalRegularFiles(safeRoot)
			if walkErr != nil {
				return walkErr
			}
			locals := make([]string, 0, len(localFiles))
			for rel := range localFiles {
				locals = append(locals, rel)
			}
			sort.Strings(locals)

			for _, rel := range locals {
				if _, ok := remotePaths[rel]; ok {
					// 即使 type 不是 file（如 docx 在线文档同名），也保留本地文件不删
					continue
				}
				abs := localFiles[rel]
				if rmErr := os.Remove(abs); rmErr != nil {
					items = append(items, item{RelPath: rel, Action: "delete_failed", Error: rmErr.Error()})
					failed++
					continue
				}
				items = append(items, item{RelPath: rel, Action: "deleted_local"})
				deletedLocal++
			}
		} else if deleteLocal && downloadFailed > 0 {
			fmt.Fprintf(cmd.ErrOrStderr(),
				"⚠ 跳过 --delete-local：上面有 %d 个下载失败，避免半同步状态。修复后重跑。\n", downloadFailed)
		}

		summary := map[string]any{
			"downloaded":    downloaded,
			"skipped":       skipped,
			"failed":        failed,
			"deleted_local": deletedLocal,
		}
		payload := map[string]any{
			"summary": summary,
			"items":   items,
		}

		if output == "json" {
			if err := printJSON(payload); err != nil {
				return err
			}
		} else {
			fmt.Printf("下载: %d  跳过: %d  删除本地: %d  失败: %d\n",
				downloaded, skipped, deletedLocal, failed)
			for _, it := range items {
				if it.Action == "failed" || it.Action == "delete_failed" {
					fmt.Printf("  ⚠ %-15s %s -- %s\n", it.Action, it.RelPath, it.Error)
				}
			}
		}

		if failed > 0 {
			return fmt.Errorf("有 %d 项失败，处于部分同步状态；修复后重跑", failed)
		}
		return nil
	},
}

func init() {
	driveCmd.AddCommand(drivePullCmd)
	drivePullCmd.Flags().String("folder-token", "", "云盘根文件夹 token（必填）")
	drivePullCmd.Flags().String("local-dir", "", "本地根目录（必填）")
	drivePullCmd.Flags().String("if-exists", driveMirrorIfExistsOverwrite, "overwrite / skip")
	drivePullCmd.Flags().Bool("delete-local", false, "清理本地不存在于远端的文件（高危，需 --yes）")
	drivePullCmd.Flags().Bool("yes", false, "与 --delete-local 配套确认删除")
	drivePullCmd.Flags().StringP("output", "o", "", "输出格式（json）")
	drivePullCmd.Flags().String("user-access-token", "", "User Access Token（覆盖登录态）")
	mustMarkFlagRequired(drivePullCmd, "folder-token")
	mustMarkFlagRequired(drivePullCmd, "local-dir")
}
