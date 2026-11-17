package io.github.kaidotio.hippocampus.crypto

import org.junit.Assert.assertEquals
import org.junit.Test

class RailsMessageEncryptorTest {
    @Test
    fun testEncryptAES_GCM() {
        val secret = "0123456789abcdef0123456789abcdef"
        
        System.setProperty("RAILS_MASTER_KEY", secret)
        
        val message = "test"
        val encryptor = RailsMessageEncryptor()
        
        val encrypted = encryptor.encrypt(message)
        val decrypted = encryptor.decrypt(encrypted)
        
        assertEquals(message, decrypted)
    }
}