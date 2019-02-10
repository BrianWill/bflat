var exports = {};
export default exports;

// assumes no newlines
function insertText(text, textBuffer, cursor, editor) {
    let c = cursor;
    let line = textBuffer[c.line];
    textBuffer[c.line] = line.splice(c.pos, 0, text);
    c.pos += text.length;
    c.preferredPos = c.pos;
    c.selectionPos = c.pos;
    c.selectionLine = c.line;
    updateScrollAfterCursorMove(c.line, editor);
}

function updateScrollAfterCursorMove(line, editor) {
    let [min, max] = minMaxScrollForLine(line, editor);
    if (editor.scroll < min) {
        editor.scroll = min;
    } else if (editor.scroll > max) {
        editor.scroll = max;
    }

    // returns [min, max]
    function minMaxScrollForLine(line, editor) {
        let d = editor.dimensions;
        const maxAdjustment = -6;
        const minAdjustment = 12;
        let max = line * d.lineHeight + d.firstLineOffsetY;  // max is top of the line
        let min = max - d.height + d.lineHeight;
        max += maxAdjustment;
        min += minAdjustment;
        if (max > editor.maxScroll) {
            max = editor.maxScroll;
        }
        if (min < 0) {
            min = 0;
        }
        return [min, max];
    }
}

// return false if area encompases no text (and so no delete is performed)
// updates cursor and selection states accordingly
function deleteSelection(cursor, textBuffer) {
    let c = cursor;
    let tb = textBuffer;
    if (c.pos === c.selectionPos && c.line === c.selectionLine) {
        return false;
    }
    let pos;
    let line;
    if (c.line === c.selectionLine) {
        let text = tb[c.line];
        line = c.line; 
        if (c.pos < c.selectionPos) {
            pos = c.pos;
            tb[c.line] = text.splice(c.pos, c.selectionPos - c.pos, '');
        } else {
            pos = c.selectionPos;
            tb[c.line] = text.splice(c.selectionPos, c.pos - c.selectionPos, '');
        }
    } else if (c.line < c.selectionLine) {
        line = c.line;
        pos = c.pos;
        let leading = tb[c.line].slice(0, c.pos);
        let trailing = tb[c.selectionLine].slice(c.selectionPos);
        tb.splice(c.line, c.selectionLine - c.line);  // discard lines
        tb[c.line] = leading + trailing;
    } else if (c.line > c.selectionLine) {
        line = c.selectionLine;
        pos = c.selectionPos;
        let leading = tb[c.selectionLine].slice(0, c.selectionPos);
        let trailing = tb[c.line].slice(c.pos);
        tb.splice(c.selectionLine, c.line - c.selectionLine);  // discard lines
        tb[c.selectionLine] = leading + trailing;
    }
    c.pos = pos;
    c.line = line;
    c.selectionPos = pos;
    c.selectionLine = line;
    return true;
}

export function keyInput(evt, editor) {
    if (evt.key.length === 1) {
        if (evt.metaKey) {
            hotkeyInput(evt, editor);
            return;
        }
        let code = evt.key.charCodeAt(0);
        if (code >= 32) {   // if not a control character
            deleteSelection(editor.cursor, editor.textBuffer);
            insertText(evt.key, editor.textBuffer, editor.cursor, editor);
        } else {
            return;
        }
    } else {
        cursorKeyInput(evt, editor, editor.cursor, editor.textBuffer);
    }
    showCursor(editor.cursor, editor);
}

export function updateMaxScroll(editor) {
    // subtract only half height so we can scroll a bit past last line
    let e = editor;
    let d = editor.dimensions;
    e.maxScroll = d.lineHeight * e.textBuffer.length - (d.height / 2);
    if (e.maxScroll < 0) {
        e.maxScroll = 0;
    }
    if (e.scroll > e.maxScroll) {
        e.scroll = e.maxScroll;
    }
}

