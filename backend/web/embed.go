package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var files embed.FS

type FS = fs.FS

// Dist 返回前端构建产物；dist 不存在时返回错误。
func Dist() (fs.FS, error) {
	return fs.Sub(files, "dist")
}
