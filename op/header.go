package op

// HeaderStructSize returns the size of the header struct.
// Similar to unsafe.Sizeof, but allow disabling alignment,
// and use hardcoded values instead of the dynamic ones based
// on the current system/architecture.
// Return the full size, the size of the name and comment fields.
func HeaderStructSize() (headerSize, nameLength, commentLength int) {
	align := 4      // Align on 4 bytes.
	headerSize += 4 // magic number.

	nameLength = ProgNameLength + 1
	if n := nameLength % align; n != 0 {
		nameLength += (align - n)
	}
	headerSize += nameLength

	headerSize += 4 // prog size.

	commentLength = CommentLength + 1
	if n := commentLength % align; n != 0 {
		commentLength += (align - n)
	}
	headerSize += commentLength

	return headerSize, nameLength, commentLength
}
