// Package syntax parses Conventional Commits while preserving source bytes.
package syntax

import (
	"iter"
	"strconv"
	"strings"
)

// Span is a half-open byte range into a commit's original message.
// An absent field has the zero span.
type Span struct {
	Start int
	End   int
}

// Len returns the number of bytes in the range.
func (s Span) Len() int { return s.End - s.Start }

// IsZero reports whether s is the zero span, indicating an absent field.
func (s Span) IsZero() bool { return s == (Span{}) }

// IsEmpty reports whether s covers zero bytes.
func (s Span) IsEmpty() bool { return s.End <= s.Start }

// Contains reports whether byte offset lies within [Start, End).
func (s Span) Contains(offset int) bool { return offset >= s.Start && offset < s.End }

// Separator identifies the punctuation separating a footer token and value.
type Separator uint8

const (
	Colon Separator = iota + 1 // Colon followed by one ASCII space.
	Hash                       // One ASCII space followed by hash.
)

func (s Separator) String() string {
	switch s {
	case Colon:
		return "Colon"
	case Hash:
		return "Hash"
	default:
		return "Separator(" + strconv.Itoa(int(s)) + ")"
	}
}

// Footer represents one parsed footer entry. Duplicate tokens are kept.
type Footer struct {
	source    string
	token     Span
	value     Span
	separator Separator
}

// Span returns the range from the start of Token to the end of Value.
func (f Footer) Span() Span {
	if f.token == (Span{}) && f.value == (Span{}) {
		return Span{}
	}
	return Span{f.token.Start, f.value.End}
}

func (f Footer) Token() string        { return f.source[f.token.Start:f.token.End] }
func (f Footer) TokenSpan() Span      { return f.token }
func (f Footer) Separator() Separator { return f.separator }
func (f Footer) Value() string        { return f.source[f.value.Start:f.value.End] }
func (f Footer) ValueSpan() Span      { return f.value }

// SeparatorSpan returns the delimiter span between Token and Value, or zero if absent.
func (f Footer) SeparatorSpan() Span {
	if f.token == (Span{}) {
		return Span{}
	}
	return Span{f.token.End, f.value.Start}
}

// IsBreaking reports whether this footer uses an uppercase breaking token.
func (f Footer) IsBreaking() bool {
	t := f.Token()
	return t == "BREAKING CHANGE" || t == "BREAKING-CHANGE"
}

// Gap describes the raw line-ending separator before a present body or footer.
// Span includes the preceding section's line ending. BlankLines excludes that
// first ending. An absent section returns the zero gap.
type Gap struct {
	Span       Span
	BlankLines int
}

// IsZero reports whether g represents an absent separator.
func (g Gap) IsZero() bool { return g == (Gap{}) }

// Commit is a read-only parsed message. Copies share the original input and
// footer storage. Retaining any field may retain the full input allocation.
// Its zero value is safe to inspect.
type Commit struct {
	source         string
	header         Span
	kind           Span
	scope          Span
	sep            Span
	description    Span
	body           Span
	footer         Span
	footers        []Footer
	bang           int
	footerBreaking bool
}

func (c Commit) Message() string           { return c.source }
func (c Commit) Text(span Span) string     { return c.source[span.Start:span.End] }
func (c Commit) Header() string            { return c.source[c.header.Start:c.header.End] }
func (c Commit) HeaderSpan() Span          { return c.header }
func (c Commit) Type() string              { return c.source[c.kind.Start:c.kind.End] }
func (c Commit) TypeSpan() Span            { return c.kind }
func (c Commit) HasScope() bool            { return c.scope != (Span{}) }
func (c Commit) Scope() string             { return c.source[c.scope.Start:c.scope.End] }
func (c Commit) ScopeSpan() Span           { return c.scope }
func (c Commit) HeaderSeparator() string   { return c.source[c.sep.Start:c.sep.End] }
func (c Commit) HeaderSeparatorSpan() Span { return c.sep }
func (c Commit) Description() string       { return c.source[c.description.Start:c.description.End] }
func (c Commit) DescriptionSpan() Span     { return c.description }
func (c Commit) HasBody() bool             { return c.body != (Span{}) }
func (c Commit) Body() string              { return c.source[c.body.Start:c.body.End] }
func (c Commit) BodySpan() Span            { return c.body }
func (c Commit) HasFooter() bool           { return c.footer != (Span{}) }
func (c Commit) Footer() string            { return c.source[c.footer.Start:c.footer.End] }
func (c Commit) FooterSpan() Span          { return c.footer }
func (c Commit) FooterCount() int          { return len(c.footers) }
func (c Commit) IsBreakingChange() bool    { return c.HasHeaderBang() || c.HasBreakingFooter() }

