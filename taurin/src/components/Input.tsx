import {h} from "preact";
import {useState, useEffect} from "preact/hooks";

const Input = ({label, value, setValue}: { label: string; value: string; setValue: (value: string) => void }) => {
    const [currentValue, setCurrentValue] = useState(value);

    useEffect(() => {
        setCurrentValue(value);
    }, [value]);

    return (
        h("div", {class: "flex justify-between items-center"}, [
            h("label", {class: "text-lg flex justify-between items-center w-full mr-4"},
                label,
                h("input", {
                    type: "text",
                    value: currentValue,
                    onInput: (event: Event) => setCurrentValue((event.target as HTMLInputElement).value),
                    class: "rounded-md border-4 border-orange-600 focus:ring-4 focus:ring-orange-600 p-4 ml-auto",
                }),
            ),
            h("button", {
                onClick: () => setValue(currentValue),
                disabled: currentValue === value,
                class: "bg-blue-600 hover:bg-blue-700 text-white font-bold py-2 px-4 rounded disabled:opacity-50 disabled:cursor-not-allowed",
            }, "Save"),
            h("button", {
                onClick: () => setCurrentValue(value),
                disabled: currentValue === value,
                class: "bg-red-600 hover:bg-red-700 text-white font-bold py-2 px-4 rounded disabled:opacity-50 disabled:cursor-not-allowed",
            }, "Reset"),
        ])
    );
};

export default Input;