var cursorBlinkTimeoutHandle;
export function showCursor(cursor, editor) {
    window.clearTimeout(cursorBlinkTimeoutHandle);
    cursor.shown = true;
    cursorBlinkTimeoutHandle = window.setTimeout(toggleCursor, cursor.blinkTime);

    function toggleCursor() {
        if (cursor.shown) {
            cursor.shown = false;
        } else {
            cursor.shown = true;
        }
        draw(editor);
        cursorBlinkTimeoutHandle = window.setTimeout(toggleCursor, cursor.blinkTime);
    }
}

function hotkeyInput(evt, editor) {
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
            // adjust zoom
            return;
        case "o":
        case "O":
            evt.preventDefault();
            return;
        case "u":
        case "U":
            evt.preventDefault();
            return;
        case "i":
        case "I":
            evt.preventDefault();
            return;
        case "k":
        case "K":
            evt.preventDefault();
            deleteCurrentLine(editor.textBuffer, editor.cursor, editor);
            showCursor(editor.cursor, editor);
            draw(editor);
            return;
        case "c":
        case "C":
            evt.preventDefault();
            copy(editor.textBuffer, editor.cursor);
            return;
        case "v":
        case "V":
            evt.preventDefault();
            paste(editor);
            showCursor(editor.cursor, editor);
            draw(editor);
            return;
        default:
            evt.preventDefault();
            return;
    }

    function deleteCurrentLine(textBuffer, cursor, editor) {
        let c = cursor;
        if (textBuffer.length === 1) {
            editor.textBuffer = [''];
            c.pos = 0;
            c.preferredPos = 0;
            c.line = 0;
        } else {
            textBuffer.splice(c.line, 1);
            if (c.line > textBuffer.length - 1) {
                c.line = textBuffer.length - 1;
            }
            let newLineLength = textBuffer[c.line].length;
            if (c.preferredPos <= newLineLength) {
                c.pos = c.preferredPos;
            } else {
                c.pos = newLineLength;
                c.preferredPos = newLineLength;
            }
            updateMaxScroll(editor);
        }
        c.selectionPos = c.pos;
        c.selectionLine = c.line;
        updateScrollAfterCursorMove(c.line, editor);
    }


    function paste(editor) {
        console.log('pasting');
        navigator.clipboard.readText().then(text => {
            // `text` contains the text read from the clipboard
            console.log('Text: ', text);
            insertTextMultiline(text, editor.textBuffer, editor.cursor, editor);
            showCursor(editor.cursor, editor);
            draw(editor);
        }).catch(err => {
            // maybe user didn't grant access to read from clipboard
            console.log('Something went wrong reading clipboard: ', err);
        });

        // accounts for newlines in the inserted text
        function insertTextMultiline(text, textBuffer, cursor, editor) {
            let lines = text.split('\n');
            if (lines.length === 1) {
                insertText(text, textBuffer, cursor, editor);
                return;
            }
            let c = cursor;
            let line = textBuffer[c.line];
            let preceding = line.slice(0, c.pos);
            let following = line.slice(c.pos);
            lines[0] = preceding + lines[0];
            c.pos = lines[lines.length - 1].length;
            c.preferredPos = c.pos;
            lines[lines.length - 1] = lines[lines.length - 1] + following;
            textBuffer.splice(c.line, 1, ...lines);
            c.line += lines.length - 1;
            c.selectionPos = c.pos;
            c.selectionLine = c.line;
            updateMaxScroll(editor);
            updateScrollAfterCursorMove(c.line, editor);
        }
    }

    function copy(textBuffer, cursor) {
        let selection = getSelection(textBuffer, cursor);
        if (selection) {
            navigator.clipboard.writeText(selection).then(function(msg) {
                console.log('successfully wrote to clipboard: ', msg); 
            }, function() {
                console.log('failed to write to clipboard'); 
            });
        }

        // returns text in selection area (or empty string if no selected text) 
        function getSelection(textBuffer, cursor) {
            let c = cursor;
            if (c.pos === c.selectionPos && c.line === c.selectionLine) {
                return '';
            }
            if (c.line === c.selectionLine) {
                let line = textBuffer[c.line];
                return (c.pos < c.selectionPos) ? line.slice(c.pos, c.selectionPos) :
                    line.slice(c.selectionPos, c.pos);
            } else {
                let startPos = c.pos;
                let startLine = c.line;
                let endPos = c.selectionPos;
                let endLine = c.selectionLine;
                if (c.line > c.selectionLine) {
                    startPos = c.selectionPos;
                    startLine = c.selectionLine;
                    endPos = c.pos;
                    endLine = c.line;
                }
                let s = textBuffer[startLine].slice(startPos);
                for (let i = startLine + 1; i < endLine; i++) {
                    s += '\n' + textBuffer[i];
                }
                s += '\n' + textBuffer[endLine].slice(0, endPos);
                return s;
            }
        }
    }
}