// Indent returns leading horizontal whitespace before Type, if recovered by Inspect.
func (c Commit) Indent() string {
	span := c.IndentSpan()
	return c.source[span.Start:span.End]
}

// IndentSpan locates leading horizontal whitespace before Type, or returns zero if absent.
func (c Commit) IndentSpan() Span {
	if c.kind == (Span{}) || c.kind.Start == 0 {
		return Span{}
	}
	return Span{0, c.kind.Start}
}

// Trailing returns terminal line endings after the last content section.
func (c Commit) Trailing() string {
	span := c.TrailingSpan()
	return c.source[span.Start:span.End]
}

// TrailingSpan locates terminal line endings after the last content section.
func (c Commit) TrailingSpan() Span {
	end := contentEnd(c.source, len(c.source))
	if end >= len(c.source) {
		return Span{}
	}
	return Span{end, len(c.source)}
}

// FooterAt returns a footer by value. It panics for an out-of-range index.
func (c Commit) FooterAt(index int) Footer { return c.footers[index] }

// Footers iterates over all parsed footers in source order.
func (c Commit) Footers() iter.Seq[Footer] {
	return func(yield func(Footer) bool) {
		for _, f := range c.footers {
			if !yield(f) {
				return
			}
		}
	}
}

// HasHeaderBang reports a bang immediately before the header colon.
func (c Commit) HasHeaderBang() bool { return c.bang > 0 }

// HeaderBangSpan locates the header bang, or returns the zero span if absent.
func (c Commit) HeaderBangSpan() Span {
	if c.bang == 0 {
		return Span{}
	}
	return Span{c.bang, c.bang + 1}
}

// HasBreakingFooter reports an uppercase breaking footer marker. During
// inspection its value can be invalid; diagnostics describe that separately.
func (c Commit) HasBreakingFooter() bool { return c.footerBreaking }

// BodyGap describes the separator between the header and body. Inspection can
// expose a present body with zero blank lines and a corresponding diagnostic.
func (c Commit) BodyGap() Gap {
	if c.body == (Span{}) {
		return Gap{}
	}
	return c.gap(c.header.End, c.body.Start)
}

// FooterGap describes the separator before the footer section, following the
// body if present and the header otherwise.
func (c Commit) FooterGap() Gap {
	if c.footer == (Span{}) {
		return Gap{}
	}
	start := c.header.End
	if c.body != (Span{}) {
		start = c.body.End
	}
	return c.gap(start, c.footer.Start)
}

func (c Commit) gap(start, end int) Gap {
	count := strings.Count(c.source[start:end], "\n")
	if count > 0 {
		count--
	}
	return Gap{Span{start, end}, count}
}

// Position converts a zero-based byte offset in Message() into a 1-based line
// and byte column.
func (c Commit) Position(offset int) Position {
	return Locate(c.source, offset)
}

// Lines visits a source range with offsets relative to Message(), including
// when the range is empty. Invalid ranges panic like ordinary slice bounds.
func (c Commit) Lines(span Span) LineIterator {
	_ = c.source[span.Start:span.End]
	return LineIterator{source: c.source[:span.End], pos: span.Start}
}

// BodyLines iterates over lines in Body(), yielding zero lines when absent.
func (c Commit) BodyLines() iter.Seq[Line] {
	return c.sectionLines(c.body)
}

// FooterLines iterates over lines in Footer(), yielding zero lines when absent.
func (c Commit) FooterLines() iter.Seq[Line] {
	return c.sectionLines(c.footer)
}

func (c Commit) sectionLines(span Span) iter.Seq[Line] {
	return func(yield func(Line) bool) {
		if span == (Span{}) {
			return
		}
		it := c.Lines(span)
		for line, ok := it.Next(); ok; line, ok = it.Next() {
			if !yield(line) {
				return
			}
		}
	}
}
