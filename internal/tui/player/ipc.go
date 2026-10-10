package player

import (
	"encoding/json"
	"fmt"
	"net"
	"time"
)

// ipcClient is owned by PlayerManager's command loop. Every request has a
// deadline: a dead player cannot hang a control, shutdown, or the visualizer.
type ipcClient struct {
	conn    net.Conn
	encoder *json.Encoder
	decoder *json.Decoder
	nextID  uint64
}

func openIPC(socket string) (*ipcClient, error) {
	conn, err := dialTerminal(socket)
	if err != nil {
		return nil, err
	}
	return &ipcClient{conn: conn, encoder: json.NewEncoder(conn), decoder: json.NewDecoder(conn)}, nil
}

func (c *ipcClient) command(command any, result any) error {
	c.nextID++
	if err := c.conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		return err
	}
	if err := c.encoder.Encode(map[string]any{"command": command, "request_id": c.nextID}); err != nil {
		return err
	}
	for {
		var reply struct {
			ID    uint64          `json:"request_id"`
			Error string          `json:"error"`
			Data  json.RawMessage `json:"data"`
		}
		if err := c.decoder.Decode(&reply); err != nil {
			return err
		}
		if reply.ID != c.nextID {
			continue
		}
		if reply.Error != "success" {
			return fmt.Errorf("mpv: %s", reply.Error)
		}
		if result != nil && len(reply.Data) > 0 {
			return json.Unmarshal(reply.Data, result)
		}
		return nil
	}
}

func (c *ipcClient) call(args ...any) error {
	return c.command(args, nil)
}

func (c *ipcClient) get(property string, result any) error {
	return c.command([]any{"get_property", property}, result)
}
