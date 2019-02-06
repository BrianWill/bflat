var editor = document.getElementById('editor');
var ctx = editor.getContext('2d');

var width = document.body.clientWidth;
var height = document.body.clientHeight;

var editorHasFocus = false;

const firstLineOffsetY = 10;
const lineOffsetX = 90;
const numbersOffsetX = 30;

const lineHeight = 26;  // todo: set proportional to font
const cursorWidth = 2;
const cursorHeight = 23;
const cursorOffsetY = -4;
var cursorShown = true;
const cursorBlinkTime = 620;

var textBuffer = ["Hello, there.", "Why hello there.", "      General Kenobi."];
var cursorPos = 0;    // position within line, 0 is before first character, line.length is just after last character
var cursorPreferredPos = 0;    // when moving cursor up and down, prefer this for new cursorPos
var cursorLine = 0;


const numSpacesForTab = 4;
const space1 = ' ';
const space2 = '  ';
const space3 = '   ';
const space4 = '    ';


const defaultTextColor = '#ddd';
const lineNumberColor = '#999';
const backgroundColor = '#422';
const defaultFont = "13pt Menlo, Monaco, 'Courier New', monospace";



ctx.font = defaultFont;
const characterWidth = ctx.measureText('12345').width / 5;  // more than 1 character so to get average (not sure if result would be different)


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


function drawText(ctx) {
    // clear screen
    ctx.fillStyle = backgroundColor;
    ctx.fillRect(0, 0, width, height);

    // draw lines
    ctx.fillStyle = defaultTextColor;
    ctx.font = defaultFont;
    ctx.textBaseline = 'top';
    ctx.textAlign = 'start';
    let y = firstLineOffsetY;
    for (let i = 0; i < textBuffer.length; i++) {
        let line = textBuffer[i];
        ctx.fillText(line, lineOffsetX, y);
        y += lineHeight;
    }

    // draw line numbers
    ctx.textAlign = 'right';
    ctx.fillStyle = lineNumberColor;
    y = firstLineOffsetY;
    for (let i = 0; i < textBuffer.length; i++) {
        ctx.fillText(i + 1, lineOffsetX - numbersOffsetX, y);
        y += lineHeight;
    }
}


function drawCursor(ctx) {
    if (!editorHasFocus) {
        return;
    }
    const cursorColor = 'rgba(255, 255, 0, 0.8)';
    ctx.fillStyle = cursorColor;
    ctx.fillRect(lineOffsetX + characterWidth * cursorPos, 
        firstLineOffsetY + cursorOffsetY + lineHeight * cursorLine,
        cursorWidth, 
        cursorHeight
    );
}



var timeoutHandle;
window.addEventListener("resize", function (evt) {
    window.clearTimeout(timeoutHandle);
    timeoutHandle = window.setTimeout(function () {
        sizeCanvas();
        draw(ctx);
    }, 150);
}, false);



// assumes no newlines
function insertText(text) {
    let line = textBuffer[cursorLine];
    textBuffer[cursorLine] = line.splice(cursorPos, 0, text);
    cursorPos += text.length;
    cursorPreferredPos = cursorPos;
}

function deleteCurrentLine() {
    if (textBuffer.length === 1) {
        textBuffer = [''];
        cursorPos = 0;
        cursorPreferredPos = 0;
        cursorLine = 0;
    } else {
        textBuffer.splice(cursorLine, 1);
        if (cursorLine > textBuffer.length - 1) {
            cursorLine = textBuffer.length - 1;
        }
        let newLineLength = textBuffer[cursorLine].length;
        if (cursorPreferredPos <= newLineLength) {
            cursorPos = cursorPreferredPos;
        } else {
            cursorPos = newLineLength;
            cursorPreferredPos = newLineLength;
        }
    }
}

function prevWhitespaceSkip(cursorPos, cursorLine, textBuffer) {
    let line = textBuffer[cursorLine];
    if (cursorPos === 0) {
        if (cursorLine === 0) {
            return null;
        }
        cursorLine--;
        line = textBuffer[cursorLine];
        cursorPos = line.length;
        if (cursorPos === 0) {
            return [cursorPos, cursorLine];
        }
    }
    let preceding = line.slice(0, cursorPos);
    let trimmed = preceding.trimEnd();
    let firstSpace = trimmed.lastIndexOf(' ');
    if (firstSpace === -1) {
        return [0, cursorLine];
    } else {
        return [firstSpace + 1,  cursorLine];
    }
}

// return [newPos, newLine], or null if cursor already at end
function nextWhitespaceSkip(cursorPos, cursorLine, textBuffer) {
    let line = textBuffer[cursorLine];
    if (cursorPos === line.length) {
        if (cursorLine === textBuffer.length - 1) {
            return null;
        }
        cursorLine++;
        cursorPos = 0;
        line = textBuffer[cursorLine];
        if (cursorPos === line.length) {
            return [cursorPos, cursorLine];
        }
    }
    let remaining = line.slice(cursorPos);
    let trimmed = remaining.trimStart();
    let firstSpace = trimmed.indexOf(' ');
    if (firstSpace === -1) {
        return [line.length, cursorLine];
    } else {
        return [cursorPos + (remaining.length - trimmed.length) + firstSpace,  cursorLine];
    }
}

