import { h } from "https://cdn.skypack.dev/preact@10.22.1";
import {
  useEffect,
  useRef,
} from "https://cdn.skypack.dev/preact@10.22.1/hooks";

const SessionContinueDialog = ({
  show,
  timeoutSeconds,
  returnFocusRef,
  onContinue,
  onEnd,
}) => {
  const dialogRef = useRef(null);
  const continueButtonRef = useRef(null);

  useEffect(() => {
    const dialog = dialogRef.current;
    if (!dialog) return;

    if (show) {
      if (!dialog.open) {
        dialog.showModal();
      }
      continueButtonRef.current?.focus();
    } else if (dialog.open) {
      dialog.close();
      returnFocusRef?.current?.focus();
    }
  }, [show]);

  return h(
    "dialog",
    {
      ref: dialogRef,
      onCancel: (event) => {
        event.preventDefault();
        onEnd();
      },
      "aria-labelledby": "session-continue-title",
      "aria-describedby": "session-continue-description",
      class:
        "w-96 max-w-full bg-white rounded-lg shadow-lg p-6 backdrop:bg-black/50",
    },
    [
      h(
        "h2",
        {
          id: "session-continue-title",
          class: "text-xl font-bold text-gray-800 mb-2",
        },
        "Continue this session?",
      ),
      h(
        "p",
        {
          id: "session-continue-description",
          class: "text-sm text-gray-600 mb-6",
        },
        `This session is about to reach its time limit. Continuing reconnects with the conversation so far. The session ends automatically if you do not respond within ${timeoutSeconds} seconds.`,
      ),
      h("div", { class: "flex justify-end gap-2" }, [
        h(
          "button",
          {
            type: "button",
            onClick: () => onEnd(),
            class:
              "border border-gray-300 text-gray-700 hover:bg-gray-100 px-4 py-2 rounded-md transition-colors",
          },
          "End",
        ),
        h(
          "button",
          {
            type: "button",
            ref: continueButtonRef,
            onClick: () => onContinue(),
            class:
              "bg-orange-500 hover:bg-orange-600 text-white px-4 py-2 rounded-md transition-colors",
          },
          "Continue",
        ),
      ]),
    ],
  );
};

export default SessionContinueDialog;
