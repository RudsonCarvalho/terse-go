package terse

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Serialize converts a Go value into TERSE format.
func Serialize(v any) (string, error) {
	s := &serializer{}
	return s.document(v)
}

// Parse converts a TERSE-formatted string into a Go value.
func Parse(src string) (any, error) {
	p := newParser(src)
	return p.parseDocument()
}

// ---------------------------------------------------------------------------
// Serializer
// ---------------------------------------------------------------------------

type serializer struct{}

func (s *serializer) document(v any) (string, error) {
	return s.value(v, 0)
}

func (s *serializer) value(v any, indent int) (string, error) {
	if v == nil {
		return "~", nil
	}
	switch t := v.(type) {
	case bool:
		if t {
			return "T", nil
		}
		return "F", nil
	case float64:
		return formatFloat(t), nil
	case int:
		return strconv.Itoa(t), nil
	case int64:
		return strconv.FormatInt(t, 10), nil
	case uint64:
		return strconv.FormatUint(t, 10), nil
	case string:
		return s.str(t), nil
	case map[string]any:
		return s.object(t, indent)
	case []any:
		return s.slice(t, indent)
	default:
		return "", fmt.Errorf("terse: unsupported type %T", v)
	}
}

func formatFloat(f float64) string {
	if math.IsInf(f, 1) {
		return "Inf"
	}
	if math.IsInf(f, -1) {
		return "-Inf"
	}
	if math.IsNaN(f) {
		return "NaN"
	}
	if f == math.Trunc(f) && math.Abs(f) < 1e15 {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}

func (s *serializer) key(k string) string {
	if isSafeId(k) {
		return k
	}
	return strconv.Quote(k)
}

func (s *serializer) str(v string) string {
	if isSafeId(v) {
		return v
	}
	return strconv.Quote(v)
}

func (s *serializer) object(m map[string]any, indent int) (string, error) {
	if len(m) == 0 {
		return "{}", nil
	}
	keys := sortedKeys(m)
	inline, err := s.tryInlineObject(m, keys)
	if err != nil {
		return "", err
	}
	if inline != "" {
		return inline, nil
	}
	return s.blockObject(m, keys, indent)
}

// inline-obj = "{" *( key ":" value SP ) "}"
func (s *serializer) tryInlineObject(m map[string]any, keys []string) (string, error) {
	var sb strings.Builder
	sb.WriteString("{")
	for _, k := range keys {
		vStr, err := s.value(m[k], 0)
		if err != nil {
			return "", err
		}
		sb.WriteString(s.key(k))
		sb.WriteString(":")
		sb.WriteString(vStr)
		sb.WriteString(" ")
	}
	sb.WriteString("}")
	result := sb.String()
	if len(result) <= 80 {
		return result, nil
	}
	return "", nil
}

func (s *serializer) blockObject(m map[string]any, keys []string, indent int) (string, error) {
	var lines []string
	for _, k := range keys {
		vStr, err := s.value(m[k], indent+2)
		if err != nil {
			return "", err
		}
		if strings.Contains(vStr, "\n") {
			lines = append(lines, s.key(k)+":")
			lines = append(lines, indentBlock(vStr, 2))
		} else {
			lines = append(lines, s.key(k)+":"+vStr)
		}
	}
	return strings.Join(lines, "\n"), nil
}

func (s *serializer) slice(arr []any, indent int) (string, error) {
	if len(arr) == 0 {
		return "[]", nil
	}
	sk := schemaKeys(arr)
	if sk != nil {
		return s.schemaArray(arr, sk, indent)
	}
	inline, err := s.tryInlineArray(arr)
	if err != nil {
		return "", err
	}
	if inline != "" {
		return inline, nil
	}
	return s.blockArray(arr, indent)
}

func schemaKeys(arr []any) []string {
	if len(arr) < 2 {
		return nil
	}
	var keys []string
	for i, elem := range arr {
		m, ok := elem.(map[string]any)
		if !ok {
			return nil
		}
		if i == 0 {
			keys = sortedKeys(m)
		} else {
			ek := sortedKeys(m)
			if len(ek) != len(keys) {
				return nil
			}
			for j, k := range keys {
				if ek[j] != k {
					return nil
				}
			}
		}
		for _, v := range m {
			switch v.(type) {
			case nil, bool, float64, int, int64, uint64, string:
			default:
				return nil
			}
		}
	}
	return keys
}

// schema-arr = "#[" *( key SP ) "]" NEWLINE *( *SP *( value SP ) NEWLINE )
func (s *serializer) schemaArray(arr []any, keys []string, indent int) (string, error) {
	var sb strings.Builder
	sb.WriteString("#[")
	for _, k := range keys {
		sb.WriteString(k)
		sb.WriteString(" ")
	}
	sb.WriteString("]\n")
	for _, elem := range arr {
		m := elem.(map[string]any)
		for _, k := range keys {
			vStr, err := s.value(m[k], 0)
			if err != nil {
				return "", err
			}
			sb.WriteString(vStr)
			sb.WriteString(" ")
		}
		sb.WriteString("\n")
	}
	result := sb.String()
	return strings.TrimRight(result, "\n"), nil
}

// inline-arr = "[" *( value SP ) "]"
func (s *serializer) tryInlineArray(arr []any) (string, error) {
	var sb strings.Builder
	sb.WriteString("[")
	for _, elem := range arr {
		vStr, err := s.value(elem, 0)
		if err != nil {
			return "", err
		}
		sb.WriteString(vStr)
		sb.WriteString(" ")
	}
	sb.WriteString("]")
	result := sb.String()
	if len(result) <= 80 {
		return result, nil
	}
	return "", nil
}

// block-arr = "[" NEWLINE 1*( indent value NEWLINE ) "]"
// Each element must be a single line — force inline for objects/arrays.
func (s *serializer) blockArray(arr []any, indent int) (string, error) {
	var lines []string
	lines = append(lines, "[")
	for _, elem := range arr {
		vStr, err := s.valueInline(elem)
		if err != nil {
			return "", err
		}
		lines = append(lines, vStr)
	}
	lines = append(lines, "]")
	return strings.Join(lines, "\n"), nil
}

// valueInline always produces a single-line representation (no block form).
func (s *serializer) valueInline(v any) (string, error) {
	if m, ok := v.(map[string]any); ok {
		if len(m) == 0 {
			return "{}", nil
		}
		keys := sortedKeys(m)
		var sb strings.Builder
		sb.WriteString("{")
		for _, k := range keys {
			vStr, err := s.valueInline(m[k])
			if err != nil {
				return "", err
			}
			sb.WriteString(s.key(k))
			sb.WriteString(":")
			sb.WriteString(vStr)
			sb.WriteString(" ")
		}
		sb.WriteString("}")
		return sb.String(), nil
	}
	if arr, ok := v.([]any); ok {
		if len(arr) == 0 {
			return "[]", nil
		}
		var sb strings.Builder
		sb.WriteString("[")
		for _, elem := range arr {
			vStr, err := s.valueInline(elem)
			if err != nil {
				return "", err
			}
			sb.WriteString(vStr)
			sb.WriteString(" ")
		}
		sb.WriteString("]")
		return sb.String(), nil
	}
	return s.value(v, 0)
}

func isSafeId(s string) bool {
	if s == "" {
		return false
	}
	switch s {
	case "~", "T", "F", "true", "false", "null", "Inf", "-Inf", "NaN":
		return false
	}
	r, _ := utf8.DecodeRuneInString(s)
	if !isSafeStart(r) {
		return false
	}
	for _, r2 := range s[len(string(r)):] {
		if !isSafeChar(r2) {
			return false
		}
	}
	if _, err := strconv.ParseFloat(s, 64); err == nil {
		return false
	}
	return true
}

func isSafeStart(r rune) bool {
	return unicode.IsLetter(r) || r == '_'
}

func isSafeChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' || r == '.'
}

