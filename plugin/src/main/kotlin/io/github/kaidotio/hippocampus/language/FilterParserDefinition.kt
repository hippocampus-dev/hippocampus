package io.github.kaidotio.hippocampus.language

import com.intellij.lang.ASTNode
import com.intellij.lang.ParserDefinition
import com.intellij.lang.PsiParser
import com.intellij.lexer.Lexer
import com.intellij.openapi.project.Project
import com.intellij.psi.FileViewProvider
import com.intellij.psi.PsiElement
import com.intellij.psi.PsiFile
import com.intellij.psi.tree.IFileElementType
import com.intellij.psi.tree.TokenSet
import io.github.kaidotio.hippocampus.lexer.FilterLexer
import io.github.kaidotio.hippocampus.lexer.FilterTokenTypes
import io.github.kaidotio.hippocampus.psi.FilterFile

class FilterParserDefinition : ParserDefinition {
    companion object {
        val FILE = IFileElementType(FilterLanguage.INSTANCE)
        val COMMENTS = TokenSet.create(FilterTokenTypes.COMMENT)
    }

    override fun createLexer(project: Project?): Lexer = FilterLexer()

    override fun createParser(project: Project?): PsiParser = FilterParser()

    override fun getFileNodeType(): IFileElementType = FILE

    override fun getCommentTokens(): TokenSet = COMMENTS

    override fun getStringLiteralElements(): TokenSet = TokenSet.create(FilterTokenTypes.STRING)

    override fun createElement(node: ASTNode?): PsiElement = FilterPsiElement(node!!)

    override fun createFile(viewProvider: FileViewProvider): PsiFile = FilterFile(viewProvider)
}
