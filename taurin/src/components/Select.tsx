import {h} from "preact";

const Select = ({label, value, onChange, options}: { label: string; value: string; onChange: (value: KeyboardEvent) => void; options: {value: string, label: string}[] }) => {
    return (
        h("label", {class: "text-lg flex justify-between items-center"},
            label,
            h("select", {
                value,
                onChange,
                class: "rounded-md border-4 border-orange-600 focus:ring-4 focus:ring-orange-600 p-4 ml-auto",
            }, options.map(option =>
                h("option", {value: option.value}, option.label)
            )),
        )
    );
};

export default Select;
