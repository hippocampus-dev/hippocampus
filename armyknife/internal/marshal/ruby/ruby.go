package ruby

import (
	"errors"
	"fmt"
	"unicode/utf8"
)

const (
	MajorVersion = 4
	MinorVersion = 8

	Offset     = 5
	TypeString = '"'
	TypeFixnum = 'i'

	LongBit  = 32
	ByteSize = 8
)

// https://github.com/ruby/ruby/blob/v3_4_1/marshal.c#L1178
func Dump(object interface{}) (string, error) {
	var buffer []int

	buffer = append(buffer, MajorVersion, MinorVersion)

	if err := wObject(&buffer, object); err != nil {
		return "", err
	}

	chars := make([]rune, len(buffer))
	for i, b := range buffer {
		chars[i] = rune(b)
	}
	return string(chars), nil
}

// https://github.com/ruby/ruby/blob/v3_4_1/marshal.c#L302
func wLong(buffer *[]int, x int) error {
	if x > 0x7fffffff || x < -0x80000000 {
		return errors.New("long too big to dump")
	}

	if x == 0 {
		*buffer = append(*buffer, 0)
		return nil
	}
	if 0 < x && x < 123 {
		*buffer = append(*buffer, x+Offset)
		return nil
	}
	if -124 < x && x < 0 {
		*buffer = append(*buffer, (x-Offset)&0xff)
		return nil
	}

	temporary := make([]byte, LongBit/ByteSize+1)
	for i := 1; i < LongBit/ByteSize+1; i++ {
		temporary[i] = byte(x & 0xff)
		x >>= ByteSize

		if x == 0 {
			temporary[0] = byte(i)
			break
		}
		if x == -1 {
			temporary[0] = byte(-i)
			break
		}
	}

	for i := 0; i <= int(temporary[0]); i++ {
		*buffer = append(*buffer, int(temporary[i]))
	}

	return nil
}

// https://github.com/ruby/ruby/blob/v3_4_1/marshal.c#L813
func wObject(buffer *[]int, object interface{}) error {
	switch v := object.(type) {
	// https://github.com/ruby/ruby/blob/v3_4_1/marshal.c#L834
	case int:
		*buffer = append(*buffer, TypeFixnum)
		return wLong(buffer, v)
	// https://github.com/ruby/ruby/blob/v3_4_1/marshal.c#L994
	case string:
		*buffer = append(*buffer, TypeString)
		if err := wLong(buffer, len(v)); nil != err {
			return err
		}
		for _, c := range v {
			*buffer = append(*buffer, int(c))
		}
		return nil
	default:
		return errors.New(fmt.Sprintf("Unsupported type: %T", v))
	}
}

// https://github.com/ruby/ruby/blob/v3_4_1/marshal.c#L1354
func rLong(buffer *[]int) (int, error) {
	c := (*buffer)[0]
	*buffer = (*buffer)[1:]

	if c == 0 {
		return 0, nil
	}
	if -1+Offset < c && c < 123+Offset {
		return c - Offset, nil
	}
	if -124-Offset < c && c < 1-Offset {
		return c + Offset, nil
	}

	if c > 0 {
		x := 0
		for i := 0; i < c; i++ {
			x |= (*buffer)[0] << (ByteSize * i)
			*buffer = (*buffer)[1:]
		}
		return x, nil
	} else {
		x := -1
		for i := 0; i < -c; i++ {
			x &= ^(0xff << (ByteSize * i))
			x |= (*buffer)[0] << (ByteSize * i)
			*buffer = (*buffer)[1:]
		}
		return x, nil
	}
}

// https://github.com/ruby/ruby/blob/v3_4_1/marshal.c#L2281
func rObject(buffer *[]int) (interface{}, error) {
	t := (*buffer)[0]
	*buffer = (*buffer)[1:]

	switch t {
	// https://github.com/ruby/ruby/blob/v3_4_1/marshal.c#L1914
	case TypeFixnum:
		return rLong(buffer)
	// https://github.com/ruby/ruby/blob/v3_4_1/marshal.c#L1984
	case TypeString:
		length, err := rLong(buffer)
		if err != nil {
			return nil, err
		}
		var chars []rune
		var totalBytes int
		for totalBytes < length {
			r := rune((*buffer)[0])
			chars = append(chars, r)
			totalBytes += utf8.RuneLen(r)
			*buffer = (*buffer)[1:]
		}
		return string(chars), nil
	default:
		return nil, errors.New(fmt.Sprintf("Unsupported type: %d", t))
	}
}

// https://github.com/ruby/ruby/blob/v3_4_1/marshal.c#L2308
func Load(s string) (interface{}, error) {
	buffer := make([]int, 0)
	for _, c := range s {
		buffer = append(buffer, int(c))
	}

	majorVersion := buffer[0]
	minorVersion := buffer[1]
	buffer = buffer[2:]

	if majorVersion != MajorVersion || minorVersion != MinorVersion {
		return nil, errors.New(fmt.Sprintf("incompatible marshal file format (can't be read): format version %d.%d required %d.%d given",
			majorVersion, minorVersion, MajorVersion, MinorVersion))
	}

	return rObject(&buffer)
}
