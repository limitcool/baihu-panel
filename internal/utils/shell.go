package utils

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// GetShell 返回当前操作系统的 shell 和参数
func GetShell() (shell string, args []string) {
	if runtime.GOOS == "windows" {
		return "cmd", []string{}
	}

	// 优先使用环境变量中的 SHELL
	if envShell := os.Getenv("SHELL"); envShell != "" {
		if _, err := os.Stat(envShell); err == nil {
			return envShell, []string{}
		}
	}

	// 尝试在 PATH 中查找 bash
	if path, err := exec.LookPath("bash"); err == nil {
		return path, []string{}
	}

	// 尝试在 PATH 中查找 zsh
	if path, err := exec.LookPath("zsh"); err == nil {
		return path, []string{}
	}

	// 尝试在 PATH 中查找 sh
	if path, err := exec.LookPath("sh"); err == nil {
		return path, []string{}
	}

	// 最后回退到最常见的硬编码路径
	shells := []string{"/bin/bash", "/usr/bin/bash", "/bin/sh"}
	for _, sh := range shells {
		if _, err := os.Stat(sh); err == nil {
			return sh, []string{}
		}
	}

	return "sh", []string{}
}

// GetShellCommand 返回执行命令的 shell 和参数
func GetShellCommand(command string) (shell string, args []string) {
	shell, _ = GetShell()
	if runtime.GOOS == "windows" {
		return shell, []string{"/c", command}
	}
	return shell, []string{"-c", command}
}

// NewShellCmd 创建一个交互式 shell 命令
func NewShellCmd() *exec.Cmd {
	shell, _ := GetShell()
	if runtime.GOOS == "windows" {
		return exec.Command(shell)
	}
	// Unix 系统使用 -i 启用交互模式
	return exec.Command(shell, "-i")
}

// NewShellCommandCmd 创建一个执行指定命令的 shell 命令
func NewShellCommandCmd(command string) *exec.Cmd {
	shell, args := GetShellCommand(command)
	return exec.Command(shell, args...)
}

// QuotePath 转义并包裹路径，防止 Shell 注入
func QuotePath(path string) string {
	if path == "" {
		return "''"
	}
	// 在 Unix-like 系统中，单引号包裹是最安全的
	// 需要将路径中的 ' 替换为 '\'' (结束当前引号，转义一个单引号，重新开启引号)
	return "'" + strings.ReplaceAll(path, "'", "'\\''") + "'"
}
