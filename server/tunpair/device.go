package tunpair

import (
	"encoding/binary"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

// Device a device, identify with it's uuid
type Device struct {
	uuid string

	wsWriteLock sync.Mutex
	conn        *websocket.Conn
	wg          sync.WaitGroup

	waitingPingCount int
}

func newDevice(uuid string, conn *websocket.Conn) *Device {
	d := &Device{
		uuid: uuid,
		conn: conn,
	}

	// ping/pong handlers
	conn.SetPingHandler(func(data string) error {
		d.writePong([]byte(data))
		return nil
	})

	conn.SetPongHandler(func(data string) error {
		d.onPong([]byte(data))
		return nil
	})

	return d
}

func (d *Device) close() {
	if d.conn != nil {
		d.conn.Close()
		d.conn = nil
	}
}

func (d *Device) loopMsg() {
	c := d.conn
	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			break
		}

		log.Println("recv dev msg length: ", len(message))
	}

	d.close()
}

func (d *Device) writePong(data []byte) {
	if d.conn == nil {
		return
	}

	d.wsWriteLock.Lock()
	d.conn.WriteMessage(websocket.PongMessage, data)
	d.wsWriteLock.Unlock()
}

func (d *Device) onPong(data []byte) {
	d.waitingPingCount = 0
}

func (d *Device) write(data []byte) {
	if d.conn == nil {
		return
	}

	d.wsWriteLock.Lock()
	d.conn.WriteMessage(websocket.BinaryMessage, data)
	d.wsWriteLock.Unlock()
}

func (d *Device) keepalive() {
	if d.conn == nil {
		return
	}

	if d.waitingPingCount > 3 {
		if d.conn != nil {
			d.conn.Close()
		}
		return
	}

	d.wsWriteLock.Lock()
	now := time.Now().Unix()
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(now))
	d.conn.WriteMessage(websocket.PingMessage, b)
	d.wsWriteLock.Unlock()

	d.waitingPingCount++
}
