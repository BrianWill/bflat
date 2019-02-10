package main

import (
	"math"
	"strings"

	"github.com/gopherjs/gopherjs/js"
)

// assumes no newlines
func insertText(text string, textBuffer []string, c *Cursor, e *Editor) {
	line := textBuffer[c.line]
	textBuffer[c.line] = line[:c.pos] + text + line[c.pos:]
	c.pos += len(text)
	c.preferredPos = c.pos
	c.selectionPos = c.pos
	c.selectionLine = c.line
	updateScrollAfterCursorMove(c.line, e)
}

func updateScrollAfterCursorMove(line int, e *Editor) {
	// returns [min, max]
	minMaxScrollForLine := func(line int, d *Dimensions, e *Editor) (min int, max int) {
		const maxAdjustment = -6
		const minAdjustment = 12
		max = line*d.lineHeight + d.firstLineOffsetY // max is top of the line
		min = max - d.height + d.lineHeight
		max += maxAdjustment
		min += minAdjustment
		if max > e.maxScroll {
			max = e.maxScroll
		}
		if min < 0 {
			min = 0
		}
		return
	}

	min, max := minMaxScrollForLine(line, &e.dimensions, e)
	if e.scroll < min {
		e.scroll = min
	} else if e.scroll > max {
		e.scroll = max
	}
}

// return false if area encompases no text (and so no delete is performed)
// updates cursor and selection states accordingly
func deleteSelection(c *Cursor, textBuffer *[]string) bool {
	tb := *textBuffer
	if c.pos == c.selectionPos && c.line == c.selectionLine {
		return false
	}
	if c.line == c.selectionLine {
		text := tb[c.line]
		if c.pos < c.selectionPos {
			tb[c.line] = text[:c.pos] + text[c.selectionPos:]
		} else {
			tb[c.line] = text[c.selectionPos:c.pos] + text[:c.pos]
			c.pos = c.selectionPos
		}
	} else if c.line < c.selectionLine {
		leading := tb[c.line][:c.pos]
		trailing := tb[c.selectionLine][:c.selectionPos]
		tb = append(tb[:c.line+1], tb[c.selectionLine+1:]...) // discard lines
		tb[c.line] = leading + trailing
		*textBuffer = tb
	} else if c.line > c.selectionLine {
		leading := tb[c.selectionLine][:c.selectionPos]
		trailing := tb[c.line][:c.pos]
		tb = append(tb[:c.selectionLine+1], tb[c.line+1:]...) // discard lines
		tb[c.selectionLine] = leading + trailing
		*textBuffer = tb
		c.line = c.selectionLine
		c.pos = c.selectionPos
	}
	c.selectionPos = c.pos
	c.selectionLine = c.line
	return true
}

func keyInput(evt *js.Object, e *Editor) {
	key := evt.Get("key").String()
	if len(key) == 1 {
		if evt.Get("metaKey").Bool() {
			hotkeyInput(evt, e)
			return
		}
		code := key[0]
		if code >= 32 { // if not a control character
			deleteSelection(&e.cursor, &e.textBuffer)
			insertText(key, e.textBuffer, &e.cursor, e)
		} else {
			return
		}
	} else {
		cursorKeyInput(evt, e, &e.cursor, e.textBuffer)
	}
	showCursor(&e.cursor, e)
}

func updateMaxScroll(e *Editor, d *Dimensions) {
	// subtract only half height so we can scroll a bit past last line
	e.maxScroll = d.lineHeight*len(e.textBuffer) - (d.height / 2)
	if e.maxScroll < 0 {
		e.maxScroll = 0
	}
	if e.scroll > e.maxScroll {
		e.scroll = e.maxScroll
	}
}

var cursorBlinkTimeoutHandle int

