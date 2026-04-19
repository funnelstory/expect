package pgformat

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

type snapTokKind uint8

const (
	snapTokWord snapTokKind = iota
	snapTokString
	snapTokIdent
	snapTokLineComment
	snapTokBlockComment
	snapTokDollar
	snapTokNumber
	snapTokOp
	snapTokLParen
	snapTokRParen
	snapTokComma
	snapTokSemicolon
	snapTokLBracket
	snapTokRBracket
)

type snapToken struct {
	kind snapTokKind
	lit  string
}

// tokenize splits SQL into tokens for the layout engine. Whitespace is not emitted.
func tokenize(sql string) []snapToken {
	sql = strings.TrimSpace(sql)
	if sql == "" {
		return nil
	}
	var out []snapToken
	r := sql
	for len(r) > 0 {
		c, w := utf8.DecodeRuneInString(r)
		switch {
		case unicode.IsSpace(c):
			r = r[w:]
			continue
		case c == '(':
			out = append(out, snapToken{kind: snapTokLParen, lit: "("})
			r = r[w:]
		case c == ')':
			out = append(out, snapToken{kind: snapTokRParen, lit: ")"})
			r = r[w:]
		case c == ',':
			out = append(out, snapToken{kind: snapTokComma, lit: ","})
			r = r[w:]
		case c == '[':
			out = append(out, snapToken{kind: snapTokLBracket, lit: "["})
			r = r[w:]
		case c == ']':
			out = append(out, snapToken{kind: snapTokRBracket, lit: "]"})
			r = r[w:]
		case c == ';':
			out = append(out, snapToken{kind: snapTokSemicolon, lit: ";"})
			r = r[w:]
		case c == '-' && len(r) > w && r[w] == '-':
			end := strings.IndexByte(r[w+2:], '\n')
			var lit string
			if end < 0 {
				lit = r
				r = ""
			} else {
				lit = r[:w+2+end]
				r = r[w+2+end:]
			}
			out = append(out, snapToken{kind: snapTokLineComment, lit: lit})
		case c == '/' && len(r) > w && r[w] == '*':
			closeIdx := strings.Index(r[2:], "*/")
			if closeIdx < 0 {
				out = append(out, snapToken{kind: snapTokBlockComment, lit: r})
				r = ""
			} else {
				end := 2 + closeIdx + 2
				out = append(out, snapToken{kind: snapTokBlockComment, lit: r[:end]})
				r = r[end:]
			}
		case c == '\'':
			lit, rest := scanSnapSingleQuoted(r)
			out = append(out, snapToken{kind: snapTokString, lit: lit})
			r = rest
		case c == '"':
			lit, rest := scanSnapDoubleQuoted(r)
			out = append(out, snapToken{kind: snapTokIdent, lit: lit})
			r = rest
		case c == '$' && len(r) > 1:
			lit, rest, ok := scanSnapDollarQuoted(r)
			if ok {
				out = append(out, snapToken{kind: snapTokDollar, lit: lit})
				r = rest
				break
			}
			fallthrough
		case unicode.IsDigit(c) || (c == '.' && len(r) > w && unicode.IsDigit(rune(r[w]))):
			lit, rest := scanSnapNumber(r)
			out = append(out, snapToken{kind: snapTokNumber, lit: lit})
			r = rest
		case isSnapOpStart(c):
			lit, rest := scanSnapOp(r)
			out = append(out, snapToken{kind: snapTokOp, lit: lit})
			r = rest
		default:
			lit, rest := scanSnapWord(r)
			out = append(out, snapToken{kind: snapTokWord, lit: lit})
			r = rest
		}
	}
	return out
}

func scanSnapSingleQuoted(s string) (lit string, rest string) {
	var b strings.Builder
	b.WriteByte('\'')
	i := 1
	for i < len(s) {
		c := s[i]
		b.WriteByte(c)
		if c == '\'' {
			if i+1 < len(s) && s[i+1] == '\'' {
				b.WriteByte('\'')
				i += 2
				continue
			}
			return b.String(), s[i+1:]
		}
		i++
	}
	return b.String(), ""
}

func scanSnapDoubleQuoted(s string) (lit string, rest string) {
	var b strings.Builder
	b.WriteByte('"')
	i := 1
	for i < len(s) {
		c := s[i]
		b.WriteByte(c)
		if c == '"' {
			if i+1 < len(s) && s[i+1] == '"' {
				b.WriteByte('"')
				i += 2
				continue
			}
			return b.String(), s[i+1:]
		}
		i++
	}
	return b.String(), ""
}

func scanSnapDollarQuoted(s string) (lit string, rest string, ok bool) {
	if len(s) < 2 || s[0] != '$' {
		return "", s, false
	}
	j := 1
	for j < len(s) && s[j] != '$' {
		if !isSnapDollarTagRune(rune(s[j])) {
			return "", s, false
		}
		j++
	}
	if j >= len(s) || s[j] != '$' {
		return "", s, false
	}
	tag := s[:j+1]
	rest = s[j+1:]
	idx := strings.Index(rest, tag)
	if idx < 0 {
		return "", s, false
	}
	body := rest[:idx+len(tag)]
	return tag + body, rest[idx+len(tag):], true
}

func isSnapDollarTagRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

func scanSnapNumber(s string) (lit string, rest string) {
	i := 0
	for i < len(s) && (unicode.IsDigit(rune(s[i])) || s[i] == '.' || s[i] == 'e' || s[i] == 'E' || s[i] == '+' || s[i] == '-') {
		i++
	}
	return s[:i], s[i:]
}

func isSnapOpStart(c rune) bool {
	switch c {
	case '=', '<', '>', '!', '|', '&', '+', '-', '*', '/', '%', '^', '~', ':', '@', '?':
		return true
	default:
		return false
	}
}

func scanSnapOp(s string) (lit string, rest string) {
	if len(s) >= 3 && s[:3] == "!~*" {
		return s[:3], s[3:]
	}
	two := []string{
		"->>", "@>", "<@", "?|", "?&", "&&", "||", "::", "->",
		"<=", ">=", "<>", "!=", ":=", "~*", "!~", "<<", ">>",
	}
	if len(s) >= 2 {
		p := s[:2]
		for _, x := range two {
			if p == x {
				return p, s[2:]
			}
		}
	}
	return s[:1], s[1:]
}

func scanSnapWord(s string) (lit string, rest string) {
	i := 0
	for i < len(s) {
		c, w := utf8.DecodeRuneInString(s[i:])
		if unicode.IsLetter(c) || unicode.IsDigit(c) || c == '_' || c == '$' {
			i += w
			continue
		}
		break
	}
	if i == 0 {
		_, w := utf8.DecodeRuneInString(s)
		return s[:w], s[w:]
	}
	return s[:i], s[i:]
}
