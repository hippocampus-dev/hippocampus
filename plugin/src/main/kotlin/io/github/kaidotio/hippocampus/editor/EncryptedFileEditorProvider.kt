package io.github.kaidotio.hippocampus.editor

import io.github.kaidotio.hippocampus.crypto.RailsMessageEncryptor
import io.github.kaidotio.hippocampus.filetype.EncryptedFileType
import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.diagnostic.thisLogger
import com.intellij.openapi.fileEditor.FileEditor
import com.intellij.openapi.fileEditor.FileEditorPolicy
import com.intellij.openapi.fileEditor.FileEditorProvider
import com.intellij.openapi.fileEditor.TextEditor
import com.intellij.openapi.fileEditor.impl.text.TextEditorProvider
import com.intellij.openapi.project.DumbAware
import com.intellij.openapi.project.Project
import com.intellij.openapi.ui.Messages
import com.intellij.openapi.util.Disposer
import com.intellij.openapi.util.Key
import com.intellij.openapi.vfs.VirtualFile
import com.intellij.openapi.vfs.VirtualFileManager
import com.intellij.openapi.fileTypes.FileTypeManager
import java.util.concurrent.ConcurrentHashMap

class EncryptedFileEditorProvider : FileEditorProvider, DumbAware {

    companion object {
        private val logger = thisLogger()
        private val tempFileMap = ConcurrentHashMap<VirtualFile, VirtualFile>()
        val IS_TEMP_FILE_KEY = Key<Boolean>("IS_ENCRYPTED_FILE_TEMP")

        fun getOriginalFileFor(tempFile: VirtualFile): VirtualFile? {
            return tempFileMap[tempFile]
        }

        private fun registerTempFile(tempFile: VirtualFile, originalFile: VirtualFile) {
            tempFileMap[tempFile] = originalFile
        }

        private fun unregisterTempFile(tempFile: VirtualFile) {
            tempFileMap.remove(tempFile)
        }
    }

    override fun accept(project: Project, file: VirtualFile): Boolean {
        return file.fileType == EncryptedFileType.INSTANCE ||
               file.getUserData(IS_TEMP_FILE_KEY) == true
    }

    override fun createEditor(project: Project, file: VirtualFile): FileEditor {
        if (file.getUserData(IS_TEMP_FILE_KEY) == true) {
            return TextEditorProvider.getInstance().createEditor(project, file) as TextEditor
        }

        return try {
            val decryptedContent = RailsMessageEncryptor().decrypt(String(file.contentsToByteArray(), Charsets.UTF_8))

            val tempFileSystem = VirtualFileManager.getInstance().getFileSystem("temp")
            val tempRoot = tempFileSystem?.findFileByPath("/")

            if (tempRoot == null) {
                logger.error("Could not access temp file system")
                throw IllegalStateException("Temp file system not available")
            }

            val nameWithoutEnc = file.nameWithoutExtension

            val fileTypeManager = FileTypeManager.getInstance()
            val (baseName, originalExtension) = run {
                val allExtensions = mutableSetOf<String>()

                for (fileType in fileTypeManager.registeredFileTypes) {
                    val defaultExtension = fileType.defaultExtension
                    if (defaultExtension.isNotEmpty()) {
                        allExtensions.add(".$defaultExtension")
                    }

                    for (matcher in fileTypeManager.getAssociations(fileType)) {
                        if (matcher is com.intellij.openapi.fileTypes.ExtensionFileNameMatcher) {
                            allExtensions.add(".${matcher.extension}")
                        }
                    }
                }

                val sortedExtensions = allExtensions.sortedByDescending { it.length }
                for (ext in sortedExtensions) {
                    if (nameWithoutEnc.endsWith(ext)) {
                        return@run nameWithoutEnc.removeSuffix(ext) to ext
                    }
                }

                nameWithoutEnc to ""
            }

            val tempFileName = "decrypted_${baseName}_${System.currentTimeMillis()}${originalExtension}"

            val tempFile = ApplicationManager.getApplication().runWriteAction<VirtualFile> {
                val newFile = tempRoot.createChildData(this, tempFileName)
                newFile.setBinaryContent(decryptedContent.toByteArray(Charsets.UTF_8))
                newFile.putUserData(IS_TEMP_FILE_KEY, true)
                newFile
            }

            registerTempFile(tempFile, file)

            val textEditor = TextEditorProvider.getInstance().createEditor(project, tempFile) as TextEditor

            Disposer.register(textEditor) {
                ApplicationManager.getApplication().runWriteAction {
                    try {
                        tempFile.delete(this)
                    } catch (e: Exception) {
                        logger.warn("Failed to delete temp file: ${tempFile.name}", e)
                    }
                }
                unregisterTempFile(tempFile)
            }

            textEditor
        } catch (e: IllegalStateException) {
            logger.error("Environment variable RAILS_MASTER_KEY is not set", e)
            showError("Environment variable RAILS_MASTER_KEY is not set.")
            TextEditorProvider.getInstance().createEditor(project, file) as TextEditor
        } catch (e: Exception) {
            logger.error("Failed to decrypt file: ${file.name}", e)
            showError("Failed to decrypt file: ${e.message}")
            TextEditorProvider.getInstance().createEditor(project, file) as TextEditor
        }
    }

    private fun showError(message: String) {
        ApplicationManager.getApplication().invokeLater {
            Messages.showErrorDialog(message, "EncryptedFileEditorProvider Error")
        }
    }

    override fun getEditorTypeId(): String = "encrypted-file-text-editor"

    override fun getPolicy(): FileEditorPolicy = FileEditorPolicy.HIDE_DEFAULT_EDITOR
}