func showCursor(c *Cursor, e *Editor) {
	var toggleCursor func()
	toggleCursor = func() {
		if c.shown {
			c.shown = false
		} else {
			c.shown = true
		}
		draw(e)
		cursorBlinkTimeoutHandle = window.Call("setTimeout", toggleCursor, c.blinkTime).Int()
	}

	window.Call("clearTimeout", cursorBlinkTimeoutHandle)
	c.shown = true
	cursorBlinkTimeoutHandle = window.Call("setTimeout", toggleCursor, c.blinkTime).Int()

}

func hotkeyInput(evt *js.Object, e *Editor) {
	key := evt.Get("key").String()
	switch key {
	case "r":
	case "R":
		// reload page
		return
	case "s":
	case "S":
		evt.Call("preventDefault")
		return
	case "-":
	case "=":
	case "0":
		// adjust zoom
		return
	case "o":
	case "O":
		evt.Call("preventDefault")
		return
	case "u":
	case "U":
		evt.Call("preventDefault")
		return
	case "i":
	case "I":
		evt.Call("preventDefault")
		return
	case "k":
	case "K":
		evt.Call("preventDefault")
		deleteCurrentLine(e.textBuffer, &e.cursor, e)
		showCursor(&e.cursor, e)
		draw(e)
		return
	case "c":
	case "C":
		evt.Call("preventDefault")
		copy(e.textBuffer, &e.cursor)
		return
	case "v":
	case "V":
		evt.Call("preventDefault")
		paste(e, &e.cursor)
		showCursor(&e.cursor, e)
		draw(e)
		return
	default:
		evt.Call("preventDefault")
		return
	}
}

func paste(e *Editor, c *Cursor) {
	// accounts for newlines in the inserted text
	insertTextMultiline := func(text string, tb []string, c *Cursor, e *Editor) {
		lines := strings.Split(text, "\n")
		if len(lines) == 1 {
			insertText(text, tb, c, e)
			return
		}
		preceding := tb[c.line][0:c.pos]
		following := tb[c.line][c.pos:]
		lines[0] = preceding + lines[0]
		c.pos = len(lines[len(lines)-1])
		c.preferredPos = c.pos
		lines[len(lines)-1] += following
		e.textBuffer = append(tb[:c.line], append(lines, tb[c.line:]...)...)
		c.line += len(lines) - 1
		c.selectionPos = c.pos
		c.selectionLine = c.line
		updateMaxScroll(e, &e.dimensions)
		updateScrollAfterCursorMove(c.line, e)
	}

	println("pasting")
	promise := js.Global.Get("navigator").Get("clipboard").Call("readText")
	promise.Call("text",
		func(text string) {
			// `text` contains the text read from the clipboard
			println("Text: ", text)
			insertTextMultiline(text, e.textBuffer, c, e)
			showCursor(c, e)
			draw(e)
		},
	).Call("catch",
		func(err *js.Object) {
			// maybe user didn't grant access to read from clipboard
			println("Something went wrong reading clipboard: ", err)
		},
	)
}

func copy(textBuffer []string, c *Cursor) {
	selection := getSelection(textBuffer, c)
	if len(selection) > 0 {
		promise := js.Global.Get("navigator").Get("clipboard").Call("writeText", selection)
		promise.Call("then",
			func(msg *js.Object) {
				println("successfully wrote to clipboard: ", msg)
			},
			func() {
				println("failed to write to clipboard")
			},
		)
	}
}

// returns text in selection area (or empty string if no selected text)
func getSelection(textBuffer []string, c *Cursor) string {
	if c.pos == c.selectionPos && c.line == c.selectionLine {
		return ""
	}
	if c.line == c.selectionLine {
		line := textBuffer[c.line]
		if c.pos < c.selectionPos {
			return line[c.pos:c.selectionPos]
		} else {
			return line[c.selectionPos:c.pos]
		}
	} else {
		startPos := c.pos
		startLine := c.line
		endPos := c.selectionPos
		endLine := c.selectionLine
		if c.line > c.selectionLine {
			startPos = c.selectionPos
			startLine = c.selectionLine
			endPos = c.pos
			endLine = c.line
		}
		s := textBuffer[startLine][startPos:]
		for i := startLine + 1; i < endLine; i++ {
			s += "\n" + textBuffer[i]
		}
		s += "\n" + textBuffer[endLine][:endPos]
		return s
	}
}