func quoteStr(v string) string {
	var sb strings.Builder
	sb.WriteByte('"')
	for _, r := range v {
		switch r {
		case '"':
			sb.WriteString("\\\"")
		case '\\':
			sb.WriteString("\\\\")
		case '\n':
			sb.WriteString("\\n")
		case '\r':
			sb.WriteString("\\r")
		case '\t':
			sb.WriteString("\\t")
		default:
			if r < 0x20 {
				sb.WriteString(fmt.Sprintf("\\u%04x", r))
			} else {
				sb.WriteRune(r)
			}
		}
	}
	sb.WriteByte('"')
	return sb.String()
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func indentBlock(s string, n int) string {
	pad := strings.Repeat(" ", n)
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if l != "" {
			lines[i] = pad + l
		}
	}
	return strings.Join(lines, "\n")
}

// ---------------------------------------------------------------------------
// Parser
// ---------------------------------------------------------------------------

type parser struct {
	src []rune
	pos int
}

func newParser(src string) *parser {
	return &parser{src: []rune(src)}
}

func (p *parser) eof() bool { return p.pos >= len(p.src) }

func (p *parser) ch() rune {
	if p.eof() {
		return 0
	}
	return p.src[p.pos]
}

func (p *parser) skipSpaces() {
	for !p.eof() && p.ch() == ' ' {
		p.pos++
	}
}

