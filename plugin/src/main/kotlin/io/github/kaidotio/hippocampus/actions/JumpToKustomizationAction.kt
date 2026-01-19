package io.github.kaidotio.hippocampus.actions

import com.intellij.openapi.actionSystem.AnAction
import com.intellij.openapi.actionSystem.AnActionEvent
import com.intellij.openapi.actionSystem.CommonDataKeys
import com.intellij.openapi.fileEditor.FileEditorManager
import com.intellij.openapi.ui.Messages
import com.intellij.psi.search.FilenameIndex
import com.intellij.psi.search.GlobalSearchScope
import org.yaml.snakeyaml.Yaml

class JumpToKustomizationAction : AnAction() {
    override fun actionPerformed(event: AnActionEvent) {
        val project = event.project ?: return
        val file = event.getData(CommonDataKeys.VIRTUAL_FILE) ?: return

        if (!isKubernetesManifest(file)) {
            Messages.showInfoMessage(project, "Not a Kubernetes manifest", "Error")
            return
        }

        val kustomizationFiles = listOf("kustomization.yaml", "kustomization.yml")
            .flatMap { FilenameIndex.getVirtualFilesByName(it, GlobalSearchScope.projectScope(project)) }
            .filter { kustomizationFile ->
                isManifestReferencedInKustomization(file, kustomizationFile)
            }
            .sortedBy { it.path }

        when (kustomizationFiles.size) {
            0 -> Messages.showInfoMessage(project, "No kustomization.yaml found", "Not Found")
            1 -> FileEditorManager.getInstance(project).openFile(kustomizationFiles.first(), true)
            else -> {
                val projectBasePath = project.basePath ?: ""
                val options = kustomizationFiles.map { kustomizationFile ->
                    val relativePath = if (kustomizationFile.path.startsWith(projectBasePath)) {
                        kustomizationFile.path.removePrefix("$projectBasePath/")
                    } else {
                        kustomizationFile.path
                    }
                    relativePath
                }.toTypedArray()

                val selected = Messages.showDialog(
                    project,
                    "Select kustomization file:",
                    "Multiple Files Found",
                    options,
                    0,
                    Messages.getQuestionIcon()
                )
                if (selected >= 0) {
                    FileEditorManager.getInstance(project).openFile(kustomizationFiles[selected], true)
                }
            }
        }
    }

    override fun update(event: AnActionEvent) {
        val file = event.getData(CommonDataKeys.VIRTUAL_FILE)
        event.presentation.isEnabledAndVisible = file?.extension in listOf("yaml", "yml")
    }

    private fun isKubernetesManifest(file: com.intellij.openapi.vfs.VirtualFile): Boolean {
        return try {
            val data = Yaml().load<Any>(String(file.contentsToByteArray()))
            when (data) {
                is Map<*, *> -> data.containsKey("apiVersion") && data.containsKey("kind")
                is List<*> -> data.any { it is Map<*, *> && it.containsKey("apiVersion") && it.containsKey("kind") }
                else -> false
            }
        } catch (e: Exception) {
            false
        }
    }

    private fun isManifestReferencedInKustomization(
        manifestFile: com.intellij.openapi.vfs.VirtualFile,
        kustomizationFile: com.intellij.openapi.vfs.VirtualFile
    ): Boolean {
        return try {
            val kustomization = Yaml().load<Map<String, Any>>(String(kustomizationFile.contentsToByteArray()))
            val kustomizationDir = kustomizationFile.parent ?: return false

            val resources = (kustomization["resources"] as? List<*>)?.map { it.toString() } ?: emptyList()
            val patches = (kustomization["patches"] as? List<*>)?.flatMap { patch ->
                when (patch) {
                    is String -> listOf(patch)
                    is Map<*, *> -> listOfNotNull(patch["path"]?.toString())
                    else -> emptyList()
                }
            } ?: emptyList()

            val allPaths = resources + patches

            allPaths.any { path ->
                val resolvedFile = kustomizationDir.findFileByRelativePath(path)
                resolvedFile?.path == manifestFile.path
            }
        } catch (e: Exception) {
            false
        }
    }
}
