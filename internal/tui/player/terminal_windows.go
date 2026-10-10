package player

import (
	"net"
	"time"

	"gopkg.in/natefinch/npipe.v2"
)

func dialTerminal(socket string) (net.Conn, error) {
	return npipe.DialTimeout(socket, 100*time.Millisecond)
}
