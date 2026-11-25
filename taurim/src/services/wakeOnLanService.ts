import { invoke } from "@tauri-apps/api/core";

export interface WakeOnLanSchedule {
  mac_address: string;
  hour: number;
  minute: number;
  enabled: boolean;
}

export async function getWakeOnLanSchedule(): Promise<WakeOnLanSchedule> {
  try {
    return await invoke<WakeOnLanSchedule>("plugin:wake-on-lan|get_schedule");
  } catch (error) {
    console.error("Failed to get Wake-on-LAN schedule:", error);
    throw error;
  }
}

export async function setWakeOnLanSchedule(
  schedule: WakeOnLanSchedule,
): Promise<void> {
  try {
    await invoke("plugin:wake-on-lan|set_schedule", { schedule });
  } catch (error) {
    console.error("Failed to set Wake-on-LAN schedule:", error);
    throw error;
  }
}

export async function wake(macAddress: string): Promise<void> {
  try {
    await invoke("plugin:wake-on-lan|wake", { macAddress });
  } catch (error) {
    console.error("Failed to send magic packet:", error);
    throw error;
  }
}
