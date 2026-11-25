import { invoke } from "@tauri-apps/api/core";

export async function waitForUpdate(): Promise<void> {
  try {
    await invoke("plugin:auto-update|wait_for_update");
  } catch (error) {
    console.error("Failed to wait for update:", error);
    throw error;
  }
}

export async function installUpdate(): Promise<void> {
  try {
    await invoke("plugin:auto-update|install_update");
  } catch (error) {
    console.error("Failed to install update:", error);
    throw error;
  }
}
