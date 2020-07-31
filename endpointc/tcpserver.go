package endpointc

import (
	"fmt"
	"net"
	"sync"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"

	"github.com/golang/snappy"
)

type wsholder struct {
	conn      *websocket.Conn
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

func (wh *wsholder) write(mt int, data []byte) error {
	if wh.conn == nil {
		return fmt.Errorf("wsholder write failed, no ws")
	}

	wh.writeLock.Lock()
	err := wh.conn.WriteMessage(mt, data)
	wh.writeLock.Unlock()
	return err
}

func (wh *wsholder) close() {
	if wh.conn != nil {
		wh.conn.Close()
		wh.conn = nil
	}
}

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

func handleRequest(conn *net.TCPConn) {
	defer conn.Close()

	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		log.Println("handleRequest failed connect to websocket server:", err)
		return
	}
	wh := newHolder(ws)
	defer wh.close()

	decoded := make([]byte, 8192)
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

func writeAll(conn *net.TCPConn, buf []byte) error {
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
