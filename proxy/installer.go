package proxy

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// DependencyStatus 依赖状态
type DependencyStatus struct {
	Name      string `json:"name"`
	Installed bool   `json:"installed"`
	Version   string `json:"version"`
	Path      string `json:"path"`
	Required  bool   `json:"required"`
}

// ReleaseInfo 描述上游最新发行版
type ReleaseInfo struct {
	Name        string `json:"name"`
	Tag         string `json:"tag"`
	Body        string `json:"body"`
	URL         string `json:"url"`
	PublishedAt string `json:"publishedAt"`
}

// ReleaseStatus 聚合当前安装和最新版本信息
type ReleaseStatus struct {
	Xray      ReleaseSummary `json:"xray"`
	Hysteria2 ReleaseSummary `json:"hysteria2"`
}

// ReleaseSummary 单个依赖的版本/更新摘要
type ReleaseSummary struct {
	Name             string `json:"name"`
	InstalledVersion string `json:"installedVersion"`
	LatestVersion    string `json:"latestVersion"`
	PublishedAt      string `json:"publishedAt"`
	URL              string `json:"url"`
	Notes            string `json:"notes"`
}

// EnvironmentStatus 环境状态
type EnvironmentStatus struct {
	Ready        bool               `json:"ready"`
	Dependencies []DependencyStatus `json:"dependencies"`
	Message      string             `json:"message"`
}

// Installer 依赖安装器
type Installer struct {
	BinDir string
}

// NewInstaller 创建安装器
func NewInstaller() *Installer {
	execPath, _ := os.Executable()
	binDir := filepath.Join(filepath.Dir(execPath), "bin")
	os.MkdirAll(binDir, 0755)

	return &Installer{
		BinDir: binDir,
	}
}

// CheckEnvironment 检查环境状态
func (i *Installer) CheckEnvironment() EnvironmentStatus {
	status := EnvironmentStatus{
		Ready:        true,
		Dependencies: []DependencyStatus{},
	}

	// 检查 Xray
	xrayStatus := i.checkXray()
	status.Dependencies = append(status.Dependencies, xrayStatus)
	if !xrayStatus.Installed {
		status.Ready = false
	}

	// 检查 Hysteria2
	hy2Status := i.checkHysteria2()
	status.Dependencies = append(status.Dependencies, hy2Status)
	if !hy2Status.Installed {
		status.Ready = false
	}

	// 设置消息
	if status.Ready {
		status.Message = "环境已就绪，可以使用代理功能"
	} else {
		status.Message = "环境未就绪，请安装缺失的依赖"
	}

	return status
}

// checkXray 检查 Xray 是否安装
func (i *Installer) checkXray() DependencyStatus {
	status := DependencyStatus{
		Name:     "Xray-core",
		Required: true,
	}

	// 先检查本地 bin 目录
	localXray := filepath.Join(i.BinDir, "xray")
	if _, err := os.Stat(localXray); err == nil {
		status.Installed = true
		status.Path = localXray
		status.Version = i.getXrayVersion(localXray)
		return status
	}

	// 检查系统 PATH
	if xrayPath, err := exec.LookPath("xray"); err == nil {
		status.Installed = true
		status.Path = xrayPath
		status.Version = i.getXrayVersion(xrayPath)
		return status
	}

	status.Installed = false
	return status
}

// checkHysteria2 检查 Hysteria2 是否安装
func (i *Installer) checkHysteria2() DependencyStatus {
	status := DependencyStatus{
		Name:     "Hysteria2",
		Required: true,
	}

	// 先检查本地 bin 目录
	localHy2 := filepath.Join(i.BinDir, "hysteria2")
	if _, err := os.Stat(localHy2); err == nil {
		status.Installed = true
		status.Path = localHy2
		status.Version = i.getHy2Version(localHy2)
		return status
	}

	// 检查 hysteria 命名
	localHy := filepath.Join(i.BinDir, "hysteria")
	if _, err := os.Stat(localHy); err == nil {
		status.Installed = true
		status.Path = localHy
		status.Version = i.getHy2Version(localHy)
		return status
	}

	// 检查系统 PATH
	if hy2Path, err := exec.LookPath("hysteria2"); err == nil {
		status.Installed = true
		status.Path = hy2Path
		status.Version = i.getHy2Version(hy2Path)
		return status
	}

	if hyPath, err := exec.LookPath("hysteria"); err == nil {
		status.Installed = true
		status.Path = hyPath
		status.Version = i.getHy2Version(hyPath)
		return status
	}

	status.Installed = false
	return status
}