func (p *parser) skipToEOL() {
	for !p.eof() && p.ch() != '\n' {
		p.pos++
	}
}

func (p *parser) isBlankOrComment() bool {
	save := p.pos
	p.skipSpaces()
	if p.eof() || p.ch() == '\n' {
		p.pos = save
		return true
	}
	if p.ch() == '#' && (p.pos+1 >= len(p.src) || p.src[p.pos+1] != '[') {
		p.pos = save
		return true
	}
	p.pos = save
	return false
}

func (p *parser) skipBlankAndComments() {
	for !p.eof() {
		if p.isBlankOrComment() {
			p.skipToEOL()
			if !p.eof() {
				p.pos++
			}
		} else {
			break
		}
	}
}

func (p *parser) peekIndent() int {
	save := p.pos
	count := 0
	for !p.eof() && p.ch() == ' ' {
		count++
		p.pos++
	}
	p.pos = save
	return count
}

func (p *parser) consumeIndent(n int) bool {
	save := p.pos
	for i := 0; i < n; i++ {
		if p.eof() || p.ch() != ' ' {
			p.pos = save
			return false
		}
		p.pos++
	}
	return true
}

func (p *parser) lineIsKV(indent int) bool {
	save := p.pos
	defer func() { p.pos = save }()
	if !p.consumeIndent(indent) {
		return false
	}
	if p.eof() {
		return false
	}
	if p.ch() == '"' {
		p.pos++
		for !p.eof() && p.ch() != '"' && p.ch() != '\n' {
			if p.ch() == '\\' {
				p.pos++
			}
			p.pos++
		}
		if p.eof() || p.ch() != '"' {
			return false
		}
		p.pos++
	} else {
		if !isSafeStart(p.ch()) {
			return false
		}
		for !p.eof() && isSafeChar(p.ch()) {
			p.pos++
		}
	}
	p.skipSpaces()
	return !p.eof() && p.ch() == ':'
}

func (p *parser) parseDocument() (any, error) {
	p.skipBlankAndComments()
	if p.eof() {
		return nil, nil
	}
	if p.lineIsKV(0) {
		return p.parseKVBlock(0)
	}
	return p.parseValueAtIndent(0)
}

func (p *parser) parseValueAtIndent(indent int) (any, error) {
	p.skipSpaces()
	if p.eof() {
		return nil, nil
	}
	c := p.ch()
	switch {
	case c == '{':
		return p.parseObject(indent)
	case c == '[':
		if p.pos+1 < len(p.src) && p.src[p.pos+1] == ']' {
			p.pos += 2
			return []any{}, nil
		}
		return p.parseArray(indent)
	case c == '#' && p.pos+1 < len(p.src) && p.src[p.pos+1] == '[':
		return p.parseSchemaArray(indent)
	default:
		v, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		p.skipToEOL()
		if !p.eof() {
			p.pos++
		}
		p.skipBlankAndComments()
		return v, nil
	}
}

