/* constants */
const editor = document.getElementById('editor');
const ctx = editor.getContext('2d');

const firstLineOffsetY = 6;
const lineOffsetX = 90;
const numbersOffsetX = 30;

const lineHeight = 26;  // todo: set proportional to font
const cursorWidth = 2;
const cursorHeight = 23;
const cursorOffsetY = -4;
const cursorBlinkTime = 620;

const numSpacesForTab = 4;
const space1 = ' ';
const space2 = '  ';
const space3 = '   ';
const space4 = '    ';

const defaultTextColor = '#ddd';
const lineNumberColor = '#887';
const backgroundColor = '#422';
const selectionColor = '#78992c';
const lineHighlightColor = '#733';
const defaultFont = "13pt Menlo, Monaco, 'Courier New', monospace";

const minScroll = 0;
const keyboardScrollSpeed = 0.40;  // pixels per second
const wheelScrollSpeed = 0.6;  // weight for wheel's event.deltaY

const mousemoveRateLimit = 30;  // milliseconds between updating selection drag

ctx.font = defaultFont;
const characterWidth = ctx.measureText('12345').width / 5;  // more than 1 character so to get average (not sure if result would be different)

/* global state */
var width = document.body.clientWidth;
var height = document.body.clientHeight;

var textBuffer = ["Hello, there.", "Why hello there.", "      General Kenobi."];
for (let i = 0; i < 50; i++) {
    textBuffer.push('asdf qwerty asdf qwerty jlcxoviu@#$ xkvjxlkvjef');
}
var cursorPos = 0;    // position within line, 0 is before first character, line.length is just after last character
var cursorLine = 0;
var cursorPreferredPos = 0;    // when moving cursor up and down, prefer this for new cursorPos 
                               // (based on its position when last set by left/right arrow or click cursor move)

// when selection pos/line matches cursor, no selection is active
var selectionPos = 0; 
var selectionLine = 0; 

var editorHasFocus = false;
var cursorShown = true;

var scrollUpKeyIsDown = false;
var scrollDownKeyIsDown = false;
var mouseDown = false;

var resizeTimeoutHandle;
var lastMouseTimestamp = 0;

var maxScroll = 0; // todo
var currentScroll = 0;


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
    updateMaxScroll();
}


function updateMaxScroll() {
    // subtract only half height so we can scroll a bit past last line
    maxScroll = lineHeight * textBuffer.length - (height / 2);
    if (maxScroll < 0) {
        maxScroll = 0;
    }
    if (currentScroll > maxScroll) {
        currentScroll = maxScroll;
    }
}

function drawText(ctx, firstLine, lastLine, drawCursorLineHighlight) {
    // draw lines
    ctx.fillStyle = defaultTextColor;
    ctx.font = defaultFont;
    ctx.textBaseline = 'top';
    ctx.textAlign = 'start';
    let startY = firstLineOffsetY - currentScroll + lineHeight * firstLine;
    let y = startY;
    for (let i = firstLine; i <= lastLine; i++) {
        let line = textBuffer[i];
        if (drawCursorLineHighlight && i === cursorLine) {
            const highlightOffsetY = 5;
            ctx.fillStyle = lineHighlightColor;
            ctx.fillRect(lineOffsetX, y - highlightOffsetY, width, lineHeight);
            ctx.fillStyle = defaultTextColor;
            ctx.font = defaultFont;
        }
        ctx.fillText(line, lineOffsetX, y);
        y += lineHeight;
    }

    // draw line numbers
    ctx.textAlign = 'right';
    ctx.fillStyle = lineNumberColor;
    y = startY;
    for (let i = firstLine; i <= lastLine; i++) {
        ctx.fillText(i + 1, lineOffsetX - numbersOffsetX, y);
        y += lineHeight;
    }
}

// delta time
function updateScroll(dt) {
    if (scrollUpKeyIsDown && scrollDownKeyIsDown) {
        // do nothing
    } else if (scrollUpKeyIsDown) {
        currentScroll -= (keyboardScrollSpeed * dt);
        if (currentScroll < 0) {
            currentScroll = 0;
            scrollUpKeyIsDown = false;
            scrollDownKeyIsDown = false;
        }
        draw(ctx);
    } else if (scrollDownKeyIsDown) {
        currentScroll += (keyboardScrollSpeed * dt);
        if (currentScroll > maxScroll) {
            currentScroll = maxScroll;
            scrollUpKeyIsDown = false;
            scrollDownKeyIsDown = false;
        }
        draw(ctx);
    }
}


