package main

import (
	"math"
	"strconv"

	"github.com/gopherjs/gopherjs/js"
)

type Editor struct {
	ele                *js.Object
	ctx                *js.Object
	defaultFont        string
	wheelScrollSpeed   float64
	mousemoveRateLimit float64
	textBuffer         []string
	mouseDown          bool
	scroll             int
	maxScroll          int
	hasFocus           bool
	dimensions         Dimensions
	cursor             Cursor
	colors             Colors

	// };
}

type Dimensions struct {
	width            int
	height           int
	firstLineOffsetY int
	lineOffsetX      int
	numbersOffsetX   int
	lineHeight       int
	cursorWidth      int
	cursorHeight     int
	cursorOffsetY    int
	numSpacesForTab  int
	characterWidth   float64
}

type Cursor struct {
	pos          int // position within line, 0 is before first character, line.length is just after last character
	line         int
	preferredPos int // when moving cursor up and down, prefer this for new pos
	//                                // (based on its position when last set by left/right arrow or click cursor move)
	selectionPos  int // when selection pos/line matches cursor, no selection is active
	shown         bool
	blinkTime     int
	selectionLine int
}

type Colors struct {
	defaultText   string
	lineNumber    string
	background    string
	selection     string
	lineHighlight string
	cursor        string
}

func Abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func spliceStr(s string, start int, delCount int, insertStr string) string {
	return s[0:start] + insertStr + s[start+Abs(delCount):]
}

func sizeCanvas(e *Editor, body *js.Object) {
	// account for pixel ratio (avoids blurry text on high dpi screens)
	width := body.Get("clientWidth").Int()
	height := body.Get("clientHeight").Int()
	e.dimensions.width = width
	e.dimensions.height = height
	ratio := js.Global.Get("devicePixelRatio").Float()
	e.ele.Call("setAttribute", "width", int(float64(width)*ratio))
	e.ele.Call("setAttribute", "height", int(float64(height)*ratio))
	style := e.ele.Get("style")
	style.Set("width", strconv.Itoa(width)+"px")
	style.Set("height", strconv.Itoa(height)+"px")
	e.ctx.Call("scale", ratio, ratio)
}

var window = js.Global.Get("window")

func main() {
	document := js.Global.Get("document")
	body := document.Get("body")
	ele := document.Call("getElementById", "editor")
	ctx := ele.Call("getContext", "2d")
	const defaultFont = "13pt Menlo, Monaco, 'Courier New', monospace"

	editor := &Editor{
		ele:                ele,
		ctx:                ctx,
		wheelScrollSpeed:   0.6,
		mousemoveRateLimit: 30,
		textBuffer:         []string{"hi there.", "why hello there", "     General Kenobi"},
		mouseDown:          false,
		scroll:             0,
		maxScroll:          0,
		hasFocus:           false,
	}
	editor.dimensions = Dimensions{
		width:            body.Get("width").Int(),
		height:           body.Get("height").Int(),
		firstLineOffsetY: 6,
		lineOffsetX:      90,
		numbersOffsetX:   30,
		lineHeight:       26, // todo: set proportional to font
		cursorWidth:      2,
		cursorHeight:     23,
		cursorOffsetY:    -4,
		numSpacesForTab:  4,
		characterWidth:   ctx.Call("measureText", "12345").Get("width").Float() / 5,
		// more than 1 character so to get average
		// (not sure if result of measuring one character would be different)
	}
	editor.cursor = Cursor{
		pos:          0, // position within line, 0 is before first character, line.length is just after last character
		line:         0,
		preferredPos: 0, // when moving cursor up and down, prefer this for new pos
		// (based on its position when last set by left/right arrow or click cursor move)
		selectionPos:  0, // when selection pos/line matches cursor, no selection is active
		selectionLine: 0,
		shown:         true,
		blinkTime:     620,
	}
	editor.colors = Colors{
		defaultText:   "#ddd",
		lineNumber:    "#887",
		background:    "#422",
		selection:     "#78992c",
		lineHighlight: "#733",
		cursor:        "rgba(255, 255, 0, 0.8)",
	}
	js.Global.Set("editor", editor) // for debug

	var resizeTimeoutHandle int
	window.Call("addEventListener", "resize", func(event *js.Object) {
		window.Call("clearTimeout", resizeTimeoutHandle)
		resizeTimeoutHandle = window.Call("setTimeout", func() {
			sizeCanvas(editor, body)
			updateMaxScroll(editor, &editor.dimensions)
			draw(editor)
		}, 150).Int()
	}, false)

	js.Global.Get("navigator").Get("permissions").Call("query",
		map[string]interface{}{"name": "clipboard-read"},
	).Call("then", func(result *js.Object) {
		state := result.Get("state").String()
		if state == "granted" {
			println("clipboard-read permission granted")
		} else if state == "prompt" {
			println("clipboard-read permission prompt")
		}
	})

	body.Call("addEventListener", "keydown", func(evt *js.Object) {
		evt.Call("stopPropagation")
		keyInput(evt, editor)
		draw(editor)
	}, false)

	body.Call("addEventListener", "wheel", func(evt *js.Object) {
		deltaY := evt.Get("deltaY").Float()
		editor.scroll += int(math.Round(deltaY * editor.wheelScrollSpeed))
		if editor.scroll < 0 {
			editor.scroll = 0
		} else if editor.scroll > editor.maxScroll {
			editor.scroll = editor.maxScroll
		}
		draw(editor)
	}, false)

	editor.ele.Call("addEventListener", "mousedown", func(evt *js.Object) {
		mouseInput(evt, true, editor)
		draw(editor)
	}, false)

	editor.ele.Call("addEventListener", "mouseup", func(evt *js.Object) {
		editor.mouseDown = false
	}, false)

	editor.ele.Call("addEventListener", "mouseout", func(evt *js.Object) {
		editor.mouseDown = false
	}, false)

	var lastMouseTimestamp float64 = 0
	editor.ele.Call("addEventListener", "mousemove", func(evt *js.Object) {
		if editor.mouseDown {
			timeStamp := evt.Get("timeStamp").Float()
			if (timeStamp - lastMouseTimestamp) > editor.mousemoveRateLimit {
				lastMouseTimestamp = timeStamp
				mouseInput(evt, false, editor)
				draw(editor)
			}
		}
	}, false)

	editor.ele.Call("addEventListener", "focus", func(evt *js.Object) {
		editor.hasFocus = true
		showCursor(&editor.cursor, editor)
		draw(editor)
	}, false)

	editor.ele.Call("addEventListener", "blur", func(evt *js.Object) {
		editor.hasFocus = false
		showCursor(&editor.cursor, editor)
		draw(editor)
	}, false)

	sizeCanvas(editor, body)
	updateMaxScroll(editor, &editor.dimensions)
	showCursor(&editor.cursor, editor)
	draw(editor)
	editor.ele.Call("focus")

}
