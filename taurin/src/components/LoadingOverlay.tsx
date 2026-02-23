import {h} from "preact";

const LoadingOverlay = ({onCancel}: { onCancel?: () => void }) => {
    return h("div", {class: "absolute inset-0 flex flex-col items-center justify-center z-10 bg-gray-500 bg-opacity-50"}, [
        h("div", {class: "w-12 h-12 border-4 border-t-blue-500 border-gray-300 rounded-full animate-spin"}),
        onCancel && h("button", {
            onClick: onCancel,
            class: "mt-4 bg-gray-600 hover:bg-gray-700 text-white font-bold py-2 px-4 rounded",
        }, "Cancel"),
    ]);
};

export default LoadingOverlay;
