package io.github.kaidotio.hippocampus.lexer

import com.intellij.psi.tree.IElementType
import io.github.kaidotio.hippocampus.language.FilterLanguage

object FilterTokenTypes {
    @JvmField
    val COMMENT = IElementType("FILTER_COMMENT", FilterLanguage.INSTANCE)

    @JvmField
    val BLOCK_KEYWORD = IElementType("FILTER_BLOCK_KEYWORD", FilterLanguage.INSTANCE)

    @JvmField
    val CONDITION_KEYWORD = IElementType("FILTER_CONDITION_KEYWORD", FilterLanguage.INSTANCE)

    @JvmField
    val ACTION_KEYWORD = IElementType("FILTER_ACTION_KEYWORD", FilterLanguage.INSTANCE)

    @JvmField
    val OPERATOR = IElementType("FILTER_OPERATOR", FilterLanguage.INSTANCE)

    @JvmField
    val NUMBER = IElementType("FILTER_NUMBER", FilterLanguage.INSTANCE)

    @JvmField
    val STRING = IElementType("FILTER_STRING", FilterLanguage.INSTANCE)

    @JvmField
    val IDENTIFIER = IElementType("FILTER_IDENTIFIER", FilterLanguage.INSTANCE)

    @JvmField
    val RARITY_VALUE = IElementType("FILTER_RARITY_VALUE", FilterLanguage.INSTANCE)

    @JvmField
    val BOOLEAN_VALUE = IElementType("FILTER_BOOLEAN_VALUE", FilterLanguage.INSTANCE)

    @JvmField
    val EOL = IElementType("FILTER_EOL", FilterLanguage.INSTANCE)
}
