package io.github.kaidotio.hippocampus.listeners

import io.github.kaidotio.hippocampus.crypto.RailsMessageEncryptor
import io.github.kaidotio.hippocampus.editor.EncryptedFileEditorProvider
import io.github.kaidotio.hippocampus.filetype.EncryptedFileType
import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.diagnostic.thisLogger
import com.intellij.openapi.editor.Document
import com.intellij.openapi.fileEditor.FileDocumentManager
import com.intellij.openapi.fileEditor.FileDocumentSynchronizationVetoer
import com.intellij.openapi.ui.Messages
import com.intellij.openapi.util.Key

class EncryptedFileSaveListener : FileDocumentSynchronizationVetoer() {

    companion object {
        private val logger = thisLogger()
        private val IS_SAVING_KEY = Key<Boolean>("IS_SAVING_ENCRYPTED_FILE")
    }

    override fun maySaveDocument(document: Document, isSaveExplicit: Boolean): Boolean {
        val file = FileDocumentManager.getInstance().getFile(document) ?: return true

        val originalFile = if (file.getUserData(EncryptedFileEditorProvider.IS_TEMP_FILE_KEY) == true) {
            EncryptedFileEditorProvider.getOriginalFileFor(file)
        } else if (file.fileType == EncryptedFileType.INSTANCE) {
            file
        } else {
            return true
        }

        if (originalFile == null) {
            return true
        }

        if (document.getUserData(IS_SAVING_KEY) == true) {
            return false
        }

        try {
            document.putUserData(IS_SAVING_KEY, true)

            val encryptor = RailsMessageEncryptor()
            val currentContent = document.text
            val originalDecrypted = encryptor.decrypt(String(originalFile.contentsToByteArray(), Charsets.UTF_8))

            if (currentContent == originalDecrypted) {
                if (file.getUserData(EncryptedFileEditorProvider.IS_TEMP_FILE_KEY) == true) {
                    ApplicationManager.getApplication().invokeLater {
                        ApplicationManager.getApplication().runWriteAction {
                            val doc = FileDocumentManager.getInstance().getDocument(file)
                            if (doc != null && FileDocumentManager.getInstance().isDocumentUnsaved(doc)) {
                                doc.setReadOnly(true)
                                doc.setReadOnly(false)
                            }
                        }
                    }
                }
                return false
            }

            ApplicationManager.getApplication().runWriteAction {
                originalFile.setBinaryContent(encryptor.encrypt(currentContent).toByteArray(Charsets.UTF_8))
            }

            if (file.getUserData(EncryptedFileEditorProvider.IS_TEMP_FILE_KEY) == true) {
                ApplicationManager.getApplication().invokeLater {
                    ApplicationManager.getApplication().runWriteAction {
                        val doc = FileDocumentManager.getInstance().getDocument(file)
                        if (doc != null && FileDocumentManager.getInstance().isDocumentUnsaved(doc)) {
                            doc.setReadOnly(true)
                            doc.setReadOnly(false)
                        }
                    }
                }
                return false
            }

            return false
        } catch (e: IllegalStateException) {
            logger.error("Environment variable RAILS_MASTER_KEY is not set", e)
            showError("Environment variable RAILS_MASTER_KEY is not set.")
            return false
        } catch (e: Exception) {
            logger.error("Failed to encrypt file: ${originalFile.name}", e)
            showError("Failed to encrypt file: ${e.message}")
            return false
        } finally {
            document.putUserData(IS_SAVING_KEY, null)
        }
    }

    private fun showError(message: String) {
        ApplicationManager.getApplication().invokeLater {
            Messages.showErrorDialog(message, "EncryptedFileSaveHandler Error")
        }
    }
}