function cursorKeyInput(evt, editor, cursor, textBuffer) {
    let c = cursor;
    switch (evt.key) {
        // generally immitates VSCode behavior
        case "ArrowLeft":
            if (evt.metaKey) {
                evt.preventDefault();
                let text = textBuffer[c.line];
                let trimmed = text.trimStart();
                let newPos = text.length - trimmed.length; 
                if (newPos === c.pos) {
                    c.pos = 0;
                } else {
                    c.pos = newPos;
                }
                if (!evt.shiftKey) {
                    c.selectionPos = c.pos;
                }
                c.preferredPos = c.pos;
            } else if (evt.altKey) {
                let result = prevWhitespaceSkip(c.pos, c.line, textBuffer);
                if (result) {
                    c.pos = result[0];
                    c.line = result[1];
                    c.preferredPos = c.pos;
                    if (!evt.shiftKey) {
                        c.selectionPos = c.pos;
                        c.selectionLine = c.line;
                    }
                }
            } else {
                if (c.pos === 0) {
                    if (c.line === 0) {
                        return;
                    }
                    c.line--;
                    c.pos = textBuffer[c.line].length;
                    c.preferredPos = c.pos;
                } else {
                    c.pos--;
                    c.preferredPos = c.pos;
                }
                if (!evt.shiftKey) {
                    c.selectionPos = c.pos;
                    c.selectionLine = c.line;
                }
            }
            updateScrollAfterCursorMove(c.line, editor);
            break;
        case "ArrowRight":
            if (evt.metaKey) {
                evt.preventDefault();
                c.pos = textBuffer[c.line].length;
                c.preferredPos = c.pos;
                if (!evt.shiftKey) {
                    c.selectionPos = c.pos;
                }
            } else if (evt.altKey) {
                let result = nextWhitespaceSkip(c.pos, c.line, textBuffer);
                if (result) {
                    c.pos = result[0];
                    c.line = result[1];
                    c.preferredPos = c.pos;
                    if (!evt.shiftKey) {
                        c.selectionPos = c.pos;
                        c.selectionLine = c.line;
                    }
                }
            } else {
                if (c.pos === textBuffer[c.line].length) {
                    if (c.line === textBuffer.length - 1) {
                        return;
                    }
                    c.line++;
                    c.pos = 0;
                    c.preferredPos = 0;
                } else {
                    c.pos++;
                    c.preferredPos = c.pos;
                }
                if (!evt.shiftKey) {
                    c.selectionPos = c.pos;
                    c.selectionLine = c.line;
                }
            }
            updateScrollAfterCursorMove(c.line, editor);
            break;
        case "ArrowUp":
            if (c.line > 0) {
                c.line--;
                let newLineLength = textBuffer[c.line].length;
                if (c.preferredPos <= newLineLength) {
                    c.pos = c.preferredPos;
                } else {
                    c.pos = newLineLength;
                }
                if (!evt.shiftKey) {
                    c.selectionPos = c.pos;
                    c.selectionLine = c.line;
                }
            }
            updateScrollAfterCursorMove(c.line, editor);
            break;
        case "ArrowDown":
            if (c.line < textBuffer.length - 1) {
                c.line++;
                let newLineLength = textBuffer[c.line].length;
                if (c.preferredPos <= newLineLength) {
                    c.pos = c.preferredPos;
                } else {
                    c.pos = newLineLength;
                }
                if (!evt.shiftKey) {
                    c.selectionPos = c.pos;
                    c.selectionLine = c.line;
                }
            }
            updateScrollAfterCursorMove(c.line, editor);
            break;
        case "Backspace":
            if (deleteSelection(c, textBuffer)) {
                updateMaxScroll(editor);
            } else if (c.pos > 0 ) {
                c.pos--;
                c.preferredPos = c.pos;
                let line = textBuffer[c.line];
                textBuffer[c.line] = line.slice(0, c.pos) + line.slice(c.pos + 1);
                c.selectionPos = c.pos;
            } else if (c.line > 0) {
                var prevLineIdx = c.line - 1;
                var prevLine = textBuffer[prevLineIdx];
                textBuffer[prevLineIdx] = prevLine + textBuffer[c.line];
                textBuffer.splice(c.line, 1);
                c.pos = prevLine.length;
                c.line = prevLineIdx;
                c.selectionPos = c.pos;
                c.selectionLine = c.line;
                updateMaxScroll(editor);
            }
            updateScrollAfterCursorMove(c.line, editor);
            break;
        case "Delete":
            if (deleteSelection(c, textBuffer)) {
                updateMaxScroll(editor);
            } else if (c.pos < textBuffer[c.line].length) {
                let line = textBuffer[c.line];
                textBuffer[c.line] = line.slice(0, c.pos) + line.slice(c.pos + 1);
                c.preferredPos = c.pos;
                c.selectionPos = c.pos;
                c.selectionLine = c.line;
            } else if (c.line < textBuffer.length - 1) {
                textBuffer[c.line] = textBuffer[c.line] + textBuffer[c.line + 1];
                textBuffer.splice(c.line + 1, 1);
                c.selectionPos = c.pos;
                c.selectionLine = c.line;
                updateMaxScroll(editor);
            }
            updateScrollAfterCursorMove(c.line, editor);
            break;
        case "Enter":
            deleteSelection(c, textBuffer);
            let line = textBuffer[c.line];
            textBuffer[c.line] = line.slice(0, c.pos);
            c.line++;
            textBuffer.splice(c.line, 0, line.slice(c.pos));
            c.pos = 0;
            c.preferredPos = c.pos;
            c.selectionPos = c.pos;
            c.selectionLine = c.line;
            updateMaxScroll(editor);
            updateScrollAfterCursorMove(c.line, editor);
            break;
        case "Tab":
            evt.preventDefault();
            deleteSelection(c, textBuffer);
            let numSpacesForTab = editor.dimensions.numSpacesForTab;
            let numSpaces = numSpacesForTab - (c.pos % numSpacesForTab);
            let insert;
            switch (numSpaces) {
                case 1:
                    insert = ' ';
                    break;
                case 2:
                    insert = '  ';
                    break;
                case 3:
                    insert = '   ';
                    break;
                case 4:
                    insert = '    ';
                    break;
            }
            insertText(insert, textBuffer, c, editor);
            break;
        default:
            break;
    }

    // returns new [pos, line], or null if cursor already at start of text
    function prevWhitespaceSkip(pos, line, textBuffer) {
        let text = textBuffer[line];
        if (pos === 0) {
            if (line === 0) {
                return null;
            }
            line--;
            text = textBuffer[line];
            pos = text.length;
            if (pos === 0) {
                return [pos, line];
            }
        }
        let preceding = text.slice(0, pos);
        let trimmed = preceding.trimEnd();
        let firstSpace = trimmed.lastIndexOf(' ');
        if (firstSpace === -1) {
            return [0, line];
        } else {
            return [firstSpace + 1,  line];
        }
    }

    // return [newPos, newLine], or null if cursor already at end
    function nextWhitespaceSkip(pos, line, textBuffer) {
        let text = textBuffer[line];
        if (pos === text.length) {
            if (line === textBuffer.length - 1) {
                return null;
            }
            line++;
            pos = 0;
            text = textBuffer[line];
            if (pos === text.length) {
                return [pos, line];
            }
        }
        let remaining = text.slice(pos);
        let trimmed = remaining.trimStart();
        let firstSpace = trimmed.indexOf(' ');
        if (firstSpace === -1) {
            return [text.length, line];
        } else {
            return [pos + (remaining.length - trimmed.length) + firstSpace,  line];
        }
    }
}