func (p *parser) parseKVBlock(indent int) (any, error) {
	m := map[string]any{}
	for {
		p.skipBlankAndComments()
		if p.eof() {
			break
		}
		if p.peekIndent() != indent {
			break
		}
		if !p.lineIsKV(indent) {
			break
		}
		p.consumeIndent(indent)
		k, err := p.parseKey()
		if err != nil {
			return nil, err
		}
		p.skipSpaces()
		if p.eof() || p.ch() != ':' {
			return nil, fmt.Errorf("terse: expected ':' after key %q", k)
		}
		p.pos++ // consume ':'
		p.skipSpaces()
		if p.eof() || p.ch() == '\n' {
			// block value on next lines
			if !p.eof() {
				p.pos++
			}
			p.skipBlankAndComments()
			childIndent := p.peekIndent()
			if childIndent <= indent {
				m[k] = nil
				continue
			}
			// FIX: if child lines are KV pairs, parse as nested block object
			if p.lineIsKV(childIndent) {
				child, err := p.parseKVBlock(childIndent)
				if err != nil {
					return nil, err
				}
				m[k] = child
			} else {
				p.consumeIndent(childIndent)
				child, err := p.parseValueAtIndent(childIndent)
				if err != nil {
					return nil, err
				}
				m[k] = child
			}
		} else {
			// inline value
			val, err := p.parseValue()
			if err != nil {
				return nil, err
			}
			p.skipToEOL()
			if !p.eof() {
				p.pos++
			}
			m[k] = val
		}
	}
	return m, nil
}

func (p *parser) parseValue() (any, error) {
	p.skipSpaces()
	if p.eof() {
		return nil, fmt.Errorf("terse: unexpected EOF")
	}
	c := p.ch()
	switch {
	case c == '"':
		return p.parseQuotedString()
	case c == '{':
		return p.parseObject(0)
	case c == '[':
		if p.pos+1 < len(p.src) && p.src[p.pos+1] == ']' {
			p.pos += 2
			return []any{}, nil
		}
		return p.parseArray(0)
	default:
		return p.parsePrimitive()
	}
}

func (p *parser) parsePrimitive() (any, error) {
	start := p.pos
	for !p.eof() && p.ch() != ' ' && p.ch() != '\n' && p.ch() != ':' && p.ch() != '}' && p.ch() != ']' {
		p.pos++
	}
	tok := string(p.src[start:p.pos])
	if tok == "" {
		return nil, fmt.Errorf("terse: empty token at pos %d", start)
	}
	switch tok {
	case "~":
		return nil, nil
	case "T":
		return true, nil
	case "F":
		return false, nil
	case "Inf":
		return math.Inf(1), nil
	case "-Inf":
		return math.Inf(-1), nil
	case "NaN":
		return math.NaN(), nil
	}
	if f, err := strconv.ParseFloat(tok, 64); err == nil {
		return f, nil
	}
	return tok, nil
}

func (p *parser) parseQuotedString() (string, error) {
	p.pos++
	var sb strings.Builder
	for {
		if p.eof() || p.ch() == '\n' {
			return "", fmt.Errorf("terse: unterminated string")
		}
		c := p.ch()
		if c == '"' {
			p.pos++
			break
		}
		if c != '\\' {
			sb.WriteRune(c)
			p.pos++
			continue
		}
		p.pos++
		if p.eof() {
			return "", fmt.Errorf("terse: unterminated escape")
		}
		esc := p.ch()
		p.pos++
		switch esc {
		case '"':
			sb.WriteByte('"')
		case '\\':
			sb.WriteByte('\\')
		case 'n':
			sb.WriteByte('\n')
		case 'r':
			sb.WriteByte('\r')
		case 't':
			sb.WriteByte('\t')
		case 'u':
			if p.pos+4 > len(p.src) {
				return "", fmt.Errorf("terse: short \\u escape")
			}
			hex := string(p.src[p.pos : p.pos+4])
			p.pos += 4
			n, err := strconv.ParseInt(hex, 16, 32)
			if err != nil {
				return "", fmt.Errorf("terse: invalid \\u escape: %s", hex)
			}
			sb.WriteRune(rune(n))
		default:
			sb.WriteByte('\\')
			sb.WriteRune(esc)
		}
	}
	return sb.String(), nil
}

func (p *parser) parseKey() (string, error) {
	if p.eof() {
		return "", fmt.Errorf("terse: expected key")
	}
	if p.ch() == '"' {
		return p.parseQuotedString()
	}
	return p.parseBare()
}