// getXrayVersion 获取 Xray 版本
func (i *Installer) getXrayVersion(path string) string {
	cmd := exec.Command(path, "version")
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "Xray") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[1]
			}
		}
	}

	return "unknown"
}

// getHy2Version 获取 Hysteria2 版本
func (i *Installer) getHy2Version(path string) string {
	cmd := exec.Command(path, "version")
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "hysteria") || strings.HasPrefix(line, "Hysteria") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[1]
			}
		}
	}

	return "unknown"
}

// InstallXray 安装 Xray
func (i *Installer) InstallXray() error {
	fmt.Println("[Installer] 开始安装 Xray-core...")

	// 获取最新版本
	latestVersion, err := i.getLatestXrayVersion()
	if err != nil {
		return fmt.Errorf("获取 Xray 最新版本失败: %v", err)
	}

	fmt.Printf("[Installer] 最新版本: %s\n", latestVersion)

	// 确定平台
	osType := runtime.GOOS
	archType := runtime.GOARCH

	var downloadURL string
	var fileName string

	if osType == "linux" {
		if archType == "amd64" {
			fileName = "Xray-linux-64.zip"
		} else if archType == "arm64" {
			fileName = "Xray-linux-arm64-v8a.zip"
		} else {
			return fmt.Errorf("不支持的架构: %s", archType)
		}
	} else {
		return fmt.Errorf("不支持的操作系统: %s", osType)
	}

	downloadURL = fmt.Sprintf("https://github.com/XTLS/Xray-core/releases/download/%s/%s", latestVersion, fileName)
	fmt.Printf("[Installer] 下载地址: %s\n", downloadURL)

	// 下载文件
	zipPath := filepath.Join(i.BinDir, fileName)
	if err := i.downloadFile(downloadURL, zipPath); err != nil {
		return fmt.Errorf("下载 Xray 失败: %v", err)
	}

	// 解压文件
	if err := i.unzipXray(zipPath); err != nil {
		os.Remove(zipPath)
		return fmt.Errorf("解压 Xray 失败: %v", err)
	}

	// 删除压缩包
	os.Remove(zipPath)

	// 设置执行权限
	xrayPath := filepath.Join(i.BinDir, "xray")
	os.Chmod(xrayPath, 0755)

	fmt.Println("[Installer] Xray 安装完成")
	return nil
}

// InstallHysteria2 安装 Hysteria2
func (i *Installer) InstallHysteria2() error {
	fmt.Println("[Installer] 开始安装 Hysteria2...")

	// 获取最新版本
	latestVersion, err := i.getLatestHy2Version()
	if err != nil {
		return fmt.Errorf("获取 Hysteria2 最新版本失败: %v", err)
	}

	fmt.Printf("[Installer] 最新版本: %s\n", latestVersion)

	// 确定平台
	osType := runtime.GOOS
	archType := runtime.GOARCH

	var fileName string

	if osType == "linux" {
		if archType == "amd64" {
			fileName = "hysteria-linux-amd64"
		} else if archType == "arm64" {
			fileName = "hysteria-linux-arm64"
		} else {
			return fmt.Errorf("不支持的架构: %s", archType)
		}
	} else {
		return fmt.Errorf("不支持的操作系统: %s", osType)
	}

	// 移除 v 前缀
	versionTag := latestVersion
	if !strings.HasPrefix(versionTag, "app/") {
		versionTag = "app/" + strings.TrimPrefix(versionTag, "v")
	}

	downloadURL := fmt.Sprintf("https://github.com/apernet/hysteria/releases/download/%s/%s", versionTag, fileName)
	fmt.Printf("[Installer] 下载地址: %s\n", downloadURL)

	// 下载文件
	hy2Path := filepath.Join(i.BinDir, "hysteria2")
	if err := i.downloadFile(downloadURL, hy2Path); err != nil {
		return fmt.Errorf("下载 Hysteria2 失败: %v", err)
	}

	// 设置执行权限
	os.Chmod(hy2Path, 0755)

	fmt.Println("[Installer] Hysteria2 安装完成")
	return nil
}

// getLatestXrayVersion 获取 Xray 最新版本
func (i *Installer) getLatestXrayVersion() (string, error) {
	info, err := i.fetchReleaseInfo("https://api.github.com/repos/XTLS/Xray-core/releases/latest")
	if err != nil {
		return "", err
	}
	return info.Tag, nil
}

