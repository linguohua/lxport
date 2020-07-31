package endpointc

import (
	"fmt"
	"net"
	"sync"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"

	"github.com/golang/snappy"
)

// wsholder websocket holder
type wsholder struct {
	conn *websocket.Conn
	// protect websocket conn cocurrently writing
	writeLock sync.Mutex
}

func newHolder(c *websocket.Conn) *wsholder {
	wh := &wsholder{
		conn: c,
	}

	// ping/pong handlers
	c.SetPingHandler(func(data string) error {
		wh.write(websocket.PongMessage, []byte(data))
		return nil
	})

	c.SetPongHandler(func(data string) error {
		// d.onPong([]byte(data))
		return nil
	})

	return wh
}

// write write bytes array to websocket with message type
func (wh *wsholder) write(mt int, data []byte) error {
	if wh.conn == nil {
		return fmt.Errorf("wsholder write failed, no ws")
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

// startTCPListener start tcp server, listen on localhost
func startTCPListener(port uint16) {
	address := fmt.Sprintf("127.0.0.1:%d", port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatal("startTCPListener tcp server listen failed:", err)
	}

	for {
		// Listen for an incoming connection.
		conn, err := listener.Accept()
		if err != nil {
			log.Println("startTCPListener error accepting: ", err.Error())
			continue
		}

		// Handle connections in a new goroutine.
		go handleRequest(conn.(*net.TCPConn))
	}
}

// handleRequest read tcp connection, and send to server via websocket connection
func handleRequest(conn *net.TCPConn) {
	defer conn.Close()

	// build websocket connection
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		log.Println("handleRequest failed connect to websocket server:", err)
		return
	}
	wh := newHolder(ws)
	// ensure websocket connection will be closed final
	defer wh.close()

	decoded := make([]byte, 8192)
	// read websocket message and forward to tcp
	go func() {
		for {
			_, message, err := ws.ReadMessage()
			if err != nil {
				log.Println("handleRequest ws read error:", err)
				conn.Close()
				break
			}

			// decode snappy
			sout, _ := snappy.Decode(decoded, message)
			// log.Println("handleRequest, ws message len:", len(message))
			err = writeAll(conn, sout)
			if err != nil {
				break
			}
		}
	}()

	// read tcp message and forward to websocket
	tcpbuf := make([]byte, 8192)
	for {
		n, err := conn.Read(tcpbuf)
		if err != nil {
			log.Println("handleRequest tcp read error:", err)
			ws.Close()
			break
		}

		err = wh.write(websocket.BinaryMessage, tcpbuf[:n])
		if err != nil {
			log.Println("handleRequest ws write error:", err)
			break
		}
	}
}

// writeAll a function that ensure all bytes write out
// maybe it is unnecessary, if the underlying tcp connection has ensure that
func writeAll(conn *net.TCPConn, buf []byte) error {
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