func (p *parser) parseBare() (string, error) {
	start := p.pos
	for !p.eof() && isSafeChar(p.ch()) {
		p.pos++
	}
	if p.pos == start {
		return "", fmt.Errorf("terse: expected bare identifier at pos %d", start)
	}
	return string(p.src[start:p.pos]), nil
}

// FIX: block-obj content is at same indent as '{', not indent+2.
// After parsing, consume closing '}'.
func (p *parser) parseObject(indent int) (any, error) {
	p.pos++ // consume '{'
	p.skipSpaces()
	if p.eof() || p.ch() == '\n' {
		// block object: content is at indent (same level as '{')
		if !p.eof() {
			p.pos++
		}
		m, err := p.parseKVBlock(indent)
		if err != nil {
			return nil, err
		}
		// consume closing '}'
		p.skipBlankAndComments()
		save := p.pos
		p.skipSpaces()
		if !p.eof() && p.ch() == '}' {
			p.pos++
			p.skipToEOL()
			if !p.eof() {
				p.pos++
			}
		} else {
			p.pos = save
		}
		return m, nil
	}
	// inline object
	m := map[string]any{}
	for {
		p.skipSpaces()
		if p.eof() {
			return nil, fmt.Errorf("terse: unterminated inline object")
		}
		if p.ch() == '}' {
			p.pos++
			break
		}
		k, err := p.parseKey()
		if err != nil {
			return nil, err
		}
		p.skipSpaces()
		if p.eof() || p.ch() != ':' {
			return nil, fmt.Errorf("terse: expected ':' after key %q", k)
		}
		p.pos++
		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		m[k] = val
	}
	return m, nil
}

// FIX: block-arr stops on ']', not indent-level.
func (p *parser) parseArray(indent int) (any, error) {
	p.pos++ // consume '['
	p.skipSpaces()
	if p.eof() || p.ch() == '\n' {
		// block array
		if !p.eof() {
			p.pos++
		}
		var arr []any
		for {
			p.skipBlankAndComments()
			if p.eof() {
				break
			}
			// check for closing ']'
			save := p.pos
			p.skipSpaces()
			if !p.eof() && p.ch() == ']' {
				p.pos++
				p.skipToEOL()
				if !p.eof() {
					p.pos++
				}
				break
			}
			p.pos = save

			lineIndent := p.peekIndent()
			p.consumeIndent(lineIndent)

			// nested KV block as array element
			if p.lineIsKV(0) {
				val, err := p.parseKVBlock(0)
				if err != nil {
					return nil, err
				}
				arr = append(arr, val)
				continue
			}

			val, err := p.parseValue()
			if err != nil {
				return nil, err
			}
			p.skipToEOL()
			if !p.eof() {
				p.pos++
			}
			arr = append(arr, val)
		}
		return arr, nil
	}
	// inline array
	var arr []any
	for {
		p.skipSpaces()
		if p.eof() {
			return nil, fmt.Errorf("terse: unterminated inline array")
		}
		if p.ch() == ']' {
			p.pos++
			break
		}
		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		arr = append(arr, val)
	}
	return arr, nil
}

func (p *parser) parseSchemaArray(headerIndent int) (any, error) {
	p.pos += 2 // consume '#['
	var keys []string
	for {
		p.skipSpaces()
		if p.eof() || p.ch() == ']' {
			break
		}
		k, err := p.parseBare()
		if err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	if p.eof() || p.ch() != ']' {
		return nil, fmt.Errorf("terse: unterminated schema array header")
	}
	p.pos++
	p.skipToEOL()
	if !p.eof() {
		p.pos++
	}
	var arr []any
	for {
		p.skipBlankAndComments()
		if p.eof() {
			break
		}
		lineIndent := p.peekIndent()
		if lineIndent < headerIndent {
			break
		}
		if p.lineIsKV(lineIndent) {
			break
		}
		p.consumeIndent(lineIndent)
		m := map[string]any{}
		for _, k := range keys {
			p.skipSpaces()
			val, err := p.parseValue()
			if err != nil {
				return nil, err
			}
			m[k] = val
		}
		p.skipToEOL()
		if !p.eof() {
			p.pos++
		}
		arr = append(arr, m)
	}
	return arr, nil
}
