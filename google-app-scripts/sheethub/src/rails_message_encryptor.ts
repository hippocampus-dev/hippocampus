import {dump, load} from "./marshal/ruby";
// @ts-ignore
import sjcl from "sjcl";

const AUTH_TAG_LENGTH = 16 * 8;
const SEPARATOR = "--";

enum Cipher {
    AES_GCM = "AES-GCM",
}

class RailsMessageEncryptor {
    private readonly secret: string;
    private readonly cipher: Cipher;

    constructor(secret: string, options: { cipher?: Cipher } = {}) {
        this.secret = secret;
        this.cipher = options.cipher || Cipher.AES_GCM;
    }

    public addEntropy(size: number): void {
        if (typeof window !== 'undefined' && window.crypto) {
            sjcl.random.addEntropy(new Uint32Array(32), size, "crypto.getRandomValues");
        } else if (typeof global !== 'undefined' && global.crypto) {
            sjcl.random.addEntropy(new Uint32Array(32), size, "crypto.getRandomValues");
        } else {
            sjcl.random.addEntropy(new Uint32Array(32), size);
            console.warn("No secure random number generator available.");
        }
    }

    public encrypt(value: string): string {
        switch (this.cipher) {
            case Cipher.AES_GCM:
                const iv = sjcl.random.randomWords(3); // 12
                const result = sjcl.mode.gcm.encrypt(
                    new sjcl.cipher.aes(sjcl.codec.hex.toBits(this.secret)),
                    sjcl.codec.utf8String.toBits(dump(value)),
                    iv,
                    [],
                    AUTH_TAG_LENGTH,
                );
                const l = sjcl.bitArray.bitLength(result);
                const tag = sjcl.bitArray.bitSlice(result, l - AUTH_TAG_LENGTH);
                const data = sjcl.bitArray.bitSlice(result, 0, l - AUTH_TAG_LENGTH);

                return [
                    sjcl.codec.base64.fromBits(data),
                    sjcl.codec.base64.fromBits(iv),
                    sjcl.codec.base64.fromBits(tag)
                ].join(SEPARATOR);
            default:
                throw new Error(`Unsupported cipher: ${this.cipher}`);
        }
    }

    public decrypt(message: string): string {
        switch (this.cipher) {
            case Cipher.AES_GCM:
                const [data, iv, tag] = message.split(SEPARATOR);
                if (!data || !iv || !tag) {
                    throw new Error("Invalid message format.");
                }

                const result = sjcl.codec.utf8String.fromBits(sjcl.mode.gcm.decrypt(
                    new sjcl.cipher.aes(sjcl.codec.hex.toBits(this.secret)),
                    sjcl.bitArray.concat(sjcl.codec.base64.toBits(data), sjcl.codec.base64.toBits(tag)),
                    sjcl.codec.base64.toBits(iv),
                    [],
                    AUTH_TAG_LENGTH,
                ));

                return load(result) as string;
            default:
                throw new Error(`Unsupported cipher: ${this.cipher}`);
        }
    }
}

export {RailsMessageEncryptor, Cipher};