function drawCursor(ctx) {
    if (!editorHasFocus) {
        return;
    }
    const cursorColor = 'rgba(255, 255, 0, 0.8)';
    ctx.fillStyle = cursorColor;
    ctx.fillRect(
        lineOffsetX + characterWidth * cursorPos, 
        firstLineOffsetY + cursorOffsetY + lineHeight * cursorLine - currentScroll,
        cursorWidth, 
        cursorHeight
    );
}

function drawSelection(ctx, firstLine, lastLine) {
    const adjustmentY = -5;
    ctx.fillStyle = selectionColor;
    if (cursorLine === selectionLine) {
        if (cursorPos === selectionPos) {
            // draw nothing
        } else if (cursorPos < selectionPos) {
            ctx.fillRect(
                lineOffsetX + characterWidth * cursorPos, 
                firstLineOffsetY + lineHeight * cursorLine - currentScroll + adjustmentY,
                (selectionPos - cursorPos) * characterWidth,
                lineHeight
            );
            return true;
        } else if (cursorPos > selectionPos) {
            ctx.fillRect(
                lineOffsetX + characterWidth * selectionPos, 
                firstLineOffsetY + lineHeight * cursorLine - currentScroll + adjustmentY,
                (cursorPos - selectionPos) * characterWidth,
                lineHeight
            );
            return true;
        }
    } else if (cursorLine < selectionLine) {
        // draw bottom selection line
        ctx.fillRect(
            lineOffsetX,
            firstLineOffsetY + lineHeight * selectionLine - currentScroll + adjustmentY,
            selectionPos * characterWidth,
            lineHeight
        );
        // draw middle selection lines
        let start = cursorLine + 1;
        let end = selectionLine;
        if (start <= lastLine && end >= firstLine) {
            if (start < firstLine) {
                start = firstLine;
            }
            if (end > lastLine) {
                end = lastLine + 1;
            }
            let y = firstLineOffsetY + lineHeight * start - currentScroll + adjustmentY;
            for (let i = start; i < end; i++) {
                ctx.fillRect(
                    lineOffsetX,
                    y,
                    (textBuffer[i].length + 1) * characterWidth,
                    lineHeight
                );  
                y += lineHeight;
            }
        } 
        // draw top selection line
        ctx.fillRect(
            lineOffsetX + characterWidth * cursorPos,
            firstLineOffsetY + lineHeight * cursorLine - currentScroll + adjustmentY,
            (textBuffer[cursorLine].length - cursorPos + 1) * characterWidth,
            lineHeight
        );
        return true;
    } else if (cursorLine > selectionLine) {
        // draw top selection line
        ctx.fillRect(
            lineOffsetX + characterWidth * selectionPos,
            firstLineOffsetY + lineHeight * selectionLine - currentScroll + adjustmentY,
            (textBuffer[selectionLine].length - selectionPos + 1) * characterWidth,
            lineHeight
        );
        // draw middle selection lines
        let start = selectionLine + 1;
        let end = cursorLine;
        if (start <= lastLine && end >= firstLine) {
            if (start < firstLine) {
                start = firstLine;
            }
            if (end > lastLine) {
                end = lastLine + 1;
            }
            let y = firstLineOffsetY + lineHeight * start - currentScroll + adjustmentY;
            for (let i = start; i < end; i++) {
                ctx.fillRect(
                    lineOffsetX,
                    y,
                    (textBuffer[i].length + 1) * characterWidth,
                    lineHeight
                );  
                y += lineHeight;
            }
        }
        // draw bottom selection line
        ctx.fillRect(
            lineOffsetX,
            firstLineOffsetY + lineHeight * cursorLine - currentScroll + adjustmentY,
            cursorPos * characterWidth,
            lineHeight
        );
        return true;
    }
    return false;
}


window.addEventListener("resize", function (evt) {
    window.clearTimeout(resizeTimeoutHandle);
    resizeTimeoutHandle = window.setTimeout(function () {
        sizeCanvas();
        draw(ctx);
    }, 150);
}, false);


