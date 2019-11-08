package tunpair

import (
	"encoding/binary"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

// Pair pair
type Pair struct {
	uuid string

	masterConn *websocket.Conn
	slaveConn  *websocket.Conn

	masterWaitingPing int
	slaveWaitingPing  int

	masterWriteLock sync.Mutex
	slaveWriteLock  sync.Mutex

	dev *Device

	pch chan bool
}

func newPair(uuid string, dev *Device, c *websocket.Conn) *Pair {
	pair := &Pair{
		uuid:       uuid,
		masterConn: c,
		dev:        dev,
		pch:        make(chan bool, 1),
	}

	// ping/pong handlers
	c.SetPingHandler(func(data string) error {
		pair.writeMaster(websocket.PongMessage, []byte(data))
		return nil
	})

	c.SetPongHandler(func(data string) error {
		pair.masterWaitingPing = 0
		return nil
	})

	return pair
}

func (p *Pair) loopMaster() {
	from := p.masterConn

	for {
		_, message, err := from.ReadMessage()
		if err != nil {
			log.Println("loopMaster read error:", err)
			break
		}

		p.writeSlave(websocket.BinaryMessage, message)
	}

	p.closeMaster()
	p.closeSlave()
}

func (p *Pair) loopSlave() {
	from := p.slaveConn

	for {
		_, message, err := from.ReadMessage()
		if err != nil {
			log.Println("loopSlave read error:", err)
			break
		}

		p.writeMaster(websocket.BinaryMessage, message)
	}

	p.closeMaster()
	p.closeSlave()
}

func (p *Pair) onSlaveConneted(c *websocket.Conn) {
	// ping/pong handlers
	c.SetPingHandler(func(data string) error {
		p.writeMaster(websocket.PongMessage, []byte(data))
		return nil
	})

	c.SetPongHandler(func(data string) error {
		p.masterWaitingPing = 0
		return nil
	})

	p.slaveConn = c
}

func (p *Pair) sendReq(port uint16) {
	bs := make([]byte, 3+len(p.uuid))
	bs[0] = 0 // 0, pair request
	binary.LittleEndian.PutUint16(bs[1:], port)
	copy(bs[3:], []byte(p.uuid))

	p.dev.write(bs)
}

func (p *Pair) writeMaster(mt int, message []byte) {
	if p.masterConn == nil {
		return
	}

	p.masterWriteLock.Lock()
	p.masterConn.WriteMessage(mt, message)
	p.masterWriteLock.Unlock()
}

func (p *Pair) writeSlave(mt int, message []byte) {
	if p.slaveConn == nil {
		return
	}

	p.slaveWriteLock.Lock()
	p.slaveConn.WriteMessage(mt, message)
	p.slaveWriteLock.Unlock()
}

func (p *Pair) closeMaster() {
	if p.masterConn == nil {
		return
	}

	p.masterWriteLock.Lock()
	p.masterConn.Close()
	p.masterWriteLock.Unlock()
}

func (p *Pair) closeSlave() {
	if p.slaveConn == nil {
		return
	}

	p.slaveWriteLock.Lock()
	p.slaveConn.Close()
	p.slaveWriteLock.Unlock()
}

func (p *Pair) keepalive() {
	p.keepaliveMaster()
	p.keepaliveSlave()
}

func (p *Pair) keepaliveMaster() {
	conn := p.masterConn
	if conn == nil {
		return
	}

	if p.masterWaitingPing > 3 {
		conn.Close()
		return
	}

	p.masterWriteLock.Lock()
	now := time.Now().Unix()
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(now))
	conn.WriteMessage(websocket.PingMessage, b)
	p.masterWriteLock.Unlock()

	p.masterWaitingPing++
}

func (p *Pair) keepaliveSlave() {
	conn := p.slaveConn
	if conn == nil {
		return
	}

	if p.slaveWaitingPing > 3 {
		conn.Close()
		return
	}

	p.slaveWriteLock.Lock()
	now := time.Now().Unix()
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(now))
	conn.WriteMessage(websocket.PingMessage, b)
	p.slaveWriteLock.Unlock()

	p.slaveWaitingPing++
}