func deleteCurrentLine(textBuffer []string, c *Cursor, e *Editor) {
	if len(textBuffer) == 1 {
		e.textBuffer = []string{""}
		c.pos = 0
		c.preferredPos = 0
		c.line = 0
	} else {
		e.textBuffer = append(textBuffer[:c.line], textBuffer[c.line-1:]...)
		if c.line > len(textBuffer)-1 {
			c.line = len(textBuffer) - 1
		}
		newLineLength := len(textBuffer[c.line])
		if c.preferredPos <= newLineLength {
			c.pos = c.preferredPos
		} else {
			c.pos = newLineLength
			c.preferredPos = newLineLength
		}
		updateMaxScroll(e, &e.dimensions)
	}
	c.selectionPos = c.pos
	c.selectionLine = c.line
	updateScrollAfterCursorMove(c.line, e)
}

func cursorKeyInput(evt *js.Object, e *Editor, c *Cursor, textBuffer []string) {
	// return [newPos, newLine], or null if cursor already at end
	nextWhitespaceSkip := func(c *Cursor, textBuffer []string, isShift bool) {
		text := textBuffer[c.line]
		if c.pos == len(text) {
			if c.line == len(textBuffer)-1 {
				return
			}
			c.line++
			c.pos = 0
			text = textBuffer[c.line]
		}
		// don't make an else of above if!
		if c.pos != len(text) {
			remaining := text[c.pos:]
			trimmed := strings.TrimLeft(remaining, " \t")
			firstSpace := strings.Index(trimmed, " ")
			if firstSpace == -1 {
				c.pos = len(text)
			} else {
				c.pos = c.pos + (len(remaining) - len(trimmed)) + firstSpace
			}
		}
		c.preferredPos = c.pos
		if !isShift {
			c.selectionPos = c.pos
			c.selectionLine = c.line
		}
	}

	// returns new [pos, line], or null if cursor already at start of text
	prevWhitespaceSkip := func(c *Cursor, textBuffer []string, isShift bool) {
		text := textBuffer[c.line]
		if c.pos == 0 {
			if c.line == 0 {
				return
			}
			c.line--
			c.pos = len(textBuffer[c.line])
		}
		// don't make an else of above if!
		if c.pos != 0 {
			preceding := text[:c.pos]
			trimmed := strings.TrimRight(preceding, " \t")
			firstSpace := strings.LastIndex(trimmed, " ")
			if firstSpace == -1 {
				c.pos = 0
			} else {
				c.pos = firstSpace + 1
			}
		}
		c.preferredPos = c.pos
		if !isShift {
			c.selectionPos = c.pos
			c.selectionLine = c.line
		}
	}

	metaKey := evt.Get("metaKey").Bool()
	shiftKey := evt.Get("shiftKey").Bool()
	altKey := evt.Get("altKey").Bool()

	switch evt.Get("key").String() {
	// generally immitates VSCode behavior
	case "ArrowLeft":
		if metaKey {
			evt.Call("preventDefault")
			text := textBuffer[c.line]
			trimmed := strings.TrimLeft(text, " \t")
			newPos := len(text) - len(trimmed)
			if newPos == c.pos {
				c.pos = 0
			} else {
				c.pos = newPos
			}
			if !shiftKey {
				c.selectionPos = c.pos
			}
			c.preferredPos = c.pos
		} else if altKey {
			prevWhitespaceSkip(c, textBuffer, shiftKey)
		} else {
			if c.pos == 0 {
				if c.line == 0 {
					return
				}
				c.line--
				c.pos = len(textBuffer[c.line])
				c.preferredPos = c.pos
			} else {
				c.pos--
				c.preferredPos = c.pos
			}
			if !shiftKey {
				c.selectionPos = c.pos
				c.selectionLine = c.line
			}
		}
		updateScrollAfterCursorMove(c.line, e)
	case "ArrowRight":
		if metaKey {
			evt.Call("preventDefault")
			c.pos = len(textBuffer[c.line])
			c.preferredPos = c.pos
			if !shiftKey {
				c.selectionPos = c.pos
			}
		} else if altKey {
			nextWhitespaceSkip(c, textBuffer, shiftKey)
		} else {
			if c.pos == len(textBuffer[c.line]) {
				if c.line == len(textBuffer)-1 {
					return
				}
				c.line++
				c.pos = 0
				c.preferredPos = 0
			} else {
				c.pos++
				c.preferredPos = c.pos
			}
			if !shiftKey {
				c.selectionPos = c.pos
				c.selectionLine = c.line
			}
		}
		updateScrollAfterCursorMove(c.line, e)
	case "ArrowUp":
		if c.line > 0 {
			c.line--
			newLineLength := len(textBuffer[c.line])
			if c.preferredPos <= newLineLength {
				c.pos = c.preferredPos
			} else {
				c.pos = newLineLength
			}
			if !shiftKey {
				c.selectionPos = c.pos
				c.selectionLine = c.line
			}
		}
		updateScrollAfterCursorMove(c.line, e)
	case "ArrowDown":
		if c.line < len(textBuffer)-1 {
			c.line++
			newLineLength := len(textBuffer[c.line])
			if c.preferredPos <= newLineLength {
				c.pos = c.preferredPos
			} else {
				c.pos = newLineLength
			}
			if !shiftKey {
				c.selectionPos = c.pos
				c.selectionLine = c.line
			}
		}
		updateScrollAfterCursorMove(c.line, e)
	case "Backspace":
		if deleteSelection(c, &e.textBuffer) {
			updateMaxScroll(e, &e.dimensions)
		} else if c.pos > 0 {
			c.pos--
			c.preferredPos = c.pos
			line := textBuffer[c.line]
			textBuffer[c.line] = line[:c.pos] + line[c.pos+1:]
			c.selectionPos = c.pos
		} else if c.line > 0 {
			var prevLineIdx = c.line - 1
			var prevLine = textBuffer[prevLineIdx]
			textBuffer[prevLineIdx] = prevLine + textBuffer[c.line]
			e.textBuffer = append(textBuffer[:c.line], textBuffer[c.line+1:]...)
			c.pos = len(prevLine)
			c.line = prevLineIdx
			c.selectionPos = c.pos
			c.selectionLine = c.line
			updateMaxScroll(e, &e.dimensions)
		}
		updateScrollAfterCursorMove(c.line, e)
	case "Delete":
		if deleteSelection(c, &e.textBuffer) {
			updateMaxScroll(e, &e.dimensions)
		} else if c.pos < len(textBuffer[c.line]) {
			line := textBuffer[c.line]
			textBuffer[c.line] = line[:c.pos] + line[c.pos+1:]
			c.preferredPos = c.pos
			c.selectionPos = c.pos
			c.selectionLine = c.line
		} else if c.line < len(textBuffer)-1 {
			textBuffer[c.line] = textBuffer[c.line] + textBuffer[c.line+1]
			e.textBuffer = append(textBuffer[:c.line+1], textBuffer[c.line+2:]...)
			c.selectionPos = c.pos
			c.selectionLine = c.line
			updateMaxScroll(e, &e.dimensions)
		}
		updateScrollAfterCursorMove(c.line, e)
	case "Enter":
		deleteSelection(c, &e.textBuffer)
		textBuffer = append(textBuffer, "") // make space for one more line
		e.textBuffer = textBuffer
		text := textBuffer[c.line]
		preceding := text[:c.pos]
		following := text[c.pos:]
		// shift lines down
		for i := len(textBuffer) - 1; i > c.line+1; i-- {
			e.textBuffer[i] = e.textBuffer[i-1]
		}
		textBuffer[c.line] = preceding
		textBuffer[c.line+1] = following
		c.line++
		c.pos = 0
		c.preferredPos = c.pos
		c.selectionPos = c.pos
		c.selectionLine = c.line
		updateMaxScroll(e, &e.dimensions)
		updateScrollAfterCursorMove(c.line, e)
	case "Tab":
		evt.Call("preventDefault")
		deleteSelection(c, &e.textBuffer)
		numSpacesForTab := e.dimensions.numSpacesForTab
		numSpaces := numSpacesForTab - (c.pos % numSpacesForTab)
		var insert string
		switch numSpaces {
		case 1:
			insert = " "
		case 2:
			insert = "  "
		case 3:
			insert = "   "
		case 4:
			insert = "    "
		}
		insertText(insert, textBuffer, c, e)
	}
}

