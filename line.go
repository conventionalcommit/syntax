package syntax

// Position identifies a 1-based line number and byte column within a message.
type Position struct {
	Line   int
	Column int
}

// Locate converts a zero-based byte offset in input into a 1-based line and
// byte column. Out-of-range offsets are clamped to [0, len(input)].
func Locate(input string, offset int) Position {
	if offset < 0 {
		offset = 0
	}
	if offset > len(input) {
		offset = len(input)
	}
	line := 1
	lineStart := 0
	for i := 0; i < offset; i++ {
		if input[i] == '\n' {
			line++
			lineStart = i + 1
		}
	}
	return Position{Line: line, Column: offset - lineStart + 1}
}

// Line separates content from its line ending using byte positions.
type Line struct {
	Content Span
	Ending  Span
}

// LineIterator visits lines without allocating a collection. Its zero value
// visits one empty line. Copies advance independently and retain the input.
type LineIterator struct {
	source string
	pos    int
	done   bool
}

// Lines iterates raw bytes without validating them. A final newline produces
// an empty last line, and empty input produces one empty line. Standalone CR
// is content; CR immediately followed by LF belongs to the line ending.
func Lines(input string) LineIterator { return LineIterator{source: input} }

// Next returns the next line and whether one was available. After exhaustion
// it returns a zero Line and false on every subsequent call.
func (it *LineIterator) Next() (Line, bool) {
	if it.done {
		return Line{}, false
	}
	end, next := lineEnd(it.source, it.pos)
	line := Line{Content: Span{it.pos, end}, Ending: Span{end, next}}
	it.done = end == len(it.source)
	it.pos = next
	return line, true
}
