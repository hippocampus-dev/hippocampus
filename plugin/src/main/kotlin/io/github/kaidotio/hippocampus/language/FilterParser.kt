package io.github.kaidotio.hippocampus.language

import com.intellij.lang.ASTNode
import com.intellij.lang.PsiBuilder
import com.intellij.lang.PsiParser
import com.intellij.psi.tree.IElementType

class FilterParser : PsiParser {
    override fun parse(root: IElementType, builder: PsiBuilder): ASTNode {
        val rootMarker = builder.mark()

        while (!builder.eof()) {
            val tokenType = builder.tokenType

            if (tokenType == null) {
                builder.advanceLexer()
            } else {
                val tokenMarker = builder.mark()
                builder.advanceLexer()
                tokenMarker.done(tokenType)
            }
        }

        rootMarker.done(root)
        return builder.treeBuilt
    }
}
