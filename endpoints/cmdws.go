package endpoints

import (
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/gorilla/websocket"
)

type wsholder struct {
	uuid      string
	conn      *websocket.Conn
	writeLock sync.Mutex

	waitingPingCount int
}

func newHolder(uuid string, c *websocket.Conn) *wsholder {
	wh := &wsholder{
		conn: c,
		uuid: uuid,
	}

	// ping/pong handlers
	c.SetPingHandler(func(data string) error {
		wh.write(websocket.PongMessage, []byte(data))
		return nil
	})

	c.SetPongHandler(func(data string) error {
		wh.onPong([]byte(data))
		return nil
	})

	return wh
}

func buildCmdWS() (*wsholder, error) {
	c, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return nil, err
	}

	cs := newHolder(deviceID, c)
	return cs, nil
}

func cmdwsService() {
	// never return
	for {
		cs, err := buildCmdWS()
		if err != nil {
			log.Println("reconnect later, buildCmdWS failed:", err)
			time.Sleep(15 * time.Second)
			continue
		}

		cs.loop()
	}
}

func (cs *wsholder) write(mt int, data []byte) error {
	if cs.conn == nil {
		return fmt.Errorf("wsholder write failed, no ws")
	}

	cs.writeLock.Lock()
	err := cs.conn.WriteMessage(mt, data)
	cs.writeLock.Unlock()
	return err
}

func (cs *wsholder) close() {
	if cs.conn != nil {
		cs.conn.Close()
		cs.conn = nil
	}
}

func (cs *wsholder) onPong(data []byte) {
	cs.waitingPingCount = 0
}

func (cs *wsholder) keepalive() {
	if cs.conn == nil {
		return
	}

	if cs.waitingPingCount > 3 {
		if cs.conn != nil {
			log.Println("Device keepalive failed, close:", cs.uuid)
			cs.conn.Close()
		}
		return
	}

	now := time.Now().Unix()
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(now))
	cs.write(websocket.PingMessage, b)

	cs.waitingPingCount++
}

func (cs *wsholder) loop() {
	ws := cs.conn
	wsholderMap[cs.uuid] = cs

	for {
		_, message, err := ws.ReadMessage()
		if err != nil {
			log.Println("handleRequest ws read error:", err)
			ws.Close()
			break
		}

		ops := message[0]
		switch ops {
		case 0:
			go onPairRequest(cs, message)
		default:
			log.Errorf("unsupport operation:%d", ops)
		}
	}
	delete(wsholderMap, cs.uuid)
}

func onPairRequest(cs *wsholder, message []byte) {
	// target port
	port := binary.LittleEndian.Uint16(message[1:3])
	// pair uuid
	uuid := message[3:]

	address := fmt.Sprintf("127.0.0.1:%d", port)

	conn, err := net.Dial("tcp", address)
	if err != nil {
		log.Errorf("onPairRequest connect to address:%s failed:%v", address, err)
		return
	}

	defer conn.Close()

	wsURLResp := fmt.Sprintf("%s?pt=resp&uuid=%s", wsURLRaw, string(uuid))
	ws, _, err := websocket.DefaultDialer.Dial(wsURLResp, nil)
	if err != nil {
		log.Println("onPairRequest failed connect to websocket server:", err)
		return
	}

	wh := newHolder(string(uuid), ws)
	wsholderMap[cs.uuid] = cs

	defer func() {
		wh.close()
		delete(wsholderMap, cs.uuid)
	}()

	go func() {
		for {
			_, message, err := ws.ReadMessage()
			if err != nil {
				log.Println("onPairRequest ws read error:", err)
				conn.Close()
				break
			}

			writeAll(conn, message)
			if err != nil {
				break
			}
		}
	}()

	tcpbuf := make([]byte, 8192)
	for {
		n, err := conn.Read(tcpbuf)
		if err != nil {
			log.Println("onPairRequest tcp read error:", err)
			ws.Close()
			break
		}

		err = wh.write(websocket.BinaryMessage, tcpbuf[:n])
		if err != nil {
			log.Println("onPairRequest ws write error:", err)
			break
		}
	}
}

func writeAll(conn net.Conn, buf []byte) error {
	wrote := 0
	l := len(buf)
	for {
		n, err := conn.Write(buf[wrote:])
		if err != nil {
			return err
		}

		wrote = wrote + n
		if wrote == l {
			break
		}
	}

	return nil
}
