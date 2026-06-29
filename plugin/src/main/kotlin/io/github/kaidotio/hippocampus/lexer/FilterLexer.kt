package io.github.kaidotio.hippocampus.lexer

import com.intellij.lexer.LexerBase
import com.intellij.psi.tree.IElementType

class FilterLexer : LexerBase() {
    private var buffer: CharSequence? = null
    private var startOffset = 0
    private var endOffset = 0
    private var currentPosition = 0
    private var tokenStart = 0
    private var tokenEnd = 0
    private var currentTokenType: IElementType? = null

    override fun start(buffer: CharSequence, startOffset: Int, endOffset: Int, initialState: Int) {
        this.buffer = buffer
        this.startOffset = startOffset
        this.endOffset = endOffset
        this.currentPosition = startOffset
        this.tokenStart = startOffset
        this.tokenEnd = startOffset
        this.currentTokenType = null
        advance()
    }

    override fun getState(): Int = 0

    override fun getTokenType(): IElementType? = currentTokenType

    override fun getTokenStart(): Int = tokenStart

    override fun getTokenEnd(): Int = tokenEnd

    override fun advance() {
        if (currentPosition >= endOffset) {
            currentTokenType = null
            return
        }

        tokenStart = currentPosition
        skipWhitespaceExceptNewline()

        if (currentPosition >= endOffset) {
            currentTokenType = null
            return
        }

        val char = buffer!![currentPosition]
        when {
            char == '#' -> {
                while (currentPosition < endOffset && buffer!![currentPosition] != '\n') {
                    currentPosition++
                }
                tokenEnd = currentPosition
                currentTokenType = FilterTokenTypes.COMMENT
            }
            char == '"' -> {
                currentPosition++
                while (currentPosition < endOffset && buffer!![currentPosition] != '"') {
                    if (buffer!![currentPosition] == '\\' && currentPosition + 1 < endOffset) {
                        currentPosition += 2
                    } else {
                        currentPosition++
                    }
                }
                if (currentPosition < endOffset) {
                    currentPosition++
                }
                tokenEnd = currentPosition
                currentTokenType = FilterTokenTypes.STRING
            }
            char == '\n' -> {
                currentPosition++
                tokenEnd = currentPosition
                currentTokenType = FilterTokenTypes.EOL
            }
            char.isDigit() -> {
                while (currentPosition < endOffset &&
                       (buffer!![currentPosition].isDigit() || buffer!![currentPosition] == '.')) {
                    currentPosition++
                }
                tokenEnd = currentPosition
                currentTokenType = FilterTokenTypes.NUMBER
            }
            char in "()<>=!" -> {
                currentPosition++
                if (currentPosition < endOffset) {
                    val nextChar = buffer!![currentPosition]
                    if ((char == '<' || char == '>' || char == '=' || char == '!') && nextChar == '=') {
                        currentPosition++
                    }
                }
                tokenEnd = currentPosition
                currentTokenType = FilterTokenTypes.OPERATOR
            }
            else -> {
                val wordStart = currentPosition
                while (currentPosition < endOffset &&
                       !buffer!![currentPosition].isWhitespace() &&
                       buffer!![currentPosition] !in "()<>=!\"") {
                    currentPosition++
                }
                tokenEnd = currentPosition

                val word = buffer!!.subSequence(wordStart, currentPosition).toString()
                currentTokenType = when (word) {
                    "Show", "Hide" -> FilterTokenTypes.BLOCK_KEYWORD
                    "BaseType", "Class", "Rarity", "ItemLevel", "AreaLevel",
                    "DropLevel", "Quality", "Sockets", "LinkedSockets",
                    "SocketGroup", "Corrupted", "Identified", "Elder",
                    "Shaper", "Influenced", "SynthesisedItem", "FracturedItem",
                    "HasExplicitMod", "HasInfluence", "AnyEnchantment",
                    "UnidentifiedItemTier", "WaystoneTier" -> FilterTokenTypes.CONDITION_KEYWORD
                    "SetBackgroundColor", "SetBorderColor", "SetTextColor",
                    "SetFontSize", "PlayAlertSound", "PlayAlertSoundPositional",
                    "DisableDropSound", "CustomAlertSound", "MinimapIcon",
                    "PlayEffect" -> FilterTokenTypes.ACTION_KEYWORD
                    "Normal", "Magic", "Rare", "Unique" -> FilterTokenTypes.RARITY_VALUE
                    "True", "False" -> FilterTokenTypes.BOOLEAN_VALUE
                    else -> FilterTokenTypes.IDENTIFIER
                }
            }
        }
    }

    override fun getBufferSequence(): CharSequence = buffer ?: ""

    override fun getBufferEnd(): Int = endOffset

    private fun skipWhitespaceExceptNewline() {
        while (currentPosition < endOffset) {
            val char = buffer!![currentPosition]
            if (char == ' ' || char == '\t' || char == '\r') {
                currentPosition++
            } else {
                break
            }
        }
    }
}
