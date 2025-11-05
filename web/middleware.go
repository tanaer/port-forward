package web

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"goForward/errors"
)

// ErrorHandler 统一错误处理中间件
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// 检查是否有错误
		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err

			// 如果是AppError，使用其定义的状态码和消息
			if appErr, ok := err.(*errors.AppError); ok {
				c.JSON(appErr.Status, gin.H{
					"code":    appErr.Code,
					"message": appErr.Message,
					"details": appErr.Details,
				})
				return
			}

			// 其他错误统一返回500
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "内部服务器错误",
				"details": err.Error(),
			})
		}
	}
}

// RecoverHandler 恢复panic的中间件
func RecoverHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"code":    "PANIC",
					"message": "服务器内部错误",
					"details": err,
				})
				c.Abort()
			}
		}()
		c.Next()
	}
}
