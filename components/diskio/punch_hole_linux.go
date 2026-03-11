package diskio

import (
	"os"
	"syscall"
)

func punchHole(f *os.File, offset, length int64) error {
	const (
		fallocPunchHole = 0x02 // FALLOC_FL_PUNCH_HOLE
		fallocKeepSize  = 0x01 // FALLOC_FL_KEEP_SIZE
	)
	return syscall.Fallocate(int(f.Fd()), fallocPunchHole|fallocKeepSize, offset, length)
}
