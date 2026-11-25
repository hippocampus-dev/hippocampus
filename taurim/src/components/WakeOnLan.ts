import { h } from "preact";
import { useEffect, useState } from "preact/hooks";
import {
  getWakeOnLanSchedule,
  setWakeOnLanSchedule,
  type WakeOnLanSchedule,
  wake,
} from "../services/wakeOnLanService";

export function WakeOnLan() {
  const [schedule, setSchedule] = useState<WakeOnLanSchedule | null>(null);
  const [status, setStatus] = useState("");

  useEffect(() => {
    getWakeOnLanSchedule()
      .then(setSchedule)
      .catch(() => {});
  }, []);

  if (schedule === null) {
    return null;
  }

  const time = `${String(schedule.hour).padStart(2, "0")}:${String(schedule.minute).padStart(2, "0")}`;

  const handleSave = () => {
    setWakeOnLanSchedule(schedule)
      .then(() => setStatus("Saved"))
      .catch((error) => setStatus(String(error)));
  };

  const handleWake = () => {
    wake(schedule.mac_address)
      .then(() => setStatus("Sent"))
      .catch((error) => setStatus(String(error)));
  };

  return h(
    "div",
    { className: "flex flex-col gap-3 flex-shrink-0" },
    h(
      "h2",
      { className: "text-lg font-semibold text-white/80" },
      "Wake-on-LAN",
    ),
    h("input", {
      type: "text",
      value: schedule.mac_address,
      onInput: (event: Event) =>
        setSchedule({
          ...schedule,
          mac_address: (event.target as HTMLInputElement).value,
        }),
      placeholder: "MAC address",
      "aria-label": "MAC address",
      className: `
        w-full px-4 py-2 rounded-lg
        bg-white/10 border border-white/30
        text-white placeholder-white/50
        focus:outline-none focus:border-white/50
      `,
    }),
    h(
      "div",
      { className: "flex items-center gap-3" },
      h("input", {
        type: "time",
        value: time,
        onInput: (event: Event) => {
          const value = (event.target as HTMLInputElement).value;
          if (value === "") {
            return;
          }
          const [hour, minute] = value.split(":").map(Number);
          setSchedule({ ...schedule, hour, minute });
        },
        "aria-label": "Daily wake time",
        className: `
          px-4 py-2 rounded-lg
          bg-white/10 border border-white/30
          text-white
          focus:outline-none focus:border-white/50
        `,
      }),
      h(
        "label",
        { className: "flex items-center gap-2 text-white/80 text-sm" },
        h("input", {
          type: "checkbox",
          checked: schedule.enabled,
          onChange: (event: Event) =>
            setSchedule({
              ...schedule,
              enabled: (event.target as HTMLInputElement).checked,
            }),
        }),
        "Daily",
      ),
    ),
    h(
      "div",
      { className: "flex items-center gap-3 justify-end" },
      status && h("span", { className: "text-white/50 text-xs" }, status),
      h(
        "button",
        {
          onClick: handleWake,
          className: `
            px-4 py-2 rounded-lg
            bg-white/10 hover:bg-white/20
            text-white transition-colors
          `,
        },
        "Wake Now",
      ),
      h(
        "button",
        {
          onClick: handleSave,
          className: `
            px-4 py-2 rounded-lg
            bg-white text-primary font-medium
            transition-colors
          `,
        },
        "Save",
      ),
    ),
  );
}
