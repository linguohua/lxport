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
	// unique identifier
	uuid string

	// client websocket from endpoint-c
	masterConn *websocket.Conn
	// protect websocket conn cocurrently writing
	masterWriteLock sync.Mutex
	// ping meesage that waiting for response counter
	masterWaitingPing int

	// server websocket from endpoint-s
	slaveConn *websocket.Conn
	// protect websocket conn cocurrently writing
	slaveWriteLock sync.Mutex
	// ping meesage that waiting for response counter
	slaveWaitingPing int

	// device that own this pair
	dev *Device

	// a channel use to notify pair established
	pch chan struct{}
}

func newPair(uuid string, dev *Device, master *websocket.Conn) *Pair {
	pair := &Pair{
		uuid:       uuid,
		masterConn: master,
		dev:        dev,
		pch:        make(chan struct{}, 1),
	}

	// ping/pong handlers
	master.SetPingHandler(func(data string) error {
		pair.writeMaster(websocket.PongMessage, []byte(data))
		return nil
	})

	master.SetPongHandler(func(data string) error {
		pair.masterWaitingPing = 0
		return nil
	})

	return pair
}

// loopMaster read master websocket message and forward to slave websocket
func (p *Pair) loopMaster() {
	from := p.masterConn

	for {
		_, message, err := from.ReadMessage()
		if err != nil {
			log.Println("loopMaster read error:", err)
			break
		}

		// bridge
		p.writeSlave(websocket.BinaryMessage, message)
	}

	p.closeMaster()
	p.closeSlave()
}

// loopSlave read slave websocket message and forward to master websocket
func (p *Pair) loopSlave() {
	from := p.slaveConn

	for {
		_, message, err := from.ReadMessage()
		if err != nil {
			log.Println("loopSlave read error:", err)
			break
		}

		// bridge
		p.writeMaster(websocket.BinaryMessage, message)
	}

	p.closeMaster()
	p.closeSlave()
}

// onSlaveConneted slave websocket has connected
func (p *Pair) onSlaveConneted(slave *websocket.Conn) {
	// ping/pong handlers
	slave.SetPingHandler(func(data string) error {
		p.writeSlave(websocket.PongMessage, []byte(data))
		return nil
	})

	slave.SetPongHandler(func(data string) error {
		p.slaveWaitingPing = 0
		return nil
	})

	p.slaveConn = slave
}

// sendPairCreateReq send pair create request to target device
func (p *Pair) sendPairCreateReq(port uint16) {
	// packet
	bs := make([]byte, 3+len(p.uuid))
	bs[0] = 0 // 0, pair create
	binary.LittleEndian.PutUint16(bs[1:], port)
	copy(bs[3:], []byte(p.uuid))

	// send to target device
	p.dev.write(websocket.BinaryMessage, bs)
}

// writeMaster write to master websocket
func (p *Pair) writeMaster(mt int, message []byte) {
	if p.masterConn == nil {
		return
	}

	p.masterWriteLock.Lock()
	p.masterConn.WriteMessage(mt, message)
	p.masterWriteLock.Unlock()
}

// writeSlave write to slave websocket
func (p *Pair) writeSlave(mt int, message []byte) {
	if p.slaveConn == nil {
		return
	}

	p.slaveWriteLock.Lock()
	p.slaveConn.WriteMessage(mt, message)
	p.slaveWriteLock.Unlock()
}

// closeMaster close master websocket
func (p *Pair) closeMaster() {
	if p.masterConn == nil {
		return
	}

	p.masterWriteLock.Lock()
	p.masterConn.Close()
	p.masterWriteLock.Unlock()
}

// closeSlave close slave websocket
func (p *Pair) closeSlave() {
	if p.slaveConn == nil {
		return
	}

	p.slaveWriteLock.Lock()
	p.slaveConn.Close()
	p.slaveWriteLock.Unlock()
}

// keepalive pair keepalive both master and slave websocket
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

	// timestamp
	now := time.Now().Unix()
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(now))

	p.masterWriteLock.Lock()
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

	// timestamp
	now := time.Now().Unix()
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(now))

	p.slaveWriteLock.Lock()
	conn.WriteMessage(websocket.PingMessage, b)
	p.slaveWriteLock.Unlock()

	p.slaveWaitingPing++
}
