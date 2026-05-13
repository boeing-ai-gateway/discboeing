package controlsocket

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/coder/websocket"
)

const (
	maxFrameReadBytes = 16 * 1024 * 1024

	binaryFrameHeaderLen = 6
)

var binaryFrameMagic = [4]byte{'d', 'c', 's', 1}

// Conn serializes websocket writes and exposes a small frame API for the
// sandbox agent control socket.
type Conn struct {
	ws *websocket.Conn
	mu sync.Mutex
}

func NewConn(ws *websocket.Conn) *Conn {
	ws.SetReadLimit(maxFrameReadBytes)
	return &Conn{ws: ws}
}

func (c *Conn) ReadFrame(ctx context.Context) (Frame, error) {
	messageType, data, err := c.ws.Read(ctx)
	if err != nil {
		return Frame{}, err
	}
	if messageType == websocket.MessageBinary {
		return decodeBinaryStreamFrame(data)
	}
	var frame Frame
	if err := json.Unmarshal(data, &frame); err != nil {
		return Frame{}, err
	}
	return frame, nil
}

func (c *Conn) WriteFrame(ctx context.Context, frame Frame) error {
	if frame.Version == 0 {
		frame.Version = 1
	}
	if frame.Type == TypeStreamData {
		data, err := encodeBinaryStreamFrame(frame)
		if err != nil {
			return err
		}
		c.mu.Lock()
		defer c.mu.Unlock()
		return c.ws.Write(ctx, websocket.MessageBinary, data)
	}
	data, err := json.Marshal(frame)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ws.Write(ctx, websocket.MessageText, data)
}

func encodeBinaryStreamFrame(frame Frame) ([]byte, error) {
	if frame.Channel == "" {
		return nil, fmt.Errorf("binary stream frame channel is required")
	}
	if len(frame.Channel) > 0xffff {
		return nil, fmt.Errorf("binary stream frame channel is too long")
	}
	data := make([]byte, binaryFrameHeaderLen+len(frame.Channel)+len(frame.Data))
	copy(data[:4], binaryFrameMagic[:])
	binary.BigEndian.PutUint16(data[4:6], uint16(len(frame.Channel)))
	copy(data[6:], frame.Channel)
	copy(data[6+len(frame.Channel):], frame.Data)
	return data, nil
}

func decodeBinaryStreamFrame(data []byte) (Frame, error) {
	if len(data) < binaryFrameHeaderLen {
		return Frame{}, fmt.Errorf("binary stream frame is too short")
	}
	if string(data[:4]) != string(binaryFrameMagic[:]) {
		return Frame{}, fmt.Errorf("binary stream frame has invalid magic")
	}
	channelLen := int(binary.BigEndian.Uint16(data[4:6]))
	if channelLen == 0 {
		return Frame{}, fmt.Errorf("binary stream frame channel is required")
	}
	if len(data) < binaryFrameHeaderLen+channelLen {
		return Frame{}, fmt.Errorf("binary stream frame channel is truncated")
	}
	return Frame{
		Version: 1,
		Channel: string(data[6 : 6+channelLen]),
		Type:    TypeStreamData,
		Data:    data[6+channelLen:],
	}, nil
}

func (c *Conn) Close() error {
	return c.ws.Close(websocket.StatusNormalClosure, "done")
}

func (c *Conn) CloseOnDone(ctx context.Context) {
	go func() {
		<-ctx.Done()
		_ = c.ws.Close(websocket.StatusGoingAway, ctx.Err().Error())
	}()
}
