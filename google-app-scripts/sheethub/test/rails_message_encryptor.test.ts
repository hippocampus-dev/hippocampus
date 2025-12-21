import {describe, expect, test} from "vitest";
import {RailsMessageEncryptor} from "../src/rails_message_encryptor";

describe("encrypt", () => {
    test("AES-GCM", () => {
        const secret = "0123456789abcdef0123456789abcdef";

        const message = "test";
        const encryptor = new RailsMessageEncryptor(secret);
        encryptor.addEntropy(1024);

        const encrypted = encryptor.encrypt(message);
        const decrypted = encryptor.decrypt(encrypted);

        expect(decrypted).toStrictEqual(message);
    });
});
