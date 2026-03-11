package diskio

import (
	"fmt"
	"os"
)

func punchHole(_ *os.File, _, _ int64) error {
	return fmt.Errorf("punch hole not supported on darwin")
}