document.body.addEventListener('keydown', function (evt) {
    console.log(evt);
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
                    case "o":
                    case "O":
                        evt.preventDefault();
                        return
                    case "k":
                    case "K":
                        deleteCurrentLine();
                        break;
                }
            } else {
                insertText(evt.key);
            }
        } else {
            return;
        }
    } else {
        switch (evt.key) {
            // generally immitates VSCode behavior
            case "ArrowLeft":
                if (evt.metaKey) {
                    let line = textBuffer[cursorLine];
                    let trimmed = line.trimStart();
                    let newPos = line.length - trimmed.length; 
                    if (newPos === cursorPos) {
                        cursorPos = 0;
                    } else {
                        cursorPos = newPos;
                    }
                    cursorPreferredPos = cursorPos;
                } else if (evt.altKey) {
                    let result = prevWhitespaceSkip(cursorPos, cursorLine, textBuffer);
                    if (result) {
                        cursorPos = result[0];
                        cursorLine = result[1];
                        cursorPreferredPos = cursorPos;
                    }
                } else {
                    if (cursorPos === 0) {
                        if (cursorLine === 0) {
                            return;
                        }
                        cursorLine--;
                        cursorPos = textBuffer[cursorLine].length;
                        cursorPreferredPos = cursorPos;
                    } else {
                        cursorPos--;
                        cursorPreferredPos = cursorPos;
                    }
                }
                break;
            case "ArrowRight":
                if (evt.metaKey) {
                    cursorPos = textBuffer[cursorLine].length;
                    cursorPreferredPos = cursorPos;
                } else if (evt.altKey) {
                    let result = nextWhitespaceSkip(cursorPos, cursorLine, textBuffer);
                    if (result) {
                        cursorPos = result[0];
                        cursorLine = result[1];
                        cursorPreferredPos = cursorPos;
                    }
                } else {
                    if (cursorPos === textBuffer[cursorLine].length) {
                        if (cursorLine === textBuffer.length - 1) {
                            return;
                        }
                        cursorLine++;
                        cursorPos = 0;
                        cursorPreferredPos = 0;
                    } else {
                        cursorPos++;
                        cursorPreferredPos = cursorPos;
                    }
                }
                break;
            case "ArrowUp":
                if (cursorLine > 0) {
                    cursorLine--;
                    let newLineLength = textBuffer[cursorLine].length;
                    if (cursorPreferredPos <= newLineLength) {
                        cursorPos = cursorPreferredPos;
                    } else {
                        cursorPos = newLineLength;
                    }
                }
                break;
            case "ArrowDown":
                if (cursorLine < textBuffer.length - 1) {
                    cursorLine++;
                    let newLineLength = textBuffer[cursorLine].length;
                    if (cursorPreferredPos <= newLineLength) {
                        cursorPos = cursorPreferredPos;
                    } else {
                        cursorPos = newLineLength;
                    }
                }
                break;
            case "Backspace":
                if (cursorPos > 0 ) {
                    cursorPos--;
                    cursorPreferredPos = cursorPos;
                    let line = textBuffer[cursorLine];
                    textBuffer[cursorLine] = line.slice(0, cursorPos) + line.slice(cursorPos + 1);
                } else if (cursorLine > 0) {
                    var prevLineIdx = cursorLine - 1;
                    var prevLine = textBuffer[prevLineIdx];
                    textBuffer[prevLineIdx] = prevLine + textBuffer[cursorLine];
                    textBuffer.splice(cursorLine, 1);
                    cursorPos = prevLine.length;
                    cursorLine = prevLineIdx;
                }
                break;
            case "Delete":
                if (cursorPos < textBuffer[cursorLine].length) {
                    let line = textBuffer[cursorLine];
                    textBuffer[cursorLine] = line.slice(0, cursorPos) + line.slice(cursorPos + 1);
                    cursorPreferredPos = cursorPos;
                } else if (cursorLine < textBuffer.length - 1) {
                    textBuffer[cursorLine] = textBuffer[cursorLine] + textBuffer[cursorLine + 1];
                    textBuffer.splice(cursorLine + 1, 1);
                }
                break;
            case "Enter":
                let line = textBuffer[cursorLine];
                textBuffer[cursorLine] = line.slice(0, cursorPos);
                cursorLine++;
                textBuffer.splice(cursorLine, 0, line.slice(cursorPos));
                cursorPos = 0;
                cursorPreferredPos = cursorPos;
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
                insertText(insert);
                break;
            default:
                return;
        }
    }
    showCursor();
    draw(ctx);
}, false);

editor.addEventListener('mousedown', function (evt) {
    // todo: using clientX/Y for now; should get relative from canvas (to allow a canvas editor that isn't full page)
    const cursorOffsetY = 7;   // want text cursor to select as if from center of the cursor, not top (there's no way to get the cursor's actual height, so we guess its half height)
    let newPos = Math.round((evt.clientX - lineOffsetX) / characterWidth);
    let newLine = Math.floor((evt.clientY - firstLineOffsetY + cursorOffsetY) / lineHeight);
    if (newPos < 0) {
        newPos = 0;
    }
    if (newLine < 0) {
        newLine = 0;
    }
    if (newLine > textBuffer.length - 1) {
        newLine = textBuffer.length - 1;
    }
    if (newPos > textBuffer[newLine].length) {
        newPos = textBuffer[newLine].length;
    }
    cursorPos = newPos;
    cursorPreferredPos = newPos;
    cursorLine = newLine;
    showCursor();
    draw(ctx);
}, false);

editor.addEventListener('focus', function (evt) {
    editorHasFocus = true;
    showCursor();
    draw(ctx);
});

editor.addEventListener('blur', function (evt) {
    editorHasFocus = false;
    showCursor();
    draw(ctx);
});


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
editor.focus();