// return false if area encompases no text (and so no delete)
function deleteSelection() {
    if (cursorPos === selectionPos && cursorLine === selectionLine) {
        return false;
    }
    let newPos;
    let newLine;
    if (cursorLine === selectionLine) {
        let line = textBuffer[cursorLine];
        newLine = cursorLine; 
        if (cursorPos < selectionPos) {
            newPos = cursorPos;
            textBuffer[cursorLine] = line.splice(cursorPos, selectionPos - cursorPos, '');
        } else {
            newPos = selectionPos;
            textBuffer[cursorLine] = line.splice(selectionPos, cursorPos - selectionPos, '');
        }
    } else if (cursorLine < selectionLine) {
        newLine = cursorLine;
        newPos = cursorPos;
        let leading = textBuffer[cursorLine].slice(0, cursorPos);
        let trailing = textBuffer[selectionLine].slice(selectionPos);
        textBuffer.splice(cursorLine + 1, selectionLine - cursorLine);  // discard lines
        textBuffer[cursorLine] = leading + trailing;
    } else if (cursorLine > selectionLine) {
        newLine = selectionLine;
        newPos = selectionPos;
        let leading = textBuffer[selectionLine].slice(0, selectionPos);
        let trailing = textBuffer[cursorLine].slice(cursorPos);
        textBuffer.splice(selectionLine + 1, cursorLine - selectionLine);  // discard lines
        textBuffer[selectionLine] = leading + trailing;
    }
    cursorPos = newPos;
    cursorLine = newLine;
    selectionPos = newPos;
    selectionLine = newLine;
    return true;
}

// assumes no newlines
function insertText(text) {
    let line = textBuffer[cursorLine];
    textBuffer[cursorLine] = line.splice(cursorPos, 0, text);
    cursorPos += text.length;
    cursorPreferredPos = cursorPos;
    selectionPos = cursorPos;
    selectionLine = cursorLine;
    updateScrollAfterCursorMove(cursorLine);
}

navigator.permissions.query({name:'clipboard-read'}).then(function(result) {
    if (result.state == 'granted') {
      console.log('clipboard-read permission granted');
    } else if (result.state == 'prompt') {
        console.log('clipboard-read permission prompt');
    }
    // Don't do anything if the permission was denied.
});

// accounts for newlines in the inserted text
function insertTextMultiline(text) {
    let lines = text.split('\n');
    if (lines.length === 1) {
        insertText(text);
        return;
    }
    let line = textBuffer[cursorLine];
    let preceding = line.slice(0, cursorPos);
    let following = line.slice(cursorPos);
    lines[0] = preceding + lines[0];
    cursorPos = lines[lines.length - 1].length;
    cursorPreferredPos = cursorPos;
    lines[lines.length - 1] = lines[lines.length - 1] + following;
    textBuffer.splice(cursorLine, 1, ...lines);
    cursorLine += lines.length - 1;
    selectionPos = cursorPos;
    selectionLine = cursorLine;
    updateMaxScroll();
    updateScrollAfterCursorMove(cursorLine);
}

function paste() {
    console.log('pasting');
    navigator.clipboard.readText().then(text => {
        // `text` contains the text read from the clipboard
        console.log('Text: ', text);
        insertTextMultiline(text);
        showCursor();
        draw(ctx);
    }).catch(err => {
        // maybe user didn't grant access to read from clipboard
        console.log('Something went wrong reading clipboard: ', err);
    });
}

function copy() {
    // todo
    navigator.clipboard.writeText("<empty clipboard>").then(function() {
        /* clipboard successfully set */
    }, function() {
    /* clipboard write failed */
    });
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
        updateMaxScroll();
    }
    selectionPos = cursorPos;
    selectionLine = cursorLine;
    updateScrollAfterCursorMove(cursorLine);
}

// returns new [pos, line], or null if cursor already at start of text
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

document.body.addEventListener('keyup', function (evt) {
    if (evt.key === 'Meta') {
        scrollUpKeyIsDown = false;
        scrollDownKeyIsDown = false;
    }
}, false);


// returns [min, max]
function minMaxScrollForLine(line) {
    const maxAdjustment = -6;
    const minAdjustment = 12;
    let max = line * lineHeight + firstLineOffsetY;  // max is top of the line
    let min = max - height + lineHeight;
    max += maxAdjustment;
    min += minAdjustment;
    if (max > maxScroll) {
        max = maxScroll;
    }
    if (min < 0) {
        min = 0;
    }
    return [min, max]
}

function updateScrollAfterCursorMove(line) {
    let [min, max] = minMaxScrollForLine(line);
    if (currentScroll < min) {
        currentScroll = min;
    } else if (currentScroll > max) {
        currentScroll = max;
    }
}

