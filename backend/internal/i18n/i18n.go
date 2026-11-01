package i18n

import (
	"fmt"
	"strings"
)

// Lang 表示输出语言。
type Lang string

const (
	ZH Lang = "zh"
	EN Lang = "en"
)

// Parse 依据 Accept-Language 头选择语言，默认中文。
func Parse(header string) Lang {
	for _, part := range strings.Split(header, ",") {
		tag := strings.ToLower(strings.TrimSpace(strings.SplitN(part, ";", 2)[0]))
		switch {
		case strings.HasPrefix(tag, "zh"):
			return ZH
		case strings.HasPrefix(tag, "en"):
			return EN
		}
	}
	return ZH
}

// Translate 把中文原文翻译为目标语言，未收录时返回原文。
func Translate(lang Lang, text string) string {
	if lang == EN {
		if v, ok := catalog[text]; ok {
			return v
		}
	}
	return text
}

// T 先翻译格式串再填充参数。
func T(lang Lang, format string, args ...any) string {
	msg := Translate(lang, format)
	if len(args) == 0 {
		return msg
	}
	return fmt.Sprintf(msg, args...)
}

// Error 保留格式串与参数，使错误可以按目标语言渲染。
type Error struct {
	Format string
	Args   []any
}

func (e *Error) Error() string { return fmt.Sprintf(e.Format, e.Args...) }

// Errorf 构造可翻译的错误。
func Errorf(format string, args ...any) *Error { return &Error{Format: format, Args: args} }

// Message 把错误渲染为指定语言。
func Message(lang Lang, err error) string {
	if err == nil {
		return ""
	}
	if e, ok := err.(*Error); ok {
		return T(lang, e.Format, e.Args...)
	}
	return err.Error()
}
