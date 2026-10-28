package io.github.kaidotio.hippocampus.filetype

import com.intellij.openapi.fileTypes.FileType
import com.intellij.openapi.fileTypes.FileTypeRegistry
import com.intellij.openapi.util.io.ByteSequence
import com.intellij.openapi.vfs.VirtualFile

class FilterFileTypeDetector : FileTypeRegistry.FileTypeDetector {
    override fun detect(file: VirtualFile, firstBytes: ByteSequence, firstCharsIfText: CharSequence?): FileType? {
        if (file.extension == "filter") {
            return FilterFileType.INSTANCE
        }
        return null
    }

    override fun getDesiredContentPrefixLength(): Int = 0
}