document.body.addEventListener('keydown', function (evt) {
    evt.stopPropagation();
    if (evt.key.length === 1) {
        if (evt.metaKey) {
            switch (evt.key) {
                case "r":
                case "R":
                    // reload page
                    return;
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
                    return;
                case "u":
                case "U":
                    evt.preventDefault();
                    if (scrollUpKeyIsDown || scrollDownKeyIsDown) {
                        scrollUpKeyIsDown = false;
                        scrollDownKeyIsDown = false;
                    } else {
                        scrollUpKeyIsDown = true;
                        scrollDownKeyIsDown = false;
                    }
                    return;
                case "i":
                case "I":
                    evt.preventDefault();
                    if (scrollUpKeyIsDown || scrollDownKeyIsDown) {
                        scrollUpKeyIsDown = false;
                        scrollDownKeyIsDown = false;
                    } else {
                        scrollUpKeyIsDown = false;
                        scrollDownKeyIsDown = true;
                    }
                    return;
                case "k":
                case "K":
                    evt.preventDefault();
                    deleteCurrentLine();
                    showCursor();
                    draw(ctx);
                    return;
                case "v":
                case "V":
                    evt.preventDefault();
                    paste();
                    showCursor();
                    draw(ctx);
                    return;
                default:
                    evt.preventDefault();
                    return;
            }
        }
        let code = evt.key.charCodeAt(0);
        if (code >= 32) {   // if not a control character
            deleteSelection();
            insertText(evt.key);
        } else {
            return;
        }
    } else {
        switch (evt.key) {
            // generally immitates VSCode behavior
            case "ArrowLeft":
                if (evt.metaKey) {
                    evt.preventDefault();
                    let line = textBuffer[cursorLine];
                    let trimmed = line.trimStart();
                    let newPos = line.length - trimmed.length; 
                    if (newPos === cursorPos) {
                        cursorPos = 0;
                    } else {
                        cursorPos = newPos;
                    }
                    if (!evt.shiftKey) {
                        selectionPos = cursorPos;
                    }
                    cursorPreferredPos = cursorPos;
                } else if (evt.altKey) {
                    let result = prevWhitespaceSkip(cursorPos, cursorLine, textBuffer);
                    if (result) {
                        cursorPos = result[0];
                        cursorLine = result[1];
                        cursorPreferredPos = cursorPos;
                        if (!evt.shiftKey) {
                            selectionPos = cursorPos;
                            selectionLine = cursorLine;
                        }
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
                    if (!evt.shiftKey) {
                        selectionPos = cursorPos;
                        selectionLine = cursorLine;
                    }
                }
                updateScrollAfterCursorMove(cursorLine);
                break;
            case "ArrowRight":
                if (evt.metaKey) {
                    evt.preventDefault();
                    cursorPos = textBuffer[cursorLine].length;
                    cursorPreferredPos = cursorPos;
                    if (!evt.shiftKey) {
                        selectionPos = cursorPos;
                    }
                } else if (evt.altKey) {
                    let result = nextWhitespaceSkip(cursorPos, cursorLine, textBuffer);
                    if (result) {
                        cursorPos = result[0];
                        cursorLine = result[1];
                        cursorPreferredPos = cursorPos;
                        if (!evt.shiftKey) {
                            selectionPos = cursorPos;
                            selectionLine = cursorLine;
                        }
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
                    if (!evt.shiftKey) {
                        selectionPos = cursorPos;
                        selectionLine = cursorLine;
                    }
                }
                updateScrollAfterCursorMove(cursorLine);
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
                    if (!evt.shiftKey) {
                        selectionPos = cursorPos;
                        selectionLine = cursorLine;
                    }
                }
                updateScrollAfterCursorMove(cursorLine);
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
                    if (!evt.shiftKey) {
                        selectionPos = cursorPos;
                        selectionLine = cursorLine;
                    }
                }
                updateScrollAfterCursorMove(cursorLine);
                break;
            case "Backspace":
                if (deleteSelection()) {
                    updateMaxScroll();
                } else if (cursorPos > 0 ) {
                    cursorPos--;
                    cursorPreferredPos = cursorPos;
                    let line = textBuffer[cursorLine];
                    textBuffer[cursorLine] = line.slice(0, cursorPos) + line.slice(cursorPos + 1);
                    selectionPos = cursorPos;
                } else if (cursorLine > 0) {
                    var prevLineIdx = cursorLine - 1;
                    var prevLine = textBuffer[prevLineIdx];
                    textBuffer[prevLineIdx] = prevLine + textBuffer[cursorLine];
                    textBuffer.splice(cursorLine, 1);
                    cursorPos = prevLine.length;
                    cursorLine = prevLineIdx;
                    selectionPos = cursorPos;
                    selectionLine = cursorLine;
                    updateMaxScroll();
                }
                updateScrollAfterCursorMove(cursorLine);
                break;
            case "Delete":
                if (deleteSelection()) {
                    updateMaxScroll();
                } else if (cursorPos < textBuffer[cursorLine].length) {
                    let line = textBuffer[cursorLine];
                    textBuffer[cursorLine] = line.slice(0, cursorPos) + line.slice(cursorPos + 1);
                    cursorPreferredPos = cursorPos;
                    selectionPos = cursorPos;
                    selectionLine = cursorLine;
                } else if (cursorLine < textBuffer.length - 1) {
                    textBuffer[cursorLine] = textBuffer[cursorLine] + textBuffer[cursorLine + 1];
                    textBuffer.splice(cursorLine + 1, 1);
                    selectionPos = cursorPos;
                    selectionLine = cursorLine;
                    updateMaxScroll();
                }
                updateScrollAfterCursorMove(cursorLine);
                break;
            case "Enter":
                deleteSelection();
                let line = textBuffer[cursorLine];
                textBuffer[cursorLine] = line.slice(0, cursorPos);
                cursorLine++;
                textBuffer.splice(cursorLine, 0, line.slice(cursorPos));
                cursorPos = 0;
                cursorPreferredPos = cursorPos;
                selectionPos = cursorPos;
                selectionLine = cursorLine;
                updateMaxScroll();
                updateScrollAfterCursorMove(cursorLine);
                break;
            case "Tab":
                evt.preventDefault();
                deleteSelection();
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


document.body.addEventListener('wheel', function (evt) {
    currentScroll += Math.round(evt.deltaY * wheelScrollSpeed);
    if (currentScroll < 0) {
        currentScroll = 0;
    } else if (currentScroll > maxScroll) {
        currentScroll = maxScroll;
    }
    draw(ctx);
}, false);


editor.addEventListener('mousedown', function (evt) {
    // todo: using clientX/Y for now; should get relative from canvas (to allow a canvas editor that isn't full page)
    const textCursorOffsetY = 7;   // want text cursor to select as if from center of the cursor, not top (there's no way to get the cursor's actual height, so we guess its half height)
    let newPos = Math.round((evt.clientX - lineOffsetX) / characterWidth);
    let newLine = Math.floor((evt.clientY - firstLineOffsetY + textCursorOffsetY + currentScroll) / lineHeight);
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
    if (!evt.shiftKey) {
        selectionPos = cursorPos;
        selectionLine = cursorLine;
    }
    mouseDown = true;
    updateScrollAfterCursorMove(cursorLine);
    showCursor();
    draw(ctx);
}, false);


editor.addEventListener('mouseup', function (evt) {
    mouseDown = false;
}, false);

editor.addEventListener('mouseout', function (evt) {
    mouseDown = false;
}, false);


editor.addEventListener('mousemove', function (evt) {
    if (mouseDown) {
        if ((evt.timeStamp - lastMouseTimestamp) > mousemoveRateLimit) {
            lastMouseTimestamp = evt.timeStamp;
            const textCursorOffsetY = 7;   // want text cursor to select as if from center of the cursor, not top (there's no way to get the cursor's actual height, so we guess its half height)
            let newPos = Math.round((evt.clientX - lineOffsetX) / characterWidth);
            let newLine = Math.floor((evt.clientY - firstLineOffsetY + textCursorOffsetY + currentScroll) / lineHeight);
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
            updateScrollAfterCursorMove(cursorLine);
            showCursor();
            draw(ctx);
        }
    }
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
    // clear screen
    ctx.fillStyle = backgroundColor;
    ctx.fillRect(0, 0, width, height);

    let numLines = Math.ceil(height / lineHeight) + 2;  // plus one extra line for partial in view up top, one extra for bottom
    let firstLine = Math.floor(currentScroll / lineHeight) - 1;  // back one so as to partially show top line scrolling into view
    if (firstLine < 0) {
        firstLine = 0;
    }
    let lastLine = firstLine + numLines - 1;
    if (lastLine > (textBuffer.length - 1)) {
        lastLine = textBuffer.length - 1;
    }

    let selectionActive = drawSelection(ctx, firstLine, lastLine);
    drawText(ctx, firstLine, lastLine, !selectionActive);
    if (cursorShown) {
        drawCursor(ctx);
    }
}

var priorTimestamp = 0;
function step(timestamp) {
    const maxDt = 400;
    let dt = timestamp - priorTimestamp;
    if (dt > maxDt) {
        dt = maxDt;
    }
    priorTimestamp = timestamp;
    updateScroll(dt);
    window.requestAnimationFrame(step);
}

sizeCanvas();
showCursor();
draw(ctx);
editor.focus();
window.requestAnimationFrame(step);
