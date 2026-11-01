package query

import (
	"fmt"
	"github.com/expvintl/scopex/internal/i18n"
	"strings"
)

// 查询语言：
//   app:nginx port:80 country:CN "thinkphp"
//   port:[80 TO 8080] && server:nginx || title:"登录"
//   ip:192.168.1.0/24 -title:403
// 所有字符串匹配都按字面量处理（大小写不敏感），不接受用户正则。

type Kind int

const (
	TokEOF Kind = iota
	TokIdent
	TokString
	TokLParen
	TokRParen
	TokLBrack
	TokRBrack
	TokColon
	TokEq
	TokNeq
	TokGt
	TokLt
	TokGte
	TokLte
	TokTilde
	TokAnd
	TokOr
	TokNot
)

type Token struct {
	Kind Kind
	Val  string
	Pos  int
}

func (t Token) String() string {
	switch t.Kind {
	case TokEOF:
		return "EOF"
	case TokString:
		return fmt.Sprintf("%q", t.Val)
	case TokAnd:
		return "AND"
	case TokOr:
		return "OR"
	case TokNot:
		return "NOT"
	}
	return t.Val
}

var singleTokens = map[byte]Kind{
	'(': TokLParen, ')': TokRParen, '[': TokLBrack, ']': TokRBrack,
	':': TokColon, '=': TokEq, '>': TokGt, '<': TokLt, '~': TokTilde,
}

func Lex(input string) ([]Token, error) {
	var toks []Token
	i := 0
	for i < len(input) {
		c := input[i]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			i++
			continue
		}
		pos := i
		switch {
		case c == '"' || c == '\'':
			quote := c
			i++
			var sb strings.Builder
			for i < len(input) && input[i] != quote {
				if input[i] == '\\' && i+1 < len(input) {
					i++
				}
				sb.WriteByte(input[i])
				i++
			}
			if i >= len(input) {
				return nil, i18n.Errorf("第 %d 个字符处引号未闭合", pos+1)
			}
			i++
			toks = append(toks, Token{Kind: TokString, Val: sb.String(), Pos: pos})
		case c == '!' && i+1 < len(input) && input[i+1] == '=':
			toks = append(toks, Token{Kind: TokNeq, Val: "!=", Pos: pos})
			i += 2
		case c == '>' && i+1 < len(input) && input[i+1] == '=':
			toks = append(toks, Token{Kind: TokGte, Val: ">=", Pos: pos})
			i += 2
		case c == '<' && i+1 < len(input) && input[i+1] == '=':
			toks = append(toks, Token{Kind: TokLte, Val: "<=", Pos: pos})
			i += 2
		case c == '&' && i+1 < len(input) && input[i+1] == '&':
			toks = append(toks, Token{Kind: TokAnd, Val: "&&", Pos: pos})
			i += 2
		case c == '|' && i+1 < len(input) && input[i+1] == '|':
			toks = append(toks, Token{Kind: TokOr, Val: "||", Pos: pos})
			i += 2
		case c == '!' || c == '-':
			toks = append(toks, Token{Kind: TokNot, Val: string(c), Pos: pos})
			i++
		default:
			if k, ok := singleTokens[c]; ok {
				toks = append(toks, Token{Kind: k, Val: string(c), Pos: pos})
				i++
				continue
			}
			start := i
			for i < len(input) {
				ch := input[i]
				if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
					break
				}
				if _, ok := singleTokens[ch]; ok || ch == '"' || ch == '\'' || ch == '!' || ch == '&' || ch == '|' {
					break
				}
				i++
			}
			word := input[start:i]
			if word == "" {
				return nil, i18n.Errorf("第 %d 个字符处无法识别", pos+1)
			}
			switch strings.ToUpper(word) {
			case "AND":
				toks = append(toks, Token{Kind: TokAnd, Val: word, Pos: pos})
			case "OR":
				toks = append(toks, Token{Kind: TokOr, Val: word, Pos: pos})
			case "NOT":
				toks = append(toks, Token{Kind: TokNot, Val: word, Pos: pos})
			default:
				toks = append(toks, Token{Kind: TokIdent, Val: word, Pos: pos})
			}
		}
	}
	toks = append(toks, Token{Kind: TokEOF, Pos: len(input)})
	return toks, nil
}

type Node interface{ node() }