func mouseInput(evt *js.Object, isClick bool, e *Editor) {
	c := &e.cursor
	d := e.dimensions
	tb := e.textBuffer

	shiftKey := evt.Get("shiftKey").Bool()

	// todo: using clientX/Y for now; should get relative from canvas (to allow a canvas editor that isn't full page)
	const textCursorOffsetY = 7 // want text cursor to select as if from center of the cursor, not top (there's no way to get the cursor's actual height, so we guess its half height)

	clientX := evt.Get("clientX").Int()
	clientY := evt.Get("clientY").Int()

	pos := int(math.Round(float64(clientX-d.lineOffsetX) / d.characterWidth))
	line := int(math.Floor(float64(
		(clientY - d.firstLineOffsetY + textCursorOffsetY + e.scroll) / d.lineHeight)))
	if pos < 0 {
		pos = 0
	}
	if line < 0 {
		line = 0
	}
	if line > len(tb)-1 {
		line = len(tb) - 1
	}
	if pos > len(tb[line]) {
		pos = len(tb[line])
	}
	c.pos = pos
	c.preferredPos = pos
	c.line = line
	if isClick {
		if !shiftKey {
			c.selectionPos = c.pos
			c.selectionLine = c.line
		}
		e.mouseDown = true
	}
	updateScrollAfterCursorMove(c.line, e)
	showCursor(c, e)
}

