package io.github.kaidotio.hippocampus.psi

import com.intellij.extapi.psi.PsiFileBase
import com.intellij.openapi.fileTypes.FileType
import com.intellij.psi.FileViewProvider
import io.github.kaidotio.hippocampus.filetype.FilterFileType
import io.github.kaidotio.hippocampus.language.FilterLanguage

class FilterFile(viewProvider: FileViewProvider) : PsiFileBase(viewProvider, FilterLanguage.INSTANCE) {
    override fun getFileType(): FileType = FilterFileType.INSTANCE

    override fun toString(): String = "Filter File"
}
