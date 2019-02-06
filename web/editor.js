var editor = document.getElementById('editor');
var ctx = editor.getContext('2d');

var width = document.body.clientWidth;
var height = document.body.clientHeight;

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

function sizeCanvas() {
    width = document.body.clientWidth;
    height = document.body.clientHeight;
    // account for pixel ratio (avoids blurry text on high dpi screens)
    // todo: account for browsers with no devicePixelRatio set
    let ratio = window.devicePixelRatio;
    editor.setAttribute('width', width * ratio);
    editor.setAttribute('height', height * ratio);
    editor.style.width = width + 'px';
    editor.style.height = height + 'px';
    ctx.scale(ratio, ratio);
}

const defaultTextColor = '#ddd';
const defaultFont = "14pt Menlo, Monaco, 'Courier New', monospace";

function drawText(ctx) {
    ctx.fillStyle = '#422';
    ctx.fillRect(0, 0, width, height);
    ctx.fillStyle = defaultTextColor;
    ctx.font = defaultFont;
    ctx.textBaseline = 'top';
    ctx.fillText(textBuffer, 10, 10);
}

const firstLineOffsetY = 10;
const lineOffsetX = 10;

const lineHeight = 40;
const cursorWidth = 2;
const cursorHeight = 23;
const cursorOffsetY = -4;
var cursorShown = true;

ctx.font = defaultFont;
const characterWidth = ctx.measureText('12345').width / 5;  // more than 1 character so to get average (not sure if result would be different)
console.log(characterWidth);


function drawCursor(ctx) {
    const cursorColor = 'rgba(255, 255, 0, 0.8)';
    ctx.fillStyle = cursorColor;
    ctx.fillRect(lineOffsetX + characterWidth * cursorPos, firstLineOffsetY + cursorOffsetY, cursorWidth, cursorHeight);
}

var textBuffer = "Hello, there. width: " + width + " height: " + height + " something something something something";
var cursorPos = 0;    // position within line, 0 is before first character, line.length is just after last character
var cursorLine = 0;

var timeoutHandle;
window.addEventListener("resize", function (evt) {
    window.clearTimeout(timeoutHandle);
    timeoutHandle = window.setTimeout(function () {
        sizeCanvas();
        draw(ctx);
    }, 150);
}, false);

const numSpacesForTab = 4;
const space1 = ' ';
const space2 = '  ';
const space3 = '   ';
const space4 = '    ';

document.body.addEventListener('keydown', function (evt) {
    console.log(evt);
    let redraw = false;
    evt.stopPropagation();
    if (evt.key.length === 1) {
        let code = evt.key.charCodeAt(0);
        if (code >= 32) {   // if not a control character
            if (evt.metaKey) {
                switch (evt.key) {
                    case "s":
                    case "S":
                        evt.preventDefault();
                        return;
                    case "-":
                    case "=":
                    case "0":
                        return;
                }
            }
            textBuffer = textBuffer.splice(cursorPos, 0, evt.key);
            cursorPos++;
            showCursor();
            redraw = true;
        }
    } else {
        switch (evt.key) {
            case "ArrowLeft":
                if (cursorPos > 0) {
                    cursorPos--
                    showCursor();
                    redraw = true;
                }
                break;
            case "ArrowRight":
                if (cursorPos < textBuffer.length) {
                    cursorPos++
                    showCursor();
                    redraw = true;
                }
                break;
            case "Backspace":
                if (cursorPos > 0 ) {
                    cursorPos--;
                    textBuffer = textBuffer.slice(0, cursorPos) + textBuffer.slice(cursorPos + 1);
                    showCursor();
                    redraw = true;
                }
                break;
            case "Delete":
                if (cursorPos < textBuffer.length) {
                    textBuffer = textBuffer.slice(0, cursorPos) + textBuffer.slice(cursorPos + 1);
                    showCursor();
                    redraw = true;
                }
                break;
            case "Enter":
                break;
            case "Tab":
                evt.preventDefault();
                let numSpaces = numSpacesForTab - (cursorPos % numSpacesForTab);
                let insert;
                switch (numSpaces) {
                    case 1:
                        insert = space1;
                        break;
                    case 2:
                        insert = space2;
                        break;
                    case 3:
                        insert = space3;
                        break;
                    case 4:
                        insert = space4;
                        break;
                }
                textBuffer = textBuffer.splice(cursorPos, 0, insert);
                cursorPos += numSpaces;
                showCursor();
                redraw = true;
                break;
        }
    }
    if (redraw) {
        draw(ctx);
    }
}, false);

editor.addEventListener('mousedown', function (evt) {
    // todo: using clientX/Y for now; should get relative from canvas (to allow a canvas editor that isn't full page)
    console.log(evt);
    cursorPos = Math.floor((evt.clientX - lineOffsetX) / characterWidth);
    showCursor();
    draw(ctx);
}, false);

const cursorBlinkTime = 620;
var cursorBlinkTimeoutHandle;
function toggleCursor() {
    if (cursorShown) {
        cursorShown = false;
    } else {
        cursorShown = true;
    }
    draw(ctx);
    cursorBlinkTimeoutHandle = window.setTimeout(toggleCursor, cursorBlinkTime);
}

function showCursor() {
    window.clearTimeout(cursorBlinkTimeoutHandle);
    cursorShown = true;
    cursorBlinkTimeoutHandle = window.setTimeout(toggleCursor, cursorBlinkTime);
}

function draw(ctx) {
    drawText(ctx);
    if (cursorShown) {
        drawCursor(ctx);
    }
}

sizeCanvas();
showCursor();
draw(ctx);