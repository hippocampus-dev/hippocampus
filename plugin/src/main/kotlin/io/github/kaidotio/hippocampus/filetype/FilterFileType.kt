package io.github.kaidotio.hippocampus.filetype

import com.intellij.openapi.fileTypes.LanguageFileType
import io.github.kaidotio.hippocampus.language.FilterLanguage
import javax.swing.Icon

class FilterFileType : LanguageFileType(FilterLanguage.INSTANCE) {
    companion object {
        @JvmField
        val INSTANCE = FilterFileType()
    }

    override fun getName(): String = "Filter"

    override fun getDescription(): String = "Filter file"

    override fun getDefaultExtension(): String = "filter"

    override fun getIcon(): Icon? = null
}
