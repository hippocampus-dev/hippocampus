import {h} from "preact";

const Switch = ({label, checked, onChange}: { label: string; checked: boolean; onChange?: (checked: boolean) => void }) => {
    return (
        h("label", {class: "text-lg flex justify-between items-center"},
            label,
            h("input", {
                type: "checkbox",
                onChange: () => {
                    if (onChange) {
                        onChange(!checked);
                    }
                },
                checked: checked,
                class: "sr-only",
            }),
            h("div", {class: `w-10 h-6 rounded-full transition-colors ${checked ? "bg-blue-600" : "bg-gray-300"} cursor-pointer`}, [
                h("div", {
                    class: `w-5 h-5 bg-white rounded-full transform transition-transform ${
                        checked ? "translate-x-4" : ""
                    }`,
                }),
            ]),
        )
    );
};

export default Switch;
