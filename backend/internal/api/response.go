package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/expvintl/scopex/internal/i18n"
)

// 统一响应格式：{"data": ...}，列表附带分页信息，错误为 {"error": "..."}。

func ok(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"data": data})
}

func okList(c *gin.Context, data any, total int64, page, size int) {
	c.JSON(http.StatusOK, gin.H{"data": data, "total": total, "page": page, "size": size})
}

func fail(c *gin.Context, status int, msg string) {
	c.AbortWithStatusJSON(status, gin.H{"error": i18n.T(lang(c), msg)})
}

// failf 用于带参数的错误消息，先翻译格式串再填充参数。
func failf(c *gin.Context, status int, format string, args ...any) {
	c.AbortWithStatusJSON(status, gin.H{"error": i18n.T(lang(c), format, args...)})
}

// lang 从 Accept-Language 解析请求语言。
func lang(c *gin.Context) i18n.Lang {
	return i18n.Parse(c.GetHeader("Accept-Language"))
}

func pagination(c *gin.Context, def, max int) (int, int) {
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}
	size, err := strconv.Atoi(c.DefaultQuery("size", strconv.Itoa(def)))
	if err != nil || size < 1 {
		size = def
	}
	if size > max {
		size = max
	}
	return page, size
}
