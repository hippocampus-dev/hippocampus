package io.github.kaidotio.hippocampus.crypto

import java.nio.charset.StandardCharsets
import java.security.SecureRandom
import java.util.Base64
import javax.crypto.Cipher
import javax.crypto.spec.GCMParameterSpec
import javax.crypto.spec.SecretKeySpec

class RailsMessageEncryptor {
    companion object {
        private const val AuthTagLength = 16
        private const val AuthTagBits = AuthTagLength * 8
        private const val Separator = "--"
        private const val Algorithm = "AES/GCM/NoPadding"
        private const val KeyAlgorithm = "AES"
        private const val MasterKeyEnvVar = "RAILS_MASTER_KEY"
    }

    private val keyBytes: ByteArray = hexStringToByteArray(getMasterKey())

    private fun getMasterKey(): String {
        return System.getProperty(MasterKeyEnvVar)
            ?: System.getenv(MasterKeyEnvVar)
            ?: throw IllegalStateException("Environment variable $MasterKeyEnvVar is not set")
    }

    fun encrypt(plainText: String): String {
        val serialized = RubyMarshal.dump(plainText)

        val cipher = Cipher.getInstance(Algorithm)
        val keySpec = SecretKeySpec(keyBytes, KeyAlgorithm)
        val iv = ByteArray(12)
        SecureRandom().nextBytes(iv)

        val gcmSpec = GCMParameterSpec(AuthTagBits, iv)
        cipher.init(Cipher.ENCRYPT_MODE, keySpec, gcmSpec)
        val serializedBytes = serialized.toByteArray(StandardCharsets.UTF_8)
        val encrypted = cipher.doFinal(serializedBytes)

        return listOf(
            Base64.getEncoder().encodeToString(encrypted.sliceArray(0 until encrypted.size - AuthTagLength)),
            Base64.getEncoder().encodeToString(iv),
            Base64.getEncoder().encodeToString(encrypted.sliceArray(encrypted.size - AuthTagLength until encrypted.size))
        ).joinToString(Separator)
    }

    fun decrypt(encryptedMessage: String): String {
        val parts = encryptedMessage.split(Separator)
        if (parts.size != 3) {
            throw IllegalArgumentException("Invalid encrypted message format")
        }

        val encryptedData = Base64.getDecoder().decode(parts[0])
        val iv = Base64.getDecoder().decode(parts[1])
        val authTag = Base64.getDecoder().decode(parts[2])

        val cipherText = encryptedData + authTag

        val cipher = Cipher.getInstance(Algorithm)
        val keySpec = SecretKeySpec(keyBytes, KeyAlgorithm)
        val gcmSpec = GCMParameterSpec(AuthTagBits, iv)
        cipher.init(Cipher.DECRYPT_MODE, keySpec, gcmSpec)
        val decrypted = cipher.doFinal(cipherText)

        val decryptedString = String(decrypted, StandardCharsets.UTF_8)
        return RubyMarshal.load(decryptedString) as String
    }

    private fun hexStringToByteArray(hex: String): ByteArray {
        val len = hex.length
        val data = ByteArray(len / 2)
        var i = 0
        while (i < len) {
            data[i / 2] = ((Character.digit(hex[i], 16) shl 4) +
                    Character.digit(hex[i + 1], 16)).toByte()
            i += 2
        }
        return data
    }
}