func draw(e *Editor) {
	ctx := e.ctx
	tb := e.textBuffer
	d := &e.dimensions
	c := &e.cursor
	colors := &e.colors

	// clear screen
	ctx.Set("fillStyle", colors.background)
	ctx.Call("fillRect", 0, 0, d.width, d.height)

	numLines := int(math.Ceil(float64(d.height)/float64(d.lineHeight)) + 2)   // plus one extra line for partial in view up top, one extra for bottom
	firstLine := int(math.Floor(float64(e.scroll)/float64(d.lineHeight)) - 1) // back one so as to partially show top line scrolling into view
	if firstLine < 0 {
		firstLine = 0
	}
	lastLine := firstLine + numLines - 1
	if lastLine > len(tb)-1 {
		lastLine = len(tb) - 1
	}

	selectionActive := drawSelection(ctx, tb, e.scroll, c, colors, d, firstLine, lastLine)
	drawText(ctx, e.defaultFont, tb, e.scroll, c.line, colors, d, firstLine, lastLine, !selectionActive)
	if c.shown && e.hasFocus {
		drawCursor(ctx, c, e.colors.cursor, d, e.scroll)
	}
}

func drawCursor(ctx *js.Object, c *Cursor, cursorColor string, d *Dimensions, scroll int) {
	ctx.Set("fillStyle", cursorColor)
	ctx.Call("fillRect",
		float64(d.lineOffsetX)+d.characterWidth*float64(c.pos),
		d.firstLineOffsetY+d.cursorOffsetY+d.lineHeight*c.line-scroll,
		d.cursorWidth,
		d.cursorHeight,
	)
}

