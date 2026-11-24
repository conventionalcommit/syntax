package syntax

// Inspection contains source facts and diagnostics for linting. Structured is
// true when the header identifies the fields unambiguously, even if invalid.
// Otherwise only Message(), Header(), HeaderSpan(), and raw Lines are available.
// Diagnostics are in source order, except byte validation precedes grammar.
// The caller owns the diagnostic slice; changing it cannot affect Commit.
type Inspection struct {
	Commit      Commit
	Diagnostics []ParseError
	Structured  bool
}

// Valid reports whether the message parsed structurally with zero diagnostics.
func (i Inspection) Valid() bool {
	return i.Structured && len(i.Diagnostics) == 0
}

// Inspect retains recoverable fields without repairing or normalising input.
// Parse remains the strict entry point. No diagnostics means Parse would accept
// the same input and produce the same fields. Invalid bytes or an ambiguous
// header stop structural recovery; callers can still inspect raw source lines.
func Inspect(input string) Inspection {
	end, next := lineEnd(input, 0)
	raw := Commit{source: input, header: Span{0, end}}
	if err := checkInput(input); err != nil {
		return Inspection{Commit: raw, Diagnostics: []ParseError{*err}}
	}
	p := parser{
		source:  input,
		commit:  raw,
		recover: true,
	}
	if err := p.parseHeader(0); err != nil {
		p.diagnostics = append(p.diagnostics, *err)
		if err.Code == InvalidType {
			// Only horizontal indentation before an otherwise recognisable header
			// is recoverable. Leading empty lines do not imply a later header.
			start := 0
			for start < end && (input[start] == ' ' || input[start] == '\t') {
				start++
			}
			if start > 0 && start < end {
				err = p.parseHeader(start)
				if err != nil {
					p.diagnostics = append(p.diagnostics, *err)
				}
			}
		}
		if err != nil && !p.recoverHeader(err) {
			return Inspection{Commit: raw, Diagnostics: p.diagnostics}
		}
	}
	p.parseSections(next)
	return Inspection{
		Commit:      p.commit,
		Diagnostics: p.diagnostics,
		Structured:  true,
	}
}

func (p *parser) recoverHeader(err *ParseError) bool {
	if err.Code == InvalidDescription {
		return true
	}
	// A known colon without its required space still provides an unambiguous
	// beginning for the description. A missing colon cannot be inferred.
	pos := err.Offset
	if err.Code != InvalidSeparator || pos == 0 || p.source[pos-1] != ':' {
		return false
	}
	p.commit.sep = Span{pos - 1, pos}
	p.commit.description = Span{pos, p.commit.header.End}
	if !nonblank(p.commit.Description()) {
		p.diagnostics = append(p.diagnostics, ParseError{InvalidDescription, pos})
	}
	return true
}
