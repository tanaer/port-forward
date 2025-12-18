package web

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"goForward/proxy"
)

// RegisterInstallerRoutes 注册安装器路由
func RegisterInstallerRoutes(r *gin.Engine) {
	installer := proxy.GetInstaller()

	// 检查环境状态
	r.GET("/api/environment/check", func(c *gin.Context) {
		status := installer.CheckEnvironment()
		c.JSON(200, status)
	})

	// 获取最新版本及发布说明
	r.GET("/api/environment/releases", func(c *gin.Context) {
		info, err := installer.GetReleaseStatus()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": fmt.Sprintf("获取版本信息失败: %v", err),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    info,
		})
	})

	// 安装 Xray
	r.POST("/api/environment/install-xray", func(c *gin.Context) {
		go func() {
			if err := installer.InstallXray(); err != nil {
				fmt.Printf("[Installer] 安装 Xray 失败: %v\n", err)
			}
		}()

		c.JSON(200, gin.H{
			"message": "Xray 安装已开始，请稍候...",
			"success": true,
		})
	})

	// 安装 Hysteria2
	r.POST("/api/environment/install-hy2", func(c *gin.Context) {
		go func() {
			if err := installer.InstallHysteria2(); err != nil {
				fmt.Printf("[Installer] 安装 Hysteria2 失败: %v\n", err)
			}
		}()

		c.JSON(200, gin.H{
			"message": "Hysteria2 安装已开始，请稍候...",
			"success": true,
		})
	})

	// 升级/重装 Xray
	r.POST("/api/environment/upgrade-xray", func(c *gin.Context) {
		go func() {
			if err := installer.InstallXray(); err != nil {
				fmt.Printf("[Installer] 升级 Xray 失败: %v\n", err)
			}
		}()

		c.JSON(200, gin.H{
			"message": "Xray 升级已开始，请稍候...",
			"success": true,
		})
	})

	// 升级/重装 Hysteria2
	r.POST("/api/environment/upgrade-hy2", func(c *gin.Context) {
		go func() {
			if err := installer.InstallHysteria2(); err != nil {
				fmt.Printf("[Installer] 升级 Hysteria2 失败: %v\n", err)
			}
		}()

		c.JSON(200, gin.H{
			"message": "Hysteria2 升级已开始，请稍候...",
			"success": true,
		})
	})

	// 一键安装所有依赖
	r.POST("/api/environment/install-all", func(c *gin.Context) {
		go func() {
			if err := installer.InstallAll(); err != nil {
				fmt.Printf("[Installer] 安装所有依赖失败: %v\n", err)
			}
		}()

		c.JSON(200, gin.H{
			"message": "依赖安装已开始，请稍候刷新页面查看状态...",
			"success": true,
		})
	})

	// 环境状态页面
	r.GET("/environment", func(c *gin.Context) {
		status := installer.CheckEnvironment()
		c.HTML(http.StatusOK, "environment.tmpl", gin.H{
			"status": status,
		})
	})
}
