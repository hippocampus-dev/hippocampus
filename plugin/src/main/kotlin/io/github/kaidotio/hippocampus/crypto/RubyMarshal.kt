package io.github.kaidotio.hippocampus.crypto

import java.nio.charset.StandardCharsets

object RubyMarshal {
    private const val MAJOR_VERSION = 4
    private const val MINOR_VERSION = 8
    private const val OFFSET = 5
    private const val TYPE_STRING = '"'.code
    private const val TYPE_FIXNUM = 'i'.code
    private const val LONG_BIT = 32
    private const val BYTE_SIZE = 8

    fun dump(value: Any): String {
        val buffer = mutableListOf<Int>()

        buffer.add(MAJOR_VERSION)
        buffer.add(MINOR_VERSION)

        when (value) {
            is Int -> {
                buffer.add(TYPE_FIXNUM)
                wLong(buffer, value)
            }
            is String -> {
                buffer.add(TYPE_STRING)
                val bytes = value.toByteArray(StandardCharsets.UTF_8)
                wLong(buffer, bytes.size)
                for (char in value) {
                    buffer.add(char.code)
                }
            }
            else -> throw IllegalArgumentException("Unsupported type: ${value::class.simpleName}")
        }

        val chars = CharArray(buffer.size) { i -> buffer[i].toChar() }
        return String(chars)
    }

    fun load(s: String): Any {
        val buffer = s.map { it.code }.toMutableList()

        val majorVersion = buffer.removeAt(0)
        val minorVersion = buffer.removeAt(0)

        if (majorVersion != MAJOR_VERSION || minorVersion != MINOR_VERSION) {
            throw IllegalArgumentException(
                "incompatible marshal file format (can't be read): format version $majorVersion.$minorVersion required $MAJOR_VERSION.$MINOR_VERSION given"
            )
        }

        val t = buffer.removeAt(0)

        return when (t) {
            TYPE_FIXNUM -> rLong(buffer)
            TYPE_STRING -> {
                val length = rLong(buffer)
                val result = StringBuilder()
                var totalBytes = 0
                while (totalBytes < length && buffer.isNotEmpty()) {
                    val codeUnit = buffer.removeAt(0)

                    if (codeUnit in 0xD800..0xDBFF && buffer.isNotEmpty()) {
                        val nextCodeUnit = buffer[0]
                        if (nextCodeUnit in 0xDC00..0xDFFF) {
                            buffer.removeAt(0)
                            result.append(codeUnit.toChar())
                            result.append(nextCodeUnit.toChar())
                            totalBytes += 4
                        } else {
                            result.append(codeUnit.toChar())
                            totalBytes += 3
                        }
                    } else {
                        result.append(codeUnit.toChar())
                        totalBytes += when {
                            codeUnit <= 0x7F -> 1
                            codeUnit <= 0x7FF -> 2
                            codeUnit <= 0xFFFF -> 3
                            else -> 4
                        }
                    }
                }
                result.toString()
            }
            else -> throw IllegalArgumentException("Unsupported type: $t")
        }
    }

    private fun wLong(buffer: MutableList<Int>, x: Int) {
        if (x > 0x7fffffff || x < -0x80000000) {
            throw IllegalArgumentException("long too big to dump")
        }

        when {
            x == 0 -> buffer.add(0)
            x in 1..122 -> buffer.add(x + OFFSET)
            x in -123..-1 -> buffer.add((x - OFFSET) and 0xFF)
            else -> {
                val temporary = IntArray(LONG_BIT / BYTE_SIZE + 1)
                var remaining = x
                for (i in 1 until LONG_BIT / BYTE_SIZE + 1) {
                    temporary[i] = remaining and 0xFF
                    remaining = remaining shr BYTE_SIZE

                    if (remaining == 0) {
                        temporary[0] = i
                        break
                    }
                    if (remaining == -1) {
                        temporary[0] = -i
                        break
                    }
                }

                buffer.add(temporary[0])
                for (i in 1..kotlin.math.abs(temporary[0])) {
                    buffer.add(temporary[i])
                }
            }
        }
    }

    private fun rLong(buffer: MutableList<Int>): Int {
        if (buffer.isEmpty()) throw IllegalArgumentException("Buffer is empty")
        val c = buffer.removeAt(0)

        return when {
            c == 0 -> 0
            c in (OFFSET..127) -> c - OFFSET
            c in (128..251) -> c - 256 + OFFSET
            else -> {
                val signed = if (c > 127) c - 256 else c
                if (signed > 0) {
                    var x = 0
                    for (i in 0 until signed) {
                        if (buffer.isEmpty()) throw IllegalArgumentException("Buffer underflow reading long")
                        x = x or (buffer.removeAt(0) shl (BYTE_SIZE * i))
                    }
                    x
                } else {
                    var x = -1
                    for (i in 0 until -signed) {
                        if (buffer.isEmpty()) throw IllegalArgumentException("Buffer underflow reading long")
                        x = x and (0xFF shl (BYTE_SIZE * i)).inv()
                        x = x or (buffer.removeAt(0) shl (BYTE_SIZE * i))
                    }
                    x
                }
            }
        }
    }
}
