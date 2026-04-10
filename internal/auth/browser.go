package auth

import (
	"os/exec"
	"runtime"
)

// openBrowser 跨平台打开浏览器
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait() // 回收子进程，避免僵尸进程
	return nil
}

// TryOpenBrowser 尝试打开浏览器。
//
// 与环境无关，无论本地桌面还是 SSH 远程都会尝试调用本地 open/xdg-open，
// 命令失败时静默返回 error（调用方通常 `_ = TryOpenBrowser(...)` 忽略）。
//
// 设计目的：Device Flow 在所有环境下都只把链接打印到 stderr，
// 若本机有 GUI 则顺便尝试打开，没有 GUI 则无副作用。
func TryOpenBrowser(url string) error {
	return openBrowser(url)
}
