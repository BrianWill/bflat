package main

import "errors"

// returns true if rune is a letter of the English alphabet
func isAlpha(r rune) bool {
	return (r >= 65 && r <= 90) || (r >= 97 && r <= 122)
}

// returns true if rune is a numeral
func isNumeral(r rune) bool {
	return (r >= 48 && r <= 57)
}

func isSigil(r rune) bool {
	for _, s := range sigils {
		if r == s {
			return true
		}
	}
	return false
}

func lex(code string) ([]Token, error) {
	tokens := []Token{}
	runes := []rune(code)
	line := 1
	column := 1
	for i := 0; i < len(runes); {
		r := runes[i]
		if r >= 128 {
			return nil, errors.New("File improperly contains a non-ASCII character at line " + itoa(line) + " and column " + itoa(column))
		}
		if r == '\n' {
			tokens = append(tokens, Token{Newline, "\n", line, column})
			line++
			column = 1
			i++
		} else if r == '\r' {
			if runes[i+1] != '\n' {
				return nil, errors.New("File improperly contains a CR not followed by a LF at end of line " + itoa(line))
			}
			tokens = append(tokens, Token{Newline, "\n", line, column})
			line++
			column = 1
			i += 2
		} else if r == '/' && runes[i+1] == '/' { // start of a comment
			i += 2
			for runes[i] != '\n' {
				i++
			}
			i++
			tokens = append(tokens, Token{Newline, "\n", line, column})
			line++
			column = 1
		} else if r == '(' {
			tokens = append(tokens, Token{OpenParen, "(", line, column})
			column++
			i++
		} else if r == ')' {
			tokens = append(tokens, Token{CloseParen, ")", line, column})
			column++
			i++
		} else if r == '[' {
			tokens = append(tokens, Token{OpenSquare, "[", line, column})
			column++
			i++
		} else if r == ']' {
			tokens = append(tokens, Token{CloseSquare, "]", line, column})
			column++
			i++
		} else if r == '{' {
			tokens = append(tokens, Token{OpenCurly, "{", line, column})
			column++
			i++
		} else if r == '}' {
			tokens = append(tokens, Token{CloseCurly, "}", line, column})
			column++
			i++
		} else if r == '<' {
			tokens = append(tokens, Token{OpenAngle, "<", line, column})
			column++
			i++
		} else if r == '>' {
			tokens = append(tokens, Token{CloseAngle, ">", line, column})
			column++
			i++
		} else if r == ' ' {
			isIndentation := i > 0 && runes[i-1] == '\n'
			firstIdx := i
			for i < len(runes) && runes[i] == ' ' {
				column++
				i++
			}
			content := string(runes[firstIdx:i])
			if isIndentation && len(content)%IndentSpaces != 0 {
				return nil, errors.New("Indentation on line " + itoa(line) + " is not a multiple of " + itoa(IndentSpaces) + " spaces.")
			}
			tokens = append(tokens, Token{Spaces, content, line, column})
		} else if r == '\t' {
			return nil, errors.New("File improperly contains a tab character: line " + itoa(line) + " and column " + itoa(column))
		} else if r == '`' { // start of a string
			prev := r
			endIdx := i + 1
			endColumn := column
			endLine := line
			for {
				if endIdx >= len(runes) {
					return nil, errors.New("String literal not closed by end of file on line " + itoa(line) + " and column " + itoa(column))
				}
				current := runes[endIdx]
				if current == '\n' {
					endLine++
					endColumn = 1
				} else {
					endColumn++
				}
				if current == '`' && prev != '\\' { // end of the string
					endIdx++
					break
				}
				prev = current
				endIdx++
			}
			tokens = append(tokens, Token{StringLiteral, string(runes[i:endIdx]), line, column})
			column = endColumn
			line = endLine
			i = endIdx
		} else if isNumeral(r) { // start of a number
			endIdx := i + 1
			for isNumeral(runes[endIdx]) {
				endIdx++
			}
			tokens = append(tokens, Token{NumberLiteral, string(runes[i:endIdx]), line, column})
			column += (endIdx - i)
			i = endIdx
		} else if isAlpha(r) { // start of a word
			endIdx := i + 1
			r := runes[endIdx]
			for isAlpha(r) || r == '_' || isNumeral(r) {
				endIdx++
				r = runes[endIdx]
			}

			content := string(runes[i:endIdx])

			tokens = append(tokens, Token{Word, content, line, column})
			column += (endIdx - i)
			i = endIdx
		} else if isSigil(r) {
			tokens = append(tokens, Token{Sigil, string(r), line, column})
			column++
			i++
		} else {
			return nil, errors.New("Unexpected character " + string(r) + " at line " + itoa(line) + ", column " + itoa(column))
		}
	}
	return tokens, nil
}

