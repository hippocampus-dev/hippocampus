package io.github.kaidotio.hippocampus.crypto

import org.junit.Assert.assertEquals
import org.junit.Assert.fail
import org.junit.Test
import kotlin.math.pow

class RubyMarshalTest {

    private val majorVersionCode = 4.toChar().toString()
    private val minorVersionCode = 8.toChar().toString()

    @Test
    fun testDump_fixnum_zero() {
        val n = 0
        val typeFixnumCode = 'i'.toString()

        val expected = "$majorVersionCode$minorVersionCode$typeFixnumCode${0.toChar()}"
        val result = RubyMarshal.dump(n)

        assertEquals(expected, result)
    }

    @Test
    fun testDump_fixnum_short() {
        val n = 1
        val typeFixnumCode = 'i'.toString()
        val offset = 5

        val expected = "$majorVersionCode$minorVersionCode$typeFixnumCode${(n + offset).toChar()}"
        val result = RubyMarshal.dump(n)

        assertEquals(expected, result)
    }

    @Test
    fun testDump_fixnum_long() {
        val n = Int.MAX_VALUE
        val typeFixnumCode = 'i'.toString()

        val long = ByteArray(4)
        long[0] = (n and 0xff).toByte()
        long[1] = ((n shr 8) and 0xff).toByte()
        long[2] = ((n shr 16) and 0xff).toByte()
        long[3] = ((n shr 24) and 0xff).toByte()

        var index = long.size
        for (i in long.size - 1 downTo 0) {
            if (long[i].toInt() != 0) {
                index = i + 1
                break
            }
        }

        val s = StringBuilder()
        for (i in 0 until index) {
            s.append(long[i].toUByte().toInt().toChar())
        }

        val expected = "$majorVersionCode$minorVersionCode$typeFixnumCode${index.toChar()}$s"
        val result = RubyMarshal.dump(n)

        assertEquals(expected, result)
    }

    @Test
    fun testDump_string_short() {
        val s = "test"
        val typeStringCode = '"'.toString()

        val length = (s.length + 5).toChar().toString()

        val expected = "$majorVersionCode$minorVersionCode$typeStringCode$length$s"
        val result = RubyMarshal.dump(s)

        assertEquals(expected, result)
    }

    @Test
    fun testDump_string_long() {
        val s = "test".repeat(100)
        val typeStringCode = '"'.toString()

        val long = ByteArray(4)
        long[0] = (s.length and 0xff).toByte()
        long[1] = ((s.length shr 8) and 0xff).toByte()
        long[2] = ((s.length shr 16) and 0xff).toByte()
        long[3] = ((s.length shr 24) and 0xff).toByte()

        var index = long.size
        for (i in long.size - 1 downTo 0) {
            if (long[i].toInt() != 0) {
                index = i + 1
                break
            }
        }

        val length = StringBuilder()
        for (i in 0 until index) {
            length.append(long[i].toUByte().toInt().toChar())
        }

        val expected = "$majorVersionCode$minorVersionCode$typeStringCode${index.toChar()}$length$s"
        val result = RubyMarshal.dump(s)

        assertEquals(expected, result)
    }

    @Test
    fun testDump_string_multibyte() {
        val s = "テスト"
        val typeStringCode = '"'.toString()

        val length = (s.toByteArray(Charsets.UTF_8).size + 5).toChar().toString()

        val expected = "$majorVersionCode$minorVersionCode$typeStringCode$length$s"
        val result = RubyMarshal.dump(s)

        assertEquals(expected, result)
    }

    @Test
    fun testLoad_fixnum_zero() {
        val n = 0
        val typeFixnumCode = 'i'.toString()

        val result = RubyMarshal.load("$majorVersionCode$minorVersionCode$typeFixnumCode${0.toChar()}")

        assertEquals(n, result)
    }

    @Test
    fun testLoad_fixnum_short() {
        val n = 1
        val typeFixnumCode = 'i'.toString()
        val offset = 5

        val result = RubyMarshal.load("$majorVersionCode$minorVersionCode$typeFixnumCode${(n + offset).toChar()}")

        assertEquals(n, result)
    }

    @Test
    fun testLoad_fixnum_long() {
        val n = Int.MAX_VALUE
        val typeFixnumCode = 'i'.toString()

        val long = ByteArray(4)
        long[0] = (n and 0xff).toByte()
        long[1] = ((n shr 8) and 0xff).toByte()
        long[2] = ((n shr 16) and 0xff).toByte()
        long[3] = ((n shr 24) and 0xff).toByte()

        var index = long.size
        for (i in long.size - 1 downTo 0) {
            if (long[i].toInt() != 0) {
                index = i + 1
                break
            }
        }

        val length = StringBuilder()
        for (i in 0 until index) {
            length.append(long[i].toUByte().toInt().toChar())
        }

        val result = RubyMarshal.load("$majorVersionCode$minorVersionCode$typeFixnumCode${index.toChar()}$length")

        assertEquals(n, result)
    }

    @Test
    fun testLoad_string_short() {
        val s = "test"
        val typeStringCode = '"'.toString()

        val result = RubyMarshal.load("$majorVersionCode$minorVersionCode$typeStringCode${(s.length + 5).toChar()}$s")

        assertEquals(s, result)
    }

    @Test
    fun testLoad_string_long() {
        val s = "test".repeat(100)
        val typeStringCode = '"'.toString()

        val long = ByteArray(4)
        long[0] = (s.length and 0xff).toByte()
        long[1] = ((s.length shr 8) and 0xff).toByte()
        long[2] = ((s.length shr 16) and 0xff).toByte()
        long[3] = ((s.length shr 24) and 0xff).toByte()

        var index = long.size
        for (i in long.size - 1 downTo 0) {
            if (long[i].toInt() != 0) {
                index = i + 1
                break
            }
        }

        val length = StringBuilder()
        for (i in 0 until index) {
            length.append(long[i].toUByte().toInt().toChar())
        }

        val result = RubyMarshal.load("$majorVersionCode$minorVersionCode$typeStringCode${index.toChar()}$length$s")

        assertEquals(s, result)
    }

    @Test
    fun testLoad_string_multibyte() {
        val s = "テスト"
        val typeStringCode = '"'.toString()

        val result = RubyMarshal.load("$majorVersionCode$minorVersionCode$typeStringCode${(s.toByteArray(Charsets.UTF_8).size + 5).toChar()}$s")

        assertEquals(s, result)
    }
}
