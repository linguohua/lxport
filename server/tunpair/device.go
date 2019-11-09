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
	// unique identifier
	uuid string

	// device's websocket
	conn *websocket.Conn
	// write lock protect websocket conn cocurrently writing
	wsWriteLock sync.Mutex
	// use to wait for old websocket closed
	wg sync.WaitGroup
	// ping meesage that waiting for response counter
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

// close close device's websocket
func (d *Device) close() {
	if d.conn != nil {
		d.conn.Close()
		d.conn = nil
	}
}

// loopMsg read message from device websocket
func (d *Device) loopMsg() {
	c := d.conn
	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			log.Println("device read ws error:", err)
			break
		}

		// the device currently not send anything to server
		log.Println("recv dev msg length: ", len(message))
	}

	d.close()
}

// writePong write pong data to websocket
func (d *Device) writePong(data []byte) {
	if d.conn == nil {
		return
	}

	d.write(websocket.PongMessage, data)
}

// onPong reset ping counter
func (d *Device) onPong(data []byte) {
	d.waitingPingCount = 0
}

// write write message with type to websocket
func (d *Device) write(mt int, data []byte) {
	if d.conn == nil {
		return
	}

	// lock, prevent cocurrently writing
	d.wsWriteLock.Lock()
	d.conn.WriteMessage(mt, data)
	d.wsWriteLock.Unlock()
}

// keepalive send ping message to websocket
func (d *Device) keepalive() {
	if d.conn == nil {
		return
	}

	// too many un-response ping, close the websocket
	if d.waitingPingCount > 3 {
		if d.conn != nil {
			log.Println("Device keepalive failed, close:", d.uuid)
			d.conn.Close()
		}
		return
	}

	now := time.Now().Unix()
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(now))
	d.write(websocket.PingMessage, b)

	d.waitingPingCount++
}