// getLatestHy2Version 获取 Hysteria2 最新版本
func (i *Installer) getLatestHy2Version() (string, error) {
	info, err := i.fetchReleaseInfo("https://api.github.com/repos/apernet/hysteria/releases/latest")
	if err != nil {
		return "", err
	}
	return info.Tag, nil
}

// downloadFile 下载文件
func (i *Installer) downloadFile(url, destPath string) error {
	fmt.Printf("[Installer] 开始下载: %s\n", url)

	client := &http.Client{
		Timeout: 5 * time.Minute,
	}

	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败: HTTP %d", resp.StatusCode)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	fmt.Println("[Installer] 下载完成")
	return err
}

// GetReleaseStatus 获取依赖的当前/最新版本及发布说明摘要
func (i *Installer) GetReleaseStatus() (ReleaseStatus, error) {
	env := i.CheckEnvironment()
	status := ReleaseStatus{
		Xray:      ReleaseSummary{Name: "Xray-core"},
		Hysteria2: ReleaseSummary{Name: "Hysteria2"},
	}

	// 读取已安装版本
	for _, dep := range env.Dependencies {
		if dep.Name == "Xray-core" {
			status.Xray.InstalledVersion = dep.Version
		}
		if dep.Name == "Hysteria2" {
			status.Hysteria2.InstalledVersion = dep.Version
		}
	}

	xrayRelease, err := i.fetchReleaseInfo("https://api.github.com/repos/XTLS/Xray-core/releases/latest")
	if err != nil {
		return status, err
	}
	hyRelease, err := i.fetchReleaseInfo("https://api.github.com/repos/apernet/hysteria/releases/latest")
	if err != nil {
		return status, err
	}

	status.Xray.LatestVersion = xrayRelease.Tag
	status.Xray.PublishedAt = xrayRelease.PublishedAt
	status.Xray.URL = xrayRelease.URL
	status.Xray.Notes = trimReleaseNotes(xrayRelease.Body)

	status.Hysteria2.LatestVersion = hyRelease.Tag
	status.Hysteria2.PublishedAt = hyRelease.PublishedAt
	status.Hysteria2.URL = hyRelease.URL
	status.Hysteria2.Notes = trimReleaseNotes(hyRelease.Body)

	return status, nil
}

// fetchReleaseInfo 请求 GitHub release 信息
func (i *Installer) fetchReleaseInfo(url string) (*ReleaseInfo, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("获取版本信息失败: HTTP %d", resp.StatusCode)
	}

	var release struct {
		TagName     string `json:"tag_name"`
		Name        string `json:"name"`
		Body        string `json:"body"`
		HTMLURL     string `json:"html_url"`
		PublishedAt string `json:"published_at"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &ReleaseInfo{
		Name:        release.Name,
		Tag:         release.TagName,
		Body:        release.Body,
		URL:         release.HTMLURL,
		PublishedAt: release.PublishedAt,
	}, nil
}

// trimReleaseNotes 截断发布说明，避免超长输出
func trimReleaseNotes(body string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return "暂无发布说明"
	}

	const maxLen = 1200
	runes := []rune(body)
	if len(runes) > maxLen {
		runes = runes[:maxLen]
		return string(runes) + "..."
	}
	return string(runes)
}

// unzipXray 解压 Xray
func (i *Installer) unzipXray(zipPath string) error {
	fmt.Println("[Installer] 开始解压...")

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		// 只解压 xray 可执行文件
		if f.Name != "xray" {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		destPath := filepath.Join(i.BinDir, f.Name)
		outFile, err := os.Create(destPath)
		if err != nil {
			rc.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}

		fmt.Printf("[Installer] 已解压: %s\n", f.Name)
	}

	return nil
}

// InstallAll 安装所有依赖
func (i *Installer) InstallAll() error {
	// 检查环境
	status := i.CheckEnvironment()

	// 安装缺失的依赖
	for _, dep := range status.Dependencies {
		if !dep.Installed {
			fmt.Printf("[Installer] 安装 %s...\n", dep.Name)

			var err error
			if dep.Name == "Xray-core" {
				err = i.InstallXray()
			} else if dep.Name == "Hysteria2" {
				err = i.InstallHysteria2()
			}

			if err != nil {
				return fmt.Errorf("安装 %s 失败: %v", dep.Name, err)
			}
		} else {
			fmt.Printf("[Installer] %s 已安装: %s\n", dep.Name, dep.Version)
		}
	}

	return nil
}

// GetGlobalInstaller 获取全局安装器
var globalInstaller *Installer

func GetInstaller() *Installer {
	if globalInstaller == nil {
		globalInstaller = NewInstaller()
	}
	return globalInstaller
}
