import {draw, mouseInput, keyInput, updateMaxScroll, showCursor} from './logic.js';

const ele = document.getElementById('editor');
const ctx = ele.getContext('2d');
const defaultFont = "13pt Menlo, Monaco, 'Courier New', monospace"
ctx.font = defaultFont;

/* constants */
var editor = {
    ele: ele,
    ctx: ctx,
    defaultFont: defaultFont,
    wheelScrollSpeed: 0.6,   // weight for wheel's event.deltaY
    mousemoveRateLimit: 30,  // milliseconds between updating selection drag
    textBuffer: ["Hello, there.", "Why hello there.", "      General Kenobi."],
    mouseDown: false,
    maxScroll: 0,
    scroll: 0,
    hasFocus: false,
    dimensions: {
        width: document.body.clientWidth,
        height: document.body.clientHeight,
        firstLineOffsetY: 6,
        lineOffsetX: 90,
        numbersOffsetX: 30,
        lineHeight: 26,  // todo: set proportional to font
        cursorWidth: 2,
        cursorHeight: 23,
        cursorOffsetY: -4,
        numSpacesForTab: 4,
        characterWidth: ctx.measureText('12345').width / 5,  // more than 1 character so to get average 
                                                             // (not sure if result of measuring one character would be different)
    },
    colors: {
        defaultText: '#ddd',
        lineNumber: '#887',
        background: '#422',
        selection: '#78992c',
        lineHighlight: '#733',
        cursor: 'rgba(255, 255, 0, 0.8)',
    },
    cursor: {
        pos: 0,    // position within line, 0 is before first character, line.length is just after last character
        line: 0,
        preferredPos: 0,    // when moving cursor up and down, prefer this for new pos 
                               // (based on its position when last set by left/right arrow or click cursor move)
        selectionPos: 0,    // when selection pos/line matches cursor, no selection is active
        selectionLine: 0, 
        shown: true,
        blinkTime: 620,
    },
};

editor.colors = Object.freeze(editor.colors);


// for (let i = 0; i < 50; i++) {
//     editor.textBuffer.push('something otherthing');
// }

if (!String.prototype.splice) {
    /**
     * {JSDoc}
     *
     * The splice() method changes the content of a string by removing a range of
     * characters and/or adding new characters.
     *
     * @this {String}
     * @param {number} start Index at which to start changing the string.
     * @param {number} delCount An integer indicating the number of old chars to remove.
     * @param {string} newSubStr The String that is spliced in.
     * @return {string} A new string with the spliced substring.
     */
    String.prototype.splice = function(start, delCount, newSubStr) {
        return this.slice(0, start) + newSubStr + this.slice(start + Math.abs(delCount));
    };
}

var resizeTimeoutHandle;
window.addEventListener("resize", function (evt) {
    window.clearTimeout(resizeTimeoutHandle);
    resizeTimeoutHandle = window.setTimeout(function () {
        sizeCanvas(editor);
        updateMaxScroll(editor);
        draw(editor);
    }, 150);
}, false);

navigator.permissions.query({name:'clipboard-read'}).then(function(result) {
    if (result.state == 'granted') {
      console.log('clipboard-read permission granted');
    } else if (result.state == 'prompt') {
        console.log('clipboard-read permission prompt');
    }
});

document.body.addEventListener('keydown', function (evt) {
    evt.stopPropagation();
    keyInput(evt, editor);
    draw(editor);
}, false);

document.body.addEventListener('wheel', function (evt) {
    editor.scroll += Math.round(evt.deltaY * editor.wheelScrollSpeed);
    if (editor.scroll < 0) {
        editor.scroll = 0;
    } else if (editor.scroll > editor.maxScroll) {
        editor.scroll = editor.maxScroll;
    }
    draw(editor);
}, false);

editor.ele.addEventListener('mousedown', function (evt) {
    mouseInput(evt, true, editor);
    draw(editor);
}, false);

editor.ele.addEventListener('mouseup', function (evt) {
    editor.mouseDown = false;
}, false);

editor.ele.addEventListener('mouseout', function (evt) {
    editor.mouseDown = false;
}, false);

var lastMouseTimestamp = 0;
editor.ele.addEventListener('mousemove', function (evt) {
    if (editor.mouseDown) {
        if ((evt.timeStamp - lastMouseTimestamp) > editor.mousemoveRateLimit) {
            lastMouseTimestamp = evt.timeStamp;
            mouseInput(evt, false, editor);
            draw(editor);
        }
    }
}, false);

editor.ele.addEventListener('focus', function (evt) {
    editor.hasFocus = true;
    showCursor(editor.cursor, editor);
    draw(editor);
});

editor.ele.addEventListener('blur', function (evt) {
    editor.hasFocus = false;
    showCursor(editor.cursor, editor);
    draw(editor);
});

function sizeCanvas(editor) {
    // account for pixel ratio (avoids blurry text on high dpi screens)
    let width = editor.dimensions.width = document.body.clientWidth;
    let height = editor.dimensions.height = document.body.clientHeight;
    let ratio = window.devicePixelRatio;
    let ele = editor.ele;
    ele.setAttribute('width', width * ratio);
    ele.setAttribute('height', height * ratio);
    ele.style.width = width + 'px';
    ele.style.height = height + 'px';
    ctx.scale(ratio, ratio);
}

sizeCanvas(editor);
updateMaxScroll(editor);
showCursor(editor.cursor, editor);
draw(editor);
editor.ele.focus();
