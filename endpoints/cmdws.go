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
	// unique id, as wsholder object's identifier
	uuid string

	conn *websocket.Conn
	// protect websocket conn cocurrently writing
	writeLock sync.Mutex

	// ping meesage that waiting for response counter
	waitingPingCount int
}

// newHolder create a websocket holder object
// the uuid must be unique
func newHolder(uuid string, c *websocket.Conn) *wsholder {
	wh := &wsholder{
		conn: c,
		uuid: uuid,
	}

	// ping/pong handlers
	c.SetPingHandler(func(data string) error {
		// echo to peer
		wh.write(websocket.PongMessage, []byte(data))
		return nil
	})

	c.SetPongHandler(func(data string) error {
		wh.onPong([]byte(data))
		return nil
	})

	return wh
}

// buildCmdWS build a websocket dedicated to recv command
func buildCmdWS() (*wsholder, error) {
	c, _, err := websocket.DefaultDialer.Dial(wsURLRegister, nil)
	if err != nil {
		return nil, err
	}

	wh := newHolder(deviceID, c)
	return wh, nil
}

// cmdwsService long run service, never return
func cmdwsService() {
	// never return
	for {
		// build/re-build command websocket
		wh, err := buildCmdWS()
		if err != nil {
			log.Println("cmdwsService reconnect later, buildCmdWS failed:", err)
			time.Sleep(15 * time.Second)
			continue
		}

		wh.loop()
	}
}

// write write bytes array to websocket with message type
func (wh *wsholder) write(mt int, data []byte) error {
	if wh.conn == nil {
		return fmt.Errorf("wsholder write failed, no ws connection")
	}

	// lock, ensure only one goroutine can write to
	// websocket in the same time
	wh.writeLock.Lock()
	err := wh.conn.WriteMessage(mt, data)
	wh.writeLock.Unlock()

	return err
}

// close close underlying websocket connection
func (wh *wsholder) close() {
	if wh.conn != nil {
		wh.conn.Close()
		wh.conn = nil
	}
}

// onPong update waiting response ping counter
func (wh *wsholder) onPong(data []byte) {
	wh.waitingPingCount = 0
}

// keepalive send ping message peer, and counter
func (wh *wsholder) keepalive() {
	if wh.conn == nil {
		return
	}

	// too many un-response ping, close the websocket connection
	if wh.waitingPingCount > 3 {
		if wh.conn != nil {
			log.Println("wsholder keepalive failed, close:", wh.uuid)
			wh.conn.Close()
		}
		return
	}

	// current time
	now := time.Now().Unix()
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(now))
	// send ping message
	wh.write(websocket.PingMessage, b)

	wh.waitingPingCount++
}

// loop read command websocket and process command
func (wh *wsholder) loop() {
	// save to map, for keep-alive
	wsholderMap[wh.uuid] = wh
	ws := wh.conn

	for {
		_, message, err := ws.ReadMessage()
		if err != nil {
			log.Println("wsholder handleRequest ws read error:", err)
			ws.Close()
			break
		}

		ops := message[0]
		switch ops {
		case 0:
			go onPairRequest(wh, message)
		default:
			log.Errorf("wsholder unsupport operation:%d", ops)
		}
	}
	// remove from map
	delete(wsholderMap, wh.uuid)
}

// onPairRequest connect to local port via tcp,
// and then connect to server via websocket, bridge the two connections.
func onPairRequest(_ *wsholder, message []byte) {
	// target port
	port := binary.LittleEndian.Uint16(message[1:3])
	// pair uuid
	uuid := message[3:]

	// only allow connect to local host
	address := fmt.Sprintf("127.0.0.1:%d", port)

	// connect to local network via tcp
	conn, err := net.Dial("tcp", address)
	if err != nil {
		log.Errorf("onPairRequest connect to address:%s failed:%v", address, err)
		return
	}

	// ensure the tcp connection will closed final
	defer conn.Close()

	// connect to server via websocket
	wsURLResp := fmt.Sprintf("%s?pt=resp&uuid=%s", wsURLBase, string(uuid))
	ws, _, err := websocket.DefaultDialer.Dial(wsURLResp, nil)
	if err != nil {
		log.Println("onPairRequest failed connect to websocket server:", err)
		return
	}

	// use pair's uuid as wsholder's identifier
	wh := newHolder(string(uuid), ws)
	// save to map, for keep-alive
	wsholderMap[wh.uuid] = wh

	// ensure the websocket will be closed final, and remove from map
	defer func() {
		wh.close()
		delete(wsholderMap, wh.uuid)
	}()

	// receive websocket message and forward to tcp
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

	// receive tcp message and forward to websocket
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

// writeAll a function that ensure all bytes write out
// maybe it is unnecessary, if the underlying tcp connection has ensure that
func writeAll(conn net.Conn, buf []byte) error {
	wrote := 0
	l := len(buf)
	for {
		n, err := conn.Write(buf[wrote:])
		if err != nil {
			return err
		}

		if n == 0 {
			// this should not happen
			break
		}

		wrote = wrote + n
		if wrote == l {
			break
		}
	}

	return nil
}
