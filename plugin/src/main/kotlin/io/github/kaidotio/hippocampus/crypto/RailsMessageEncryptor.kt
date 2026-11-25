package io.github.kaidotio.hippocampus.crypto

import java.nio.charset.StandardCharsets
import java.security.SecureRandom
import java.util.Base64
import javax.crypto.Cipher
import javax.crypto.spec.GCMParameterSpec
import javax.crypto.spec.SecretKeySpec

class RailsMessageEncryptor {
    companion object {
        private const val AUTH_TAG_LENGTH = 16
        private const val AUTH_TAG_BITS = AUTH_TAG_LENGTH * 8
        private const val SEPARATOR = "--"
        private const val ALGORITHM = "AES/GCM/NoPadding"
        private const val KEY_ALGORITHM = "AES"
        private const val MASTER_KEY_ENVIRONMENT_VARIABLE = "RAILS_MASTER_KEY"
    }

    private val keyBytes: ByteArray = run {
        val hex = System.getProperty(MASTER_KEY_ENVIRONMENT_VARIABLE)
            ?: System.getenv(MASTER_KEY_ENVIRONMENT_VARIABLE)
            ?: throw IllegalStateException("Environment variable $MASTER_KEY_ENVIRONMENT_VARIABLE is not set")
        val length = hex.length
        val data = ByteArray(length / 2)
        var i = 0
        while (i < length) {
            data[i / 2] = ((Character.digit(hex[i], 16) shl 4) +
                    Character.digit(hex[i + 1], 16)).toByte()
            i += 2
        }
        data
    }

    fun encrypt(plainText: String): String {
        val serialized = RubyMarshal.dump(plainText)

        val cipher = Cipher.getInstance(ALGORITHM)
        val keySpec = SecretKeySpec(keyBytes, KEY_ALGORITHM)
        val iv = ByteArray(12)
        SecureRandom().nextBytes(iv)

        val gcmSpec = GCMParameterSpec(AUTH_TAG_BITS, iv)
        cipher.init(Cipher.ENCRYPT_MODE, keySpec, gcmSpec)
        val serializedBytes = serialized.toByteArray(StandardCharsets.UTF_8)
        val encrypted = cipher.doFinal(serializedBytes)

        return listOf(
            Base64.getEncoder().encodeToString(encrypted.sliceArray(0 until encrypted.size - AUTH_TAG_LENGTH)),
            Base64.getEncoder().encodeToString(iv),
            Base64.getEncoder().encodeToString(encrypted.sliceArray(encrypted.size - AUTH_TAG_LENGTH until encrypted.size))
        ).joinToString(SEPARATOR)
    }

    fun decrypt(encryptedMessage: String): String {
        val parts = encryptedMessage.split(SEPARATOR)
        if (parts.size != 3) {
            throw IllegalArgumentException("Invalid encrypted message format")
        }

        val encryptedData = Base64.getDecoder().decode(parts[0])
        val iv = Base64.getDecoder().decode(parts[1])
        val authTag = Base64.getDecoder().decode(parts[2])

        val cipherText = encryptedData + authTag

        val cipher = Cipher.getInstance(ALGORITHM)
        val keySpec = SecretKeySpec(keyBytes, KEY_ALGORITHM)
        val gcmSpec = GCMParameterSpec(AUTH_TAG_BITS, iv)
        cipher.init(Cipher.DECRYPT_MODE, keySpec, gcmSpec)
        val decrypted = cipher.doFinal(cipherText)

        val decryptedString = String(decrypted, StandardCharsets.UTF_8)
        return RubyMarshal.load(decryptedString) as String
    }
}
