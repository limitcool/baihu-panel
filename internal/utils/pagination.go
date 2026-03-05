package utils

import (
	"strconv"

	"github.com/engigu/baihu-panel/internal/cache"
	"github.com/engigu/baihu-panel/internal/constant"

	"github.com/gin-gonic/gin"
)

// ParseInt 解析字符串为整数
func ParseInt(s string) (int, error) {
	return strconv.Atoi(s)
}

// Pagination 分页参数
type Pagination struct {
	Page     int
	PageSize int
}

// getDefaultPageSize 从缓存获取默认分页大小
func getDefaultPageSize() int {
	pageSizeStr := cache.GetSiteCache(constant.KeyPageSize)
	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil || pageSize < 1 {
		return 10
	}
	return pageSize
}

// ParsePagination 从请求中解析分页参数
func ParsePagination(c *gin.Context) Pagination {
	defaultPageSize := getDefaultPageSize()
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", strconv.Itoa(defaultPageSize)))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = defaultPageSize
	}

	return Pagination{Page: page, PageSize: pageSize}
}

// Offset 计算偏移量
func (p Pagination) Offset() int {
	return (p.Page - 1) * p.PageSize
}

// PaginationData 分页数据
type PaginationData struct {
	List     interface{} `json:"list"`
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
}

// PaginatedResponse 分页响应
func PaginatedResponse(c *gin.Context, data interface{}, total int64, p Pagination) {
	Success(c, PaginationData{
		List:     data,
		Total:    total,
		Page:     p.Page,
		PageSize: p.PageSize,
	})
}
