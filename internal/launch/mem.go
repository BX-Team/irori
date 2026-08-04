package launch

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseMemMB accepts JVM-style sizes ("4G", "8192M", "4096") and returns
// megabytes. A bare number is treated as megabytes.
func ParseMemMB(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty memory size")
	}
	mult := 1
	last := s[len(s)-1]
	switch last {
	case 'g', 'G':
		mult, s = 1024, s[:len(s)-1]
	case 'm', 'M':
		mult, s = 1, s[:len(s)-1]
	case 'k', 'K':
		mult, s = 0, s[:len(s)-1]
	}
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0, fmt.Errorf("invalid memory size %q", s)
	}
	if mult == 0 {
		return n / 1024, nil
	}
	return n * mult, nil
}

func FormatMemMB(mb int) string {
	if mb%1024 == 0 && mb >= 1024 {
		return strconv.Itoa(mb/1024) + "G"
	}
	return strconv.Itoa(mb) + "M"
}