type (
	AndNode   struct{ Left, Right Node }
	OrNode    struct{ Left, Right Node }
	NotNode   struct{ Child Node }
	TextNode  struct{ Value string }
	FieldNode struct {
		Field string
		Op    string // : = != > < >= <=
		Value string
		Range *RangeVal
	}
)

type RangeVal struct {
	From, To string
	HasFrom  bool
	HasTo    bool
}

func (AndNode) node()   {}
func (OrNode) node()    {}
func (NotNode) node()   {}
func (TextNode) node()  {}
func (FieldNode) node() {}

type parser struct {
	toks []Token
	pos  int
}

func Parse(input string) (Node, error) {
	toks, err := Lex(input)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks}
	if p.peek().Kind == TokEOF {
		return nil, nil
	}
	n, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	if p.peek().Kind != TokEOF {
		return nil, i18n.Errorf("查询语句在 %q 处存在多余内容", p.peek().Val)
	}
	return n, nil
}

func (p *parser) peek() Token { return p.toks[p.pos] }

func (p *parser) next() Token {
	t := p.toks[p.pos]
	if p.pos < len(p.toks)-1 {
		p.pos++
	}
	return t
}

func (p *parser) parseOr() (Node, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.peek().Kind == TokOr {
		p.next()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = OrNode{Left: left, Right: right}
	}
	return left, nil
}

func (p *parser) parseAnd() (Node, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for {
		k := p.peek().Kind
		if k == TokAnd {
			p.next()
			right, err := p.parseUnary()
			if err != nil {
				return nil, err
			}
			left = AndNode{Left: left, Right: right}
			continue
		}
		// 隐式 AND：相邻的表达式
		if k == TokIdent || k == TokString || k == TokLParen || k == TokNot {
			right, err := p.parseUnary()
			if err != nil {
				return nil, err
			}
			left = AndNode{Left: left, Right: right}
			continue
		}
		return left, nil
	}
}

func (p *parser) parseUnary() (Node, error) {
	if p.peek().Kind == TokNot {
		p.next()
		child, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return NotNode{Child: child}, nil
	}
	return p.parsePrimary()
}

func (p *parser) parsePrimary() (Node, error) {
	t := p.peek()
	switch t.Kind {
	case TokLParen:
		p.next()
		n, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		if p.peek().Kind != TokRParen {
			return nil, i18n.Errorf("缺少右括号")
		}
		p.next()
		return n, nil
	case TokString:
		p.next()
		return TextNode{Value: t.Val}, nil
	case TokIdent:
		p.next()
		if isOperator(p.peek().Kind) {
			opTok := p.next()
			val, rng, err := p.parseValue()
			if err != nil {
				return nil, err
			}
			return FieldNode{Field: strings.ToLower(t.Val), Op: opTok.Val, Value: val, Range: rng}, nil
		}
		return TextNode{Value: t.Val}, nil
	}
	return nil, i18n.Errorf("无法解析 %q", t.String())
}

func isOperator(k Kind) bool {
	switch k {
	case TokColon, TokEq, TokNeq, TokGt, TokLt, TokGte, TokLte, TokTilde:
		return true
	}
	return false
}

func (p *parser) parseValue() (string, *RangeVal, error) {
	t := p.peek()
	switch t.Kind {
	case TokString, TokIdent:
		p.next()
		return t.Val, nil, nil
	case TokLBrack:
		p.next()
		from, err := p.rangeItem()
		if err != nil {
			return "", nil, err
		}
		if !strings.EqualFold(p.peek().Val, "TO") {
			return "", nil, i18n.Errorf("范围查询需要 TO 关键字，例如 port:[80 TO 8080]")
		}
		p.next()
		to, err := p.rangeItem()
		if err != nil {
			return "", nil, err
		}
		if p.peek().Kind != TokRBrack {
			return "", nil, i18n.Errorf("范围查询缺少 ]")
		}
		p.next()
		r := &RangeVal{From: from, To: to, HasFrom: from != "*", HasTo: to != "*"}
		return "", r, nil
	}
	return "", nil, i18n.Errorf("运算符后缺少值")
}

func (p *parser) rangeItem() (string, error) {
	t := p.peek()
	if t.Kind == TokIdent || t.Kind == TokString {
		p.next()
		return t.Val, nil
	}
	return "", i18n.Errorf("范围查询的边界不合法")
}
