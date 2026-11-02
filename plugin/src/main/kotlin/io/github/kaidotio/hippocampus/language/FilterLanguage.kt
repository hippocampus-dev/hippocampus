package io.github.kaidotio.hippocampus.language

import com.intellij.lang.Language

class FilterLanguage : Language("Filter") {
    companion object {
        @JvmField
        val INSTANCE = FilterLanguage()
    }
}
