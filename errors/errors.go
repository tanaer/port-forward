package errors

import (
	"fmt"
	"net/http"
)

// AppError 应用错误类型
type AppError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
	Status  int    `json:"-"` // HTTP状态码
}

// Error 实现error接口
func (e *AppError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// 预定义错误类型
var (
	// 通用错误
	ErrInternal = &AppError{
		Code:    "INTERNAL_ERROR",
		Message: "内部服务器错误",
		Status:  http.StatusInternalServerError,
	}

	ErrInvalidInput = &AppError{
		Code:    "INVALID_INPUT",
		Message: "输入参数无效",
		Status:  http.StatusBadRequest,
	}

	// 端口转发相关错误
	ErrPortInUse = &AppError{
		Code:    "PORT_IN_USE",
		Message: "端口已被占用",
		Status:  http.StatusConflict,
	}

	ErrForwardNotFound = &AppError{
		Code:    "FORWARD_NOT_FOUND",
		Message: "转发规则不存在",
		Status:  http.StatusNotFound,
	}

	ErrForwardExists = &AppError{
		Code:    "FORWARD_EXISTS",
		Message: "转发规则已存在",
		Status:  http.StatusConflict,
	}

	// 认证相关错误
	ErrUnauthorized = &AppError{
		Code:    "UNAUTHORIZED",
		Message: "未授权访问",
		Status:  http.StatusUnauthorized,
	}

	ErrIPBanned = &AppError{
		Code:    "IP_BANNED",
		Message: "IP已被封禁",
		Status:  http.StatusForbidden,
	}

	// 配置相关错误
	ErrInvalidConfig = &AppError{
		Code:    "INVALID_CONFIG",
		Message: "配置无效",
		Status:  http.StatusBadRequest,
	}

	ErrConfigNotFound = &AppError{
		Code:    "CONFIG_NOT_FOUND",
		Message: "配置不存在",
		Status:  http.StatusNotFound,
	}
)

// New 创建新的AppError
func New(code, message string, status int) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Status:  status,
	}
}

// WithDetails 添加详细信息
func (e *AppError) WithDetails(details string) *AppError {
	return &AppError{
		Code:    e.Code,
		Message: e.Message,
		Details: details,
		Status:  e.Status,
	}
}

// WithDetailsf 添加格式化的详细信息
func (e *AppError) WithDetailsf(format string, args ...interface{}) *AppError {
	return e.WithDetails(fmt.Sprintf(format, args...))
}

// Wrap 包装标准error为AppError
func Wrap(err error, appErr *AppError) *AppError {
	if err == nil {
		return nil
	}

	// 如果已经是AppError，直接返回
	if ae, ok := err.(*AppError); ok {
		return ae
	}

	// 包装为AppError
	return appErr.WithDetails(err.Error())
}