export function mouseInput(evt, isClick, editor) {
    let c = editor.cursor;
    let d = editor.dimensions;
    let tb = editor.textBuffer;

    // todo: using clientX/Y for now; should get relative from canvas (to allow a canvas editor that isn't full page)
    const textCursorOffsetY = 7;   // want text cursor to select as if from center of the cursor, not top (there's no way to get the cursor's actual height, so we guess its half height)
    
    let pos = Math.round((evt.clientX - d.lineOffsetX) / d.characterWidth);
    let line = Math.floor((evt.clientY - d.firstLineOffsetY + textCursorOffsetY + editor.scroll) / d.lineHeight);
    if (pos < 0) {
        pos = 0;
    }
    if (line < 0) {
        line = 0;
    }
    if (line > tb.length - 1) {
        line = tb.length - 1;
    }
    if (pos > tb[line].length) {
        pos = tb[line].length;
    }
    c.pos = pos;
    c.preferredPos = pos;
    c.line = line;
    if (isClick) {
        if (!evt.shiftKey) {
            c.selectionPos = c.pos;
            c.selectionLine = c.line;
        }
        editor.mouseDown = true;
    }
    updateScrollAfterCursorMove(c.line, editor);
    showCursor(editor.cursor, editor);
}

export function draw(editor) {
    let e = editor;
    let ctx = editor.ctx;
    let tb = editor.textBuffer;
    let d = editor.dimensions;
    let c = editor.cursor;
    let colors = e.colors;

    // clear screen
    ctx.fillStyle = colors.background;
    ctx.fillRect(0, 0, d.width, d.height);

    let numLines = Math.ceil(d.height / d.lineHeight) + 2;  // plus one extra line for partial in view up top, one extra for bottom
    let firstLine = Math.floor(e.scroll / d.lineHeight) - 1;  // back one so as to partially show top line scrolling into view
    if (firstLine < 0) {
        firstLine = 0;
    }
    let lastLine = firstLine + numLines - 1;
    if (lastLine > (tb.length - 1)) {
        lastLine = tb.length - 1;
    }

    let selectionActive = drawSelection(ctx, tb, e.scroll, c, colors, d, firstLine, lastLine);
    drawText(ctx, e.defaultFont, tb, e.scroll, c.line, colors, d, firstLine, lastLine, !selectionActive);
    if (c.shown && e.hasFocus) {
        drawCursor(ctx, e.colors.cursor, d, e.scroll);
    }

    function drawCursor(ctx, cursorColor, dimensions, scroll) {
        let d = dimensions;
        ctx.fillStyle = cursorColor;
        ctx.fillRect(
            d.lineOffsetX + d.characterWidth * c.pos, 
            d.firstLineOffsetY + d.cursorOffsetY + d.lineHeight * c.line - scroll,
            d.cursorWidth, 
            d.cursorHeight
        );
    }

    function drawText(ctx, font, textBuffer, scroll, cursorLine, colors, 
            dimensions, firstLine, lastLine, drawCursorLineHighlight) {
        const lineHighlightOffsetY = 5;
        let d = dimensions;

        // draw lines
        ctx.fillStyle = colors.defaultText;
        ctx.font = font;
        ctx.textBaseline = 'top';
        ctx.textAlign = 'start';
        let startY = d.firstLineOffsetY - scroll + d.lineHeight * firstLine;
        let y = startY;
        for (let i = firstLine; i <= lastLine; i++) {
            let text = textBuffer[i];
            if (drawCursorLineHighlight && i === cursorLine) {
                ctx.fillStyle = colors.lineHighlight;
                ctx.fillRect(d.lineOffsetX, y - lineHighlightOffsetY, d.width, d.lineHeight);
                ctx.fillStyle = colors.defaultText;
                ctx.font = font;
            }
            ctx.fillText(text, d.lineOffsetX, y);
            y += d.lineHeight;
        }

        // draw line numbers
        ctx.textAlign = 'right';
        ctx.fillStyle = colors.lineNumber;
        y = startY;
        for (let i = firstLine; i <= lastLine; i++) {
            ctx.fillText(i + 1, d.lineOffsetX - d.numbersOffsetX, y);
            y += d.lineHeight;
        }
    }

    function drawSelection(ctx, textBuffer, scroll, cursor, colors, dimensions, firstLine, lastLine) {
        const adjustmentY = -5;
        let d = dimensions;
        let c = cursor;

        ctx.fillStyle = colors.selection;
        if (c.line === c.selectionLine) {
            if (c.pos === c.selectionPos) {
                // draw nothing
                return false;
            } else if (c.pos < c.selectionPos) {
                ctx.fillRect(
                    d.lineOffsetX + d.characterWidth * c.pos, 
                    d.firstLineOffsetY + d.lineHeight * c.line - scroll + adjustmentY,
                    (c.selectionPos - c.pos) * d.characterWidth,
                    d.lineHeight
                );
            } else if (c.pos > c.selectionPos) {
                ctx.fillRect(
                    d.lineOffsetX + d.characterWidth * c.selectionPos, 
                    d.firstLineOffsetY + d.lineHeight * c.line - scroll + adjustmentY,
                    (c.pos - c.selectionPos) * d.characterWidth,
                    d.lineHeight
                );
            }
        } else if (c.line < c.selectionLine) {
            drawDownwardsSelection(ctx, textBuffer, d,
                c.pos, c.line, c.selectionPos, c.selectionLine,
                firstLine, lastLine, adjustmentY
            );
        } else if (c.line > c.selectionLine) {
            drawDownwardsSelection(ctx, textBuffer, d,
                c.selectionPos, c.selectionLine, c.pos, c.line,
                firstLine, lastLine, adjustmentY
            );
        }
        return true;

        function drawDownwardsSelection(ctx, textBuffer, dimensions, 
                topPos, topLine, bottomPos, bottomLine,
                firstLine, lastLine, adjustmentY) {
            let c = cursor;
            let d = dimensions;

            // draw top selection line
            ctx.fillRect(
                d.lineOffsetX + d.characterWidth * topPos,
                d.firstLineOffsetY + d.lineHeight * topLine - scroll + adjustmentY,
                (textBuffer[topLine].length - topPos + 1) * d.characterWidth,
                d.lineHeight
            );
            // draw middle selection lines
            let start = topLine + 1;
            let end = bottomLine;
            if (start <= lastLine && end >= firstLine) {
                if (start < firstLine) {
                    start = firstLine;
                }
                if (end > lastLine) {
                    end = lastLine + 1;
                }
                let y = d.firstLineOffsetY + d.lineHeight * start - scroll + adjustmentY;
                for (let i = start; i < end; i++) {
                    ctx.fillRect(
                        d.lineOffsetX,
                        y,
                        (textBuffer[i].length + 1) * d.characterWidth,
                        d.lineHeight
                    );  
                    y += d.lineHeight;
                }
            }
            // draw bottom selection line
            ctx.fillRect(
                d.lineOffsetX,
                d.firstLineOffsetY + d.lineHeight * bottomLine - scroll + adjustmentY,
                bottomPos * d.characterWidth,
                d.lineHeight
            );
        }
    }
}
