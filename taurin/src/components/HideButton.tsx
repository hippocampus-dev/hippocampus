import { XMarkIcon } from "@heroicons/react/24/outline";
import { getCurrentWindow } from "@tauri-apps/api/window";
import { h } from "preact";

const HideButton = ({ class: className }: { class: string }) => {
  return h(
    "button",
    {
      onClick: async () => {
        await getCurrentWindow().hide();
      },
      class: `${className} text-gray-600 hover:text-gray-900`,
    },
    [h(XMarkIcon, { class: "size-6", "aria-hidden": "true" })],
  );
};

export default HideButton;
