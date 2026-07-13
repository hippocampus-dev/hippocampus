package io.github.kaidotio.hippocampus.filetype

import com.intellij.openapi.fileTypes.FileType
import com.intellij.openapi.vfs.VirtualFile
import javax.swing.Icon

class EncryptedFileType : FileType {
    companion object {
        @JvmField
        val INSTANCE = EncryptedFileType()
    }

    override fun getName(): String = "Encrypted"

    override fun getDescription(): String = "Encrypted file"

    override fun getDefaultExtension(): String = "enc"

    override fun getIcon(): Icon? = null

    override fun isBinary(): Boolean = false

    override fun isReadOnly(): Boolean = false

    override fun getCharset(file: VirtualFile, content: ByteArray): String? = "UTF-8"
}
