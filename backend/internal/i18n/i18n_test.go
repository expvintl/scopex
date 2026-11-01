package i18n

import "testing"

func TestParse(t *testing.T) {
	cases := map[string]Lang{
		"":                               ZH,
		"zh-CN,zh;q=0.9,en;q=0.8":        ZH,
		"en-US,en;q=0.9":                 EN,
		"fr-FR,fr;q=0.9":                 ZH,
		"EN":                             EN,
		"zh-Hant-TW, zh;q=0.9, en;q=0.8": ZH,
	}
	for header, want := range cases {
		if got := Parse(header); got != want {
			t.Errorf("Parse(%q) = %q, want %q", header, got, want)
		}
	}
}

func TestTranslateWithArgs(t *testing.T) {
	got := T(EN, "查询语法错误: %s", "unexpected token")
	if want := "Query syntax error: unexpected token"; got != want {
		t.Errorf("T(EN) = %q, want %q", got, want)
	}
	if got := T(ZH, "查询语法错误: %s", "unexpected token"); got != "查询语法错误: unexpected token" {
		t.Errorf("T(ZH) = %q", got)
	}
}

func TestTranslateFallback(t *testing.T) {
	const unknown = "这条消息还没有翻译"
	if got := T(EN, unknown); got != unknown {
		t.Errorf("T(EN) fallback = %q, want %q", got, unknown)
	}
}

func TestErrorMessage(t *testing.T) {
	err := Errorf("第 %d 个字符处无法识别", 7)
	if got, want := err.Error(), "第 7 个字符处无法识别"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
	if got, want := Message(EN, err), "Unrecognized input at character 7"; got != want {
		t.Errorf("Message(EN) = %q, want %q", got, want)
	}
	if got := Message(ZH, nil); got != "" {
		t.Errorf("Message(nil) = %q", got)
	}
}