func read(tokens []Token) ([]Atom, error) {
	readerData := []Atom{}
	for i := 0; i < len(tokens); {
		atom, numTokens, err := readAtom(tokens, NoClose)
		if err != nil {
			return nil, err
		}
		tokens = tokens[numTokens:]
		if atom != nil {
			readerData = append(readerData, atom)
		}
	}
	return readerData, nil
}

// atom may be nil if tokens consumed contain only whitespace
func readAtom(tokens []Token, expectedClose TokenType) (Atom, int, error) {
	i := 0
	// advance through all whitespace tokens
Loop:
	for i < len(tokens) {
		t := tokens[i]
		switch t.Type {
		case Spaces, Newline:
			i++
		default:
			break Loop
		}
	}
	elements := []Atom{}
Loop2:
	for i < len(tokens) {
		t := tokens[i]
		switch t.Type {
		case Word:
			elements = append(elements, Symbol{t.Content, t.Line, t.Column})
			i++
		case Sigil:
			elements = append(elements, SigilAtom{t.Content, t.Line, t.Column})
			i++
		case NumberLiteral:
			elements = append(elements, NumberAtom{t.Content, t.Line, t.Column})
			i++
		case StringLiteral:
			elements = append(elements, StringAtom{t.Content, t.Line, t.Column})
			i++
		case Spaces, Newline:
			i++
			break Loop2
		case OpenParen, OpenSquare, OpenCurly, OpenAngle:
			var end TokenType
			switch t.Type {
			case OpenParen:
				end = CloseParen
			case OpenSquare:
				end = CloseSquare
			case OpenCurly:
				end = CloseCurly
			case OpenAngle:
				end = CloseAngle
			}
			list, n, err := readList(tokens[i:], end)
			if err != nil {
				return nil, 0, err
			}
			i += n
			elements = append(elements, list)
		case CloseParen, CloseSquare, CloseCurly, CloseAngle:
			if t.Type == expectedClose {
				// do NOT consume the token
				break Loop2
			} else {
				return nil, 0, errors.New("Unexpected atom token: line " + itoa(t.Line) + " column " + itoa(t.Column))
			}
		default:
			return nil, 0, errors.New("Unexpected atom token: line " + itoa(t.Line) + " column " + itoa(t.Column))
		}
	}

	if len(elements) == 1 {
		return elements[0], i, nil
	} else if len(elements) > 1 {
		return AtomChain{elements, tokens[0].Line, tokens[0].Column}, i, nil
	}
	return nil, i, nil
}

func readList(tokens []Token, expectedClose TokenType) (Atom, int, error) {
	i := 1
	elements := []Atom{}
Loop:
	for i < len(tokens) {
		t := tokens[i]
		switch t.Type {
		case expectedClose:
			i++
			break Loop
		default:
			atom, n, err := readAtom(tokens[i:], expectedClose)
			if err != nil {
				return nil, 0, err
			}
			i += n
			if atom != nil {
				elements = append(elements, atom)
			}
		}
	}

	t := tokens[0]
	switch t.Type {
	case OpenParen:
		return ParenList{elements, t.Line, t.Column}, i, nil
	case OpenSquare:
		return SquareList{elements, t.Line, t.Column}, i, nil
	case OpenCurly:
		return CurlyList{elements, t.Line, t.Column}, i, nil
	case OpenAngle:
		return AngleList{elements, t.Line, t.Column}, i, nil
	default:
		return nil, 0, errors.New("Internal error. Expecting an open delimiter: line " + itoa(t.Line) + " column " + itoa(t.Column))
	}
}
