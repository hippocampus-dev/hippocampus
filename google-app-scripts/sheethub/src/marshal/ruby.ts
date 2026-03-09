export const MAJOR_VERSION = 4;
export const MINOR_VERSION = 8;

export const OFFSET = 5;
export const TYPE_STRING = '"'.charCodeAt(0);
export const TYPE_FIXNUM = 'i'.charCodeAt(0);

const LONG_BIT = 32;
const BYTE_SIZE = 8;

// https://github.com/ruby/ruby/blob/v3_4_1/marshal.c#L1178
const dump = (object: unknown): string => {
    const buffer: number[] = [];

    buffer.push(MAJOR_VERSION, MINOR_VERSION);

    w_object(buffer, object);

    return String.fromCharCode(...buffer);
};

// https://github.com/ruby/ruby/blob/v3_4_1/marshal.c#L302
const w_long = (buffer: number[], x: number): void => {
    if (x > 0x7fffffff || x < -0x80000000) {
        throw new Error("long too big to dump");
    }

    if (x === 0) {
        buffer.push(0);
        return;
    }
    if (0 < x && x < 123) {
        buffer.push(x + OFFSET);
        return;
    }
    if (-124 < x && x < 0) {
        buffer.push((x - OFFSET) & 0xff);
        return;
    }

    const temporary = new Uint8Array(LONG_BIT / BYTE_SIZE + 1);
    for (let i = 1; i < LONG_BIT / BYTE_SIZE + 1; i++) {
        temporary[i] = x & 0xff;
        x >>= BYTE_SIZE;

        if (x === 0) {
            temporary[0] = i;
            break;
        }
        if (x === -1) {
            temporary[0] = -i;
            break;
        }
    }

    buffer.push(...Array.from(temporary.subarray(0, temporary[0] + 1)));
};

// https://github.com/ruby/ruby/blob/v3_4_1/marshal.c#L813
const w_object = (buffer: number[], object: unknown): void => {
    switch (typeof object) {
        // https://github.com/ruby/ruby/blob/v3_4_1/marshal.c#L834
        case "number":
            buffer.push(TYPE_FIXNUM);
            w_long(buffer, object);
            break;
        // https://github.com/ruby/ruby/blob/v3_4_1/marshal.c#L994
        case "string":
            buffer.push(TYPE_STRING);
            let length = 0;
            const codes: number[] = [];
            for (let i = 0; i < object.length; i++) {
                const code = object.charCodeAt(i);
                codes.push(code);
                length += utf8ByteLength(code);
            }
            w_long(buffer, length);
            for (const code of codes) {
                buffer.push(code);
            }
            break;
        default:
            throw new Error(`Unsupported type: ${typeof object}`);
    }
};

// https://github.com/ruby/ruby/blob/v3_4_1/marshal.c#L1354
const r_long = (buffer: number[]): number => {
    const c = buffer.shift();

    if (c === 0) {
        return 0;
    }
    if (-1 + OFFSET < c && c < 123 + OFFSET) {
        return c - OFFSET;
    }
    if (-124 - OFFSET < c && c < 1 - OFFSET) {
        return c + OFFSET;
    }

    if (c > 0) {
        let x = 0;
        for (let i = 0; i < c; i++) {
            x |= buffer.shift() << (BYTE_SIZE * i);
        }
        return x;
    } else {
        let x = -1;
        for (let i = 0; i < -c; i++) {
            x &= ~(0xff << (BYTE_SIZE * i));
            x |= buffer.shift() << (BYTE_SIZE * i);
        }
        return x;
    }
}

// https://github.com/ruby/ruby/blob/v3_4_1/marshal.c#L2281
const r_object = (buffer: number[]): unknown => {
    const type = buffer.shift();

    switch (type) {
        // https://github.com/ruby/ruby/blob/v3_4_1/marshal.c#L1914
        case TYPE_FIXNUM:
            return r_long(buffer);
        // https://github.com/ruby/ruby/blob/v3_4_1/marshal.c#L1984
        case TYPE_STRING:
            const length = r_long(buffer);
            let chars = "";
            let totalBytes = 0;
            while (totalBytes < length) {
                const r = buffer.shift();
                chars += String.fromCharCode(r);
                totalBytes += utf8ByteLength(r);
            }
            return chars;
        default:
            throw new Error(`Unsupported type: ${type}`);
    }
}

// https://github.com/ruby/ruby/blob/v3_4_1/marshal.c#L2308
const load = (string: string): unknown => {
    const buffer = string.split("").map((char) => char.charCodeAt(0));

    const majorVersion = buffer.shift();
    const minorVersion = buffer.shift();

    if (majorVersion !== MAJOR_VERSION || minorVersion !== MINOR_VERSION) {
        throw new Error(`incompatible marshal file format (can't be read): format version ${majorVersion}.${minorVersion} required ${MAJOR_VERSION}.${MINOR_VERSION} given`);
    }

    return r_object(buffer);
};

export {dump, load};

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
