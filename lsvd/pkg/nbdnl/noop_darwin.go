//go:build darwin

// Meet the package interface so Darwin can at least build
package nbdnl

import (
	"context"
	"net"
	"os"
)

var IndexAny uint32 = 0

func Loopback(ctx context.Context, size uint64, preferredIdx uint32) (uint32, net.Conn, *os.File, func() error, error) {
	return 0, nil, nil, nil, nil
}

func Disconnect(idx uint32) error {
	return nil
}

type DeviceStatus struct{}

func Status(idx uint32) (DeviceStatus, error) {
	return DeviceStatus{}, nil
}
