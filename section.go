package syntax

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

func (p *parser) parseSections(start int) *ParseError {
	s := p.source
	limit := contentEnd(s, len(s))
	if start >= limit {
		return nil
	}
	end, next := lineEnd(s, start)
	if end != start {
		if !p.recover {
			return &ParseError{InvalidSectionSeparator, start}
		}
		p.diagnostics = append(p.diagnostics, ParseError{InvalidSectionSeparator, start})
	} else {
		start = next
	}
	for start < limit {
		end, next = lineEnd(s, start)
		if end != start {
			break
		}
		start = next
	}
	count, first, breaking, err := p.scanFooters(start, limit, nil)
	if err != nil {
		return err
	}
	if count == 0 {
		p.commit.body = Span{start, limit}
		return nil
	}
	if first > start {
		p.commit.body = Span{start, contentEnd(s, first)}
	}
	p.commit.footer = Span{first, limit}
	p.commit.footerBreaking = breaking
	p.commit.footers = make([]Footer, count)
	p.scanFooters(start, limit, p.commit.footers)
	return nil
}

// contentEnd removes terminal line endings, preserving all other whitespace.
func contentEnd(s string, end int) int {
	for end > 0 && s[end-1] == '\n' {
		end--
		if end > 0 && s[end-1] == '\r' {
			end--
		}
	}
	return end
}

// scanFooters counts entries first, then fills exactly sized storage. A value
// lookahead ends at the next lexical token/separator pair, so each line is
// visited a bounded number of times even when candidate values are empty.
func (p *parser) scanFooters(start, limit int, out []Footer) (count, first int, breaking bool, err *ParseError) {
	s := p.source
	boundary := true
	for pos := start; pos < limit; {
		end, next := lineEnd(s, pos)
		if count > 0 || boundary {
			f, ok := footerLine(s, pos, end)
			if ok {
				token := s[f.token.Start:f.token.End]
				isBreaking := token == "BREAKING CHANGE" || token == "BREAKING-CHANGE"
				if !footerHasValue(s, f.value.Start, end, next, limit) {
					if count > 0 || isBreaking {
						if !p.recover {
							return count, first, breaking, &ParseError{InvalidFooterValue, f.value.Start}
						}
						if out == nil {
							p.diagnostics = append(p.diagnostics, ParseError{InvalidFooterValue, f.value.Start})
						}
					} else {
						boundary = false
						pos = next
						continue
					}
				}
				if count == 0 {
					first = pos
				}
				if out != nil {
					if count > 0 {
						valueEnd := pos - 1 // Exclude just the previous line ending.
						if valueEnd > 0 && s[valueEnd-1] == '\r' {
							valueEnd--
						}
						out[count-1].value.End = valueEnd
					}
					out[count] = f
				}
				breaking = breaking || isBreaking
				count++
			}
		}
		boundary = pos == end
		pos = next
	}
	if out != nil && count > 0 {
		out[count-1].value.End = limit
	}
	return count, first, breaking, nil
}

func footerHasValue(s string, start, end, next, limit int) bool {
	if nonblank(s[start:end]) {
		return true
	}
	for pos := next; pos < limit; {
		end, next = lineEnd(s, pos)
		if _, ok := footerLine(s, pos, end); ok {
			return false
		}
		if nonblank(s[pos:end]) {
			return true
		}
		pos = next
	}
	return false
}

// footerLine recognises syntax only. Value completeness is checked separately.
func footerLine(s string, start, end int) (Footer, bool) {
	line := s[start:end]
	if strings.HasPrefix(line, "BREAKING CHANGE: ") {
		return Footer{source: s, token: Span{start, start + 15}, value: Span{start + 17, end}, separator: Colon}, true
	}
	i, noun := 0, false
	for i < len(line) && line[i] != ':' && line[i] != ' ' {
		r, size := utf8.DecodeRuneInString(line[i:])
		if r < utf8.RuneSelf {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-') {
				return Footer{}, false
			}
		} else if !unicode.IsLetter(r) && !unicode.IsNumber(r) && !unicode.IsMark(r) {
			return Footer{}, false
		}
		noun = noun || unicode.IsLetter(r) || unicode.IsNumber(r)
		i += size
	}
	if !noun || i+1 >= len(line) {
		return Footer{}, false
	}
	var sep Separator
	switch {
	case line[i] == ':' && line[i+1] == ' ':
		sep = Colon
	case line[i] == ' ' && line[i+1] == '#':
		if line[:i] == "BREAKING-CHANGE" {
			return Footer{}, false
		}
		sep = Hash
	default:
		return Footer{}, false
	}
	return Footer{source: s, token: Span{start, start + i}, value: Span{start + i + 2, end}, separator: sep}, true
}