func drawSelection(ctx *js.Object, textBuffer []string, scroll int, c *Cursor,
	colors *Colors, d *Dimensions, firstLine int, lastLine int) bool {

	drawDownwardsSelection := func(ctx *js.Object, textBuffer []string, d *Dimensions,
		topPos, topLine, bottomPos, bottomLine,
		firstLine, lastLine, adjustmentY int) {

		// draw top selection line
		ctx.Call("fillRect",
			float64(d.lineOffsetX)+d.characterWidth*float64(topPos),
			d.firstLineOffsetY+d.lineHeight*topLine-scroll+adjustmentY,
			float64(len(textBuffer[topLine])-topPos+1)*d.characterWidth,
			d.lineHeight,
		)
		// draw middle selection lines
		start := topLine + 1
		end := bottomLine
		if start <= lastLine && end >= firstLine {
			if start < firstLine {
				start = firstLine
			}
			if end > lastLine {
				end = lastLine + 1
			}
			y := d.firstLineOffsetY + d.lineHeight*start - scroll + adjustmentY
			for i := start; i < end; i++ {
				ctx.Call("fillRect",
					d.lineOffsetX,
					y,
					float64(len(textBuffer[i])+1)*d.characterWidth,
					d.lineHeight,
				)
				y += d.lineHeight
			}
		}
		// draw bottom selection line
		ctx.Call("fillRect",
			d.lineOffsetX,
			d.firstLineOffsetY+d.lineHeight*bottomLine-scroll+adjustmentY,
			float64(bottomPos)*d.characterWidth,
			d.lineHeight,
		)
	}

	const adjustmentY = -5

	ctx.Set("fillStyle", colors.selection)
	if c.line == c.selectionLine {
		if c.pos == c.selectionPos {
			// draw nothing
			return false
		} else if c.pos < c.selectionPos {
			ctx.Call("fillRect",
				float64(d.lineOffsetX)+d.characterWidth*float64(c.pos),
				d.firstLineOffsetY+d.lineHeight*c.line-scroll+adjustmentY,
				float64(c.selectionPos-c.pos)*d.characterWidth,
				d.lineHeight,
			)
		} else if c.pos > c.selectionPos {
			ctx.Call("fillRect",
				float64(d.lineOffsetX)+d.characterWidth*float64(c.selectionPos),
				d.firstLineOffsetY+d.lineHeight*c.line-scroll+adjustmentY,
				float64(c.pos-c.selectionPos)*d.characterWidth,
				d.lineHeight,
			)
		}
	} else if c.line < c.selectionLine {
		drawDownwardsSelection(ctx, textBuffer, d,
			c.pos, c.line, c.selectionPos, c.selectionLine,
			firstLine, lastLine, adjustmentY,
		)
	} else if c.line > c.selectionLine {
		drawDownwardsSelection(ctx, textBuffer, d,
			c.selectionPos, c.selectionLine, c.pos, c.line,
			firstLine, lastLine, adjustmentY,
		)
	}
	return true
}

func drawText(ctx *js.Object, font string, textBuffer []string, scroll int, cursorLine int, colors *Colors,
	d *Dimensions, firstLine int, lastLine int, drawCursorLineHighlight bool) {
	const lineHighlightOffsetY = 5

	// draw lines
	ctx.Set("fillStyle", colors.defaultText)
	ctx.Set("font", font)
	ctx.Set("textBaseline", "top")
	ctx.Set("textAlign", "start")
	startY := d.firstLineOffsetY - scroll + d.lineHeight*firstLine
	y := startY
	for i := firstLine; i <= lastLine; i++ {
		text := textBuffer[i]
		if drawCursorLineHighlight && i == cursorLine {
			ctx.Set("fillStyle", colors.lineHighlight)
			ctx.Call("fillRect", d.lineOffsetX, y-lineHighlightOffsetY, d.width, d.lineHeight)
			ctx.Set("fillStyle", colors.defaultText)
			ctx.Set("font", font)
		}
		ctx.Call("fillText", text, d.lineOffsetX, y)
		y += d.lineHeight
	}

	// draw line numbers
	ctx.Set("textAlign", "right")
	ctx.Set("fillStyle", colors.lineNumber)
	y = startY
	for i := firstLine; i <= lastLine; i++ {
		ctx.Call("fillText", i+1, d.lineOffsetX-d.numbersOffsetX, y)
		y += d.lineHeight
	}
}
