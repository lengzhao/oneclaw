package memory

import (
	"bytes"
)

const truncateSuffix = "\n...(truncated for MEMORY.md byte budget)"

// TruncateMEMORYMDForInjection caps MEMORY.md at MEMORYMDMaxBytes (write-side / on-disk contract mirrored at inject time).
// It does not apply MemoryMaxRunes from [preturn.Budget] — that budget is for lengzhao/memory recall when wired.
// If truncation occurs, a suffix is appended and UTF-8 boundaries are preserved.
func TruncateMEMORYMDForInjection(src []byte) []byte {
	if len(src) <= MEMORYMDMaxBytes {
		return bytes.Clone(src)
	}
	suf := []byte(truncateSuffix)
	room := MEMORYMDMaxBytes - len(suf)
	if room <= 0 {
		if len(suf) <= MEMORYMDMaxBytes {
			return suf
		}
		return suf[:MEMORYMDMaxBytes]
	}
	out := src[:room]
	for len(out) > 0 && (out[len(out)-1]&0xC0) == 0x80 {
		out = out[:len(out)-1]
	}
	return append(out, truncateSuffix...)
}
