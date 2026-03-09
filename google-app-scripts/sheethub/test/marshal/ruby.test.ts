import {dump, load, MAJOR_VERSION, MINOR_VERSION, OFFSET, TYPE_FIXNUM, TYPE_STRING} from "../../src/marshal/ruby";
import {describe, expect, test} from "vitest";

describe("dump", () => {
    const majorVersionCode = String.fromCharCode(MAJOR_VERSION);
    const minorVersionCode = String.fromCharCode(MINOR_VERSION);

    describe("fixnum", () => {
        const typeFixnumCode = String.fromCharCode(TYPE_FIXNUM);

        test("zero", () => {
            const n = 0;

            expect(dump(n)).toStrictEqual(`${majorVersionCode}${minorVersionCode}${typeFixnumCode}${String.fromCharCode(0)}`);
        });

        test("overflow", () => {
            const n = 2 ** 31;

            expect(() => dump(n)).toThrow("long too big to dump");
        });

        test("short", () => {
            const n = 1;

            expect(dump(n)).toStrictEqual(`${majorVersionCode}${minorVersionCode}${typeFixnumCode}${String.fromCharCode(n + OFFSET)}`);
        });

        test("long", () => {
            const n = 2 ** 31 - 1;

            const long = new Uint8Array(4);
            long[0] = n & 0xff;
            long[1] = (n >> 8) & 0xff;
            long[2] = (n >> 16) & 0xff;
            long[3] = (n >> 24) & 0xff;
            const index = long.length - Array.from(long).reverse().findIndex((value) => value !== 0);

            expect(dump(n)).toStrictEqual(`${majorVersionCode}${minorVersionCode}${typeFixnumCode}${String.fromCharCode(index)}${Array.from(long.subarray(0, index)).map((value) => String.fromCharCode(value)).join("")}`);
        });
    });

    describe("string", () => {
        const typeStringCode = String.fromCharCode(TYPE_STRING);

        test("short", () => {
            const s = "test";

            const length = String.fromCharCode(s.length + 5);

            expect(dump(s)).toStrictEqual(`${majorVersionCode}${minorVersionCode}${typeStringCode}${length}${s}`);
        });

        test("long", () => {
            const s = "test".repeat(100);

            const long = new Uint8Array(4);
            long[0] = s.length & 0xff;
            long[1] = (s.length >> 8) & 0xff;
            long[2] = (s.length >> 16) & 0xff;
            long[3] = (s.length >> 24) & 0xff;
            const index = long.length - Array.from(long).reverse().findIndex((value) => value !== 0);
            const length = Array.from(long.subarray(0, index)).map((value) => String.fromCharCode(value)).join("");

            expect(dump(s)).toStrictEqual(`${majorVersionCode}${minorVersionCode}${typeStringCode}${String.fromCharCode(index)}${length}${s}`);
        });

        test("multibyte", () => {
            const s = "テスト";

            const length = String.fromCharCode(utf8ByteLength("テ".codePointAt(0)!) + utf8ByteLength("ス".codePointAt(0)!) + utf8ByteLength("ト".codePointAt(0)!) + 5);

            expect(dump(s)).toStrictEqual(`${majorVersionCode}${minorVersionCode}${typeStringCode}${length}${s}`);
        });
    });
});

describe("load", () => {
    const majorVersionCode = String.fromCharCode(MAJOR_VERSION);
    const minorVersionCode = String.fromCharCode(MINOR_VERSION);

    describe("fixnum", () => {
        const typeFixnumCode = String.fromCharCode(TYPE_FIXNUM);

        test("zero", () => {
            const n = 0;

            expect(load(`${majorVersionCode}${minorVersionCode}${typeFixnumCode}${String.fromCharCode(0)}`)).toStrictEqual(n);
        });

        test("short", () => {
            const n = 1;

            expect(load(`${majorVersionCode}${minorVersionCode}${typeFixnumCode}${String.fromCharCode(n + OFFSET)}`)).toStrictEqual(n);
        });

        test("long", () => {
            const n = 2 ** 31 - 1;

            const long = new Uint8Array(4);
            long[0] = n & 0xff;
            long[1] = (n >> 8) & 0xff;
            long[2] = (n >> 16) & 0xff;
            long[3] = (n >> 24) & 0xff;
            const index = long.length - Array.from(long).reverse().findIndex((value) => value !== 0);

            expect(load(`${majorVersionCode}${minorVersionCode}${typeFixnumCode}${String.fromCharCode(index)}${Array.from(long.subarray(0, index)).map((value) => String.fromCharCode(value)).join("")}`)).toStrictEqual(n);
        });
    });

    describe("string", () => {
        const typeStringCode = String.fromCharCode(TYPE_STRING);

        test("short", () => {
            const s = "test";

            const length = s.length + 5;

            expect(load(`${majorVersionCode}${minorVersionCode}${typeStringCode}${String.fromCharCode(length)}${s}`)).toStrictEqual(s);
        });

        test("long", () => {
            const s = "test".repeat(100);

            const long = new Uint8Array(4);
            long[0] = s.length & 0xff;
            long[1] = (s.length >> 8) & 0xff;
            long[2] = (s.length >> 16) & 0xff;
            long[3] = (s.length >> 24) & 0xff;
            const index = long.length - Array.from(long).reverse().findIndex((value) => value !== 0);
            const length = Array.from(long.subarray(0, index)).map((value) => String.fromCharCode(value)).join("");

            expect(load(`${majorVersionCode}${minorVersionCode}${typeStringCode}${String.fromCharCode(index)}${length}${s}`)).toStrictEqual(s);
        });

        test("multibyte", () => {
            const s = "テスト";

            const length = utf8ByteLength("テ".codePointAt(0)!) + utf8ByteLength("ス".codePointAt(0)!) + utf8ByteLength("ト".codePointAt(0)!) + 5;

            expect(load(`${majorVersionCode}${minorVersionCode}${typeStringCode}${String.fromCharCode(length)}${s}`)).toStrictEqual(s);
        });
    });
});

const SurrogateMin = 0xd800;
const SurrogateMax = 0xdfff;
const Rune1Max = (1 << 7) - 1;
const Rune2Max = (1 << 11) - 1;
const Rune3Max = (1 << 16) - 1;
const MaxRune = 0x10ffff;

const utf8ByteLength = (rune: number): number => {
    if (rune < 0) {
        return -1;
    } else if (rune <= Rune1Max) {
        return 1;
    } else if (rune <= Rune2Max) {
        return 2;
    } else if (SurrogateMin <= rune && rune <= SurrogateMax) {
        return -1;
    } else if (rune <= Rune3Max) {
        return 3;
    } else if (rune <= MaxRune) {
        return 4;
    } else {
        return -1;
    }
}
