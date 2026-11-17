package io.github.kaidotio.hippocampus.highlighter

import com.intellij.lexer.Lexer
import com.intellij.openapi.editor.DefaultLanguageHighlighterColors
import com.intellij.openapi.editor.colors.TextAttributesKey
import com.intellij.openapi.fileTypes.SyntaxHighlighterBase
import com.intellij.psi.tree.IElementType
import io.github.kaidotio.hippocampus.lexer.FilterLexer
import io.github.kaidotio.hippocampus.lexer.FilterTokenTypes

class FilterSyntaxHighlighter : SyntaxHighlighterBase() {
    companion object {
        private val COMMENT = TextAttributesKey.createTextAttributesKey(
            "FILTER_COMMENT",
            DefaultLanguageHighlighterColors.LINE_COMMENT
        )

        private val BLOCK_KEYWORD = TextAttributesKey.createTextAttributesKey(
            "FILTER_BLOCK_KEYWORD",
            DefaultLanguageHighlighterColors.KEYWORD
        )

        private val CONDITION_KEYWORD = TextAttributesKey.createTextAttributesKey(
            "FILTER_CONDITION_KEYWORD",
            DefaultLanguageHighlighterColors.KEYWORD
        )

        private val ACTION_KEYWORD = TextAttributesKey.createTextAttributesKey(
            "FILTER_ACTION_KEYWORD",
            DefaultLanguageHighlighterColors.FUNCTION_DECLARATION
        )

        private val NUMBER = TextAttributesKey.createTextAttributesKey(
            "FILTER_NUMBER",
            DefaultLanguageHighlighterColors.NUMBER
        )

        private val STRING = TextAttributesKey.createTextAttributesKey(
            "FILTER_STRING",
            DefaultLanguageHighlighterColors.STRING
        )

        private val OPERATOR = TextAttributesKey.createTextAttributesKey(
            "FILTER_OPERATOR",
            DefaultLanguageHighlighterColors.OPERATION_SIGN
        )

        private val RARITY_VALUE = TextAttributesKey.createTextAttributesKey(
            "FILTER_RARITY_VALUE",
            DefaultLanguageHighlighterColors.CONSTANT
        )

        private val BOOLEAN_VALUE = TextAttributesKey.createTextAttributesKey(
            "FILTER_BOOLEAN_VALUE",
            DefaultLanguageHighlighterColors.CONSTANT
        )

        private val IDENTIFIER = TextAttributesKey.createTextAttributesKey(
            "FILTER_IDENTIFIER",
            DefaultLanguageHighlighterColors.IDENTIFIER
        )

        private val COMMENT_KEYS = arrayOf(COMMENT)
        private val BLOCK_KEYWORD_KEYS = arrayOf(BLOCK_KEYWORD)
        private val CONDITION_KEYWORD_KEYS = arrayOf(CONDITION_KEYWORD)
        private val ACTION_KEYWORD_KEYS = arrayOf(ACTION_KEYWORD)
        private val NUMBER_KEYS = arrayOf(NUMBER)
        private val STRING_KEYS = arrayOf(STRING)
        private val OPERATOR_KEYS = arrayOf(OPERATOR)
        private val RARITY_VALUE_KEYS = arrayOf(RARITY_VALUE)
        private val BOOLEAN_VALUE_KEYS = arrayOf(BOOLEAN_VALUE)
        private val IDENTIFIER_KEYS = arrayOf(IDENTIFIER)
        private val EMPTY_KEYS = arrayOf<TextAttributesKey>()
    }

    override fun getHighlightingLexer(): Lexer = FilterLexer()

    override fun getTokenHighlights(tokenType: IElementType?): Array<TextAttributesKey> {
        return when (tokenType) {
            FilterTokenTypes.COMMENT -> COMMENT_KEYS
            FilterTokenTypes.BLOCK_KEYWORD -> BLOCK_KEYWORD_KEYS
            FilterTokenTypes.CONDITION_KEYWORD -> CONDITION_KEYWORD_KEYS
            FilterTokenTypes.ACTION_KEYWORD -> ACTION_KEYWORD_KEYS
            FilterTokenTypes.NUMBER -> NUMBER_KEYS
            FilterTokenTypes.STRING -> STRING_KEYS
            FilterTokenTypes.OPERATOR -> OPERATOR_KEYS
            FilterTokenTypes.RARITY_VALUE -> RARITY_VALUE_KEYS
            FilterTokenTypes.BOOLEAN_VALUE -> BOOLEAN_VALUE_KEYS
            FilterTokenTypes.IDENTIFIER -> IDENTIFIER_KEYS
            FilterTokenTypes.EOL -> EMPTY_KEYS
            else -> EMPTY_KEYS
        }
    }
}
