package io.github.kaidotio.hippocampus.crypto

import java.nio.charset.StandardCharsets

object RubyMarshal {
    private const val MajorVersion = 4
    private const val MinorVersion = 8
    private const val Offset = 5
    private const val TypeString = '"'.code
    private const val TypeFixnum = 'i'.code
    private const val LongBit = 32
    private const val ByteSize = 8

    fun dump(obj: Any): String {
        val buffer = mutableListOf<Int>()

        buffer.add(MajorVersion)
        buffer.add(MinorVersion)

        wObject(buffer, obj)

        val chars = CharArray(buffer.size) { i -> buffer[i].toChar() }
        return String(chars)
    }

    fun load(s: String): Any {
        val buffer = s.map { it.code }.toMutableList()

        val majorVersion = buffer.removeAt(0)
        val minorVersion = buffer.removeAt(0)

        if (majorVersion != MajorVersion || minorVersion != MinorVersion) {
            throw IllegalArgumentException(
                "incompatible marshal file format (can't be read): format version $majorVersion.$minorVersion required $MajorVersion.$MinorVersion given"
            )
        }

        return rObject(buffer)
    }

    private fun wLong(buffer: MutableList<Int>, x: Int) {
        if (x > 0x7fffffff || x < -0x80000000) {
            throw IllegalArgumentException("long too big to dump")
        }

        when {
            x == 0 -> buffer.add(0)
            x in 1..122 -> buffer.add(x + Offset)
            x in -123..-1 -> buffer.add((x - Offset) and 0xFF)
            else -> {
                val temporary = IntArray(LongBit / ByteSize + 1)
                var temp = x
                for (i in 1 until LongBit / ByteSize + 1) {
                    temporary[i] = temp and 0xFF
                    temp = temp shr ByteSize

                    if (temp == 0) {
                        temporary[0] = i
                        break
                    }
                    if (temp == -1) {
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

    private fun wObject(buffer: MutableList<Int>, obj: Any) {
        when (obj) {
            is Int -> {
                buffer.add(TypeFixnum)
                wLong(buffer, obj)
            }
            is String -> {
                buffer.add(TypeString)
                val bytes = obj.toByteArray(StandardCharsets.UTF_8)
                wLong(buffer, bytes.size)
                for (char in obj) {
                    buffer.add(char.code)
                }
            }
            else -> throw IllegalArgumentException("Unsupported type: ${obj::class.simpleName}")
        }
    }

    private fun rLong(buffer: MutableList<Int>): Int {
        if (buffer.isEmpty()) throw IllegalArgumentException("Buffer is empty")
        val c = buffer.removeAt(0)

        return when {
            c == 0 -> 0
            c in (Offset..127) -> c - Offset
            c in (128..251) -> c - 256 + Offset
            else -> {
                val signed = if (c > 127) c - 256 else c
                if (signed > 0) {
                    var x = 0
                    for (i in 0 until signed) {
                        if (buffer.isEmpty()) throw IllegalArgumentException("Buffer underflow reading long")
                        x = x or (buffer.removeAt(0) shl (ByteSize * i))
                    }
                    x
                } else {
                    var x = -1
                    for (i in 0 until -signed) {
                        if (buffer.isEmpty()) throw IllegalArgumentException("Buffer underflow reading long")
                        x = x and (0xFF shl (ByteSize * i)).inv()
                        x = x or (buffer.removeAt(0) shl (ByteSize * i))
                    }
                    x
                }
            }
        }
    }

    private fun rObject(buffer: MutableList<Int>): Any {
        val t = buffer.removeAt(0)

        return when (t) {
            TypeFixnum -> rLong(buffer)
            TypeString -> {
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
}
