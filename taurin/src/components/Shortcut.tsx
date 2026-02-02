import {h} from "preact";
import {useState, useEffect} from "preact/hooks";

const ShortcutSetting = ({label, value, setValue}: { label: string; value: string; setValue: (value: string) => void }) => {
    const [currentValue, setCurrentValue] = useState(value);
    const [pressedKeys, setPressedKeys] = useState(new Set<string>());

    useEffect(() => {
        setCurrentValue(value);
    }, [value]);

    return (
        h("div", {class: "flex justify-between items-center"}, [
            h("label", {class: "text-lg flex justify-between items-center w-full mr-4"},
                label,
                h('input', {
                    type: 'text',
                    readOnly: true,
                    value: currentValue,
                    onKeyDown: (event) => {
                        event.preventDefault();

                        const newPressedKeys = pressedKeys;
                        newPressedKeys.add(event.key);
                        setPressedKeys(newPressedKeys);

                        const keys = [];
                        const modifiers = [];
                        for (const key of newPressedKeys) {
                            switch (key) {
                                case "Control":
                                case "Meta":
                                case "Alt":
                                case "Shift":
                                    modifiers.push(key);
                                    break;
                                default:
                                    if (key === " ") {
                                        keys.push("space");
                                        break;
                                    }
                                    keys.push(key.toLowerCase());
                                    break;
                            }
                        }

                        if (keys.length === 0) {
                            return;
                        }

                        if (modifiers.length === 0) {
                            setCurrentValue(keys[keys.length - 1]);
                            return;
                        }

                        setCurrentValue(modifiers.join("+") + "+" + keys[keys.length - 1]);
                    },
                    onKeyUp: (event) => {
                        const newPressedKeys = pressedKeys;
                        newPressedKeys.delete(event.key);
                        setPressedKeys(newPressedKeys);
                    },
                    class: "rounded-md border-4 border-orange-600 focus:ring-4 focus:ring-orange-600 p-4 ml-auto",
                }),
            ),
            h("button", {
                onClick: () => setValue(currentValue),
                disabled: currentValue === value,
                class: "bg-blue-600 hover:bg-blue-700 text-white font-bold py-2 px-4 rounded disabled:opacity-50 disabled:cursor-not-allowed"
            }, "Save"),
            h("button", {
                onClick: () => setCurrentValue(value),
                disabled: currentValue === value,
                class: "bg-red-600 hover:bg-red-700 text-white font-bold py-2 px-4 rounded disabled:opacity-50 disabled:cursor-not-allowed",
            }, "Reset"),
        ])
    );
};

export default ShortcutSetting;
