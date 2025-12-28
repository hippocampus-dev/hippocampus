import {stringifyJSON} from "../../src/parser/json";
import {describe, expect, test} from "vitest";

describe("stringifyJSON", () => {
    test("simple", () => {
        expect(stringifyJSON([["key", "value"], [1, "a"], [2, "b"]])).toBe(`[{"key":1,"value":"a"},{"key":2,"value":"b"}]`);
    });

    test("with array", () => {
        expect(stringifyJSON([["key", "value", "options"], [1, "a", ["foo", "bar"]], [2, "b", []]])).toBe(`[{"key":1,"value":"a","options":["foo","bar"]},{"key":2,"value":"b","options":[]}]`);
    });

    test("with nested keys", () => {
        expect(stringifyJSON([["key", "value", "options.foo"], [1, "a", "bar"], [2, "b", ""]])).toBe(`[{"key":1,"value":"a","options":{"foo":"bar"}},{"key":2,"value":"b","options":{}}]`);
    });

    test("with multiline string", () => {
        expect(stringifyJSON([["key", "value"], [1, "a\nb"], [2, "c"]])).toBe(`[{"key":1,"value":"a\\nb"},{"key":2,"value":"c"}]`);
    });

    test("with path", () => {
        expect(stringifyJSON([["key", "value"], [1, "a"], [2, "b"]], "key")).toBe(`{"key":[{"key":1,"value":"a"},{"key":2,"value":"b"}]}`);
    });

    test("with nested path", () => {
        expect(stringifyJSON([["key", "value"], [1, "a"], [2, "b"]], "nested.key")).toBe(`{"nested":{"key":[{"key":1,"value":"a"},{"key":2,"value":"b"}]}}`);
    });
});
