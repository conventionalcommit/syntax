package syntax

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

type parser struct {
	source      string
	commit      Commit
	diagnostics []ParseError
	recover     bool
}

// Parse reads one Conventional Commit. Successful fields reference input without
// copying. Failure returns a zero Commit and a *ParseError, with no partial result.
func Parse(input string) (Commit, error) {
	if err := checkInput(input); err != nil {
		return Commit{}, err
	}
	end, next := lineEnd(input, 0)
	p := parser{
		source: input,
		commit: Commit{source: input, header: Span{0, end}},
	}
	if err := p.parseHeader(0); err != nil {
		return Commit{}, err
	}
	if err := p.parseSections(next); err != nil {
		return Commit{}, err
	}
	return p.commit, nil
}

// checkInput validates bytes before structural parsing so error precedence does
// not depend on which fields the scanner needs to visit.
func checkInput(s string) *ParseError {
	for i := 0; i < len(s); {
		b := s[i]
		if b > '\r' && b < utf8.RuneSelf {
			i++
			continue
		}
		switch {
		case b == 0:
			return &ParseError{InvalidCharacter, i}
		case b == '\r':
			if i+1 == len(s) || s[i+1] != '\n' {
				return &ParseError{InvalidCharacter, i}
			}
			i += 2
		case b < utf8.RuneSelf:
			i++
		default:
			_, size := utf8.DecodeRuneInString(s[i:])
			if size == 1 {
				return &ParseError{InvalidEncoding, i}
			}
			i += size
		}
	}
	return nil
}

// lineEnd excludes a validated line ending; next points beyond that ending.
func lineEnd(s string, start int) (end, next int) {
	i := strings.IndexByte(s[start:], '\n')
	if i < 0 {
		return len(s), len(s)
	}
	end = start + i
	next = end + 1
	if end > start && s[end-1] == '\r' {
		end--
	}
	return end, next
}

func (p *parser) parseHeader(start int) *ParseError {
	s := p.source[:p.commit.header.End]
	i, noun := start, false
	for i < len(s) && s[i] != '(' && s[i] != '!' && s[i] != ':' {
		r, size := utf8.DecodeRuneInString(s[i:])
		if !nameRune(r) {
			return &ParseError{InvalidType, i}
		}
		noun = noun || unicode.IsLetter(r) || unicode.IsNumber(r)
		i += size
	}
	if !noun {
		return &ParseError{InvalidType, i}
	}
	p.commit.kind = Span{start, i}
	if i < len(s) && s[i] == '(' {
		i++
		scopeStart := i
		noun = false
		for i < len(s) && s[i] != ')' {
			r, size := utf8.DecodeRuneInString(s[i:])
			if !(nameRune(r) || (r == ' ' && i > scopeStart)) {
				return &ParseError{InvalidScope, i}
			}
			noun = noun || unicode.IsLetter(r) || unicode.IsNumber(r)
			i += size
		}
		if i == len(s) || !noun {
			return &ParseError{InvalidScope, i}
		}
		if s[i-1] == ' ' {
			return &ParseError{InvalidScope, i - 1}
		}
		p.commit.scope = Span{scopeStart, i}
		i++
	}
	if i < len(s) && s[i] == '!' {
		p.commit.bang = i
		i++
	}
	if i == len(s) || s[i] != ':' {
		return &ParseError{InvalidSeparator, i}
	}
	i++
	if i == len(s) || s[i] != ' ' {
		return &ParseError{InvalidSeparator, i}
	}
	i++
	p.commit.sep = Span{i - 2, i}
	p.commit.description = Span{i, len(s)}
	if !nonblank(s[i:]) {
		return &ParseError{InvalidDescription, i}
	}
	return nil
}

func nameRune(r rune) bool {
	if r < utf8.RuneSelf {
		return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || strings.ContainsRune("\"#%&'*,./;?@[\\]_{}", r) || r == '-'
	}
	return unicode.IsLetter(r) || unicode.IsMark(r) || unicode.IsNumber(r) || unicode.IsPunct(r)
}

func nonblank(s string) bool {
	for _, r := range s {
		if !unicode.IsSpace(r) {
			return true
		}
	}
	return false
}
