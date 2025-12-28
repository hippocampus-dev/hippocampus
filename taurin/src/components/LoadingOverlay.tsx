import {h} from "preact";

const LoadingOverlay = () => {
    return h("div", {class: "absolute inset-0 flex items-center justify-center z-10 bg-gray-500 bg-opacity-50"}, [
        h("div", {class: "w-12 h-12 border-4 border-t-blue-500 border-gray-300 rounded-full animate-spin"}),
    ]);
};

export default LoadingOverlay;
