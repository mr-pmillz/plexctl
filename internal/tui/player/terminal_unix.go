//go:build !windows

package player

import (
	"net"
	"time"
)

func dialTerminal(socket string) (net.Conn, error) {
	return net.DialTimeout("unix", socket, 100*time.Millisecond)
}
