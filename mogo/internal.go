package mogo

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"net"
	"time"
)

const (
	incomingQueueSize = 100

	// Gost used a number of fixed-size buffers for incoming messages to limit allocations. This is controlled
	// by udpBufSize and nUDPBufs. Note that gost cannot accept statsd messages larger than udpBufSize.
	// In this case, the total size of buffers for incoming messages is 10e3 * 1000 = 10MB.
	udpBufSize = 10e3
	nUDPBufs   = 1000

	// All TCP connections managed by gost have this keepalive duration applied
	tcpKeepAlivePeriod = 30 * time.Second
)

var (
	configFile       = flag.String("conf", "conf.toml", "TOML configuration file")
	forwardKeyPrefix = []byte("f|")
)

type Server struct {
	conf *Conf
	quit chan struct{} // For shutting down everything

	bufPool chan []byte // pool of buffers for incoming messages

	metaStats chan *Stat

	incoming chan *Stat  // incoming stats are passed to the aggregator
	outgoing chan []byte // outgoing Graphite messages
}

// NewServer sets up a new server with some configuration without starting goroutines or listeners. out is
// where logs are written.
func NewServer(conf *Conf, out io.Writer) *Server {
	// TODO: May want to make this configurable later.
	s := &Server{
		conf:      conf,
		quit:      make(chan struct{}),
		bufPool:   make(chan []byte, nUDPBufs),
		metaStats: make(chan *Stat),
		incoming:  make(chan *Stat, incomingQueueSize),
		outgoing:  make(chan []byte),
	}
	// Preallocate the UDP buffer pool
	for i := 0; i < nUDPBufs; i++ {
		s.bufPool <- make([]byte, udpBufSize)
	}

	return s
}

// Listen launches the various server goroutines and starts the various listeners.
// If the listener params are nil, these are constructed from the parameters in the conf. Otherwise they are
// used as-is. This makes it possible for the tests to construct listeners on an available port and pass them
// in.
func (s *Server) Listen(clientConn *net.UDPConn, forwardListener net.Listener, debugListener *net.TCPListener) error {

	errorCh := make(chan error)
	if s.conf.forwarderEnabled {
		if forwardListener == nil {
			l, err := net.Listen("tcp", s.conf.ForwarderListenAddr)
			if err != nil {
				return err
			}
			forwardListener = tcpKeepAliveListener{l.(*net.TCPListener)}
		}
		fmt.Println("Listening for forwarded gost messages on", forwardListener.Addr())
	}

	if err := s.Start(s.conf.DebugPort, debugListener); err != nil {
		return err
	}

	if clientConn == nil {
		ip, err := GetLocalIp()
		if err != nil {
			return err
		}
		udpAddr := fmt.Sprintf("%s:%d", ip, s.conf.Port)
		udp, err := net.ResolveUDPAddr("udp", udpAddr)
		if err != nil {
			return err
		}
		clientConn, err = net.ListenUDP("udp", udp)
		if err != nil {
			return err
		}
	}
	fmt.Println("Listening for UDP client requests on", clientConn.LocalAddr())
	go func() {
		errorCh <- s.clientServer(clientConn)
	}()

	return <-errorCh
}

type StatType int

const (
	StatCounter StatType = iota
	StatGauge
	StatTimer
	StatSet
)

type Stat struct {
	Type       StatType
	Forward    bool
	Name       string
	Value      float64
	SampleRate float64 // Only for counters
}

// tagToStatType maps a tag (e.g., []byte("c")) to a StatType (e.g., StatCounter).
func tagToStatType(b []byte) (StatType, bool) {
	switch len(b) {
	case 1:
		switch b[0] {
		case 'c':
			return StatCounter, true
		case 'g':
			return StatGauge, true
		case 's':
			return StatSet, true
		}
	case 2:
		if b[0] == 'm' && b[1] == 's' {
			return StatTimer, true
		}
	}
	return 0, false
}

func (s *Server) handleMessages(buf []byte) {
	for _, msg := range bytes.Split(buf, []byte{'\n'}) {
		s.handleMessage(msg)
	}
	s.bufPool <- buf[:cap(buf)] // Reset buf's length and return to the pool
}

func (s *Server) handleMessage(msg []byte) {
	if len(msg) == 0 {
		return
	}
	stat, ok := parseStatsdMessage(msg, s.conf.forwardingEnabled)
	if !ok {
		fmt.Println("bad message:", string(msg))
		return
	}
	if stat.Forward {
		if stat.Type != StatCounter {
			return
		}
	} else {
		s.incoming <- stat
	}
}

func (s *Server) clientServer(c *net.UDPConn) error {
	for {
		buf := <-s.bufPool
		n, _, err := c.ReadFromUDP(buf)
		if err != nil {
			return err
		}
		if n >= udpBufSize {
			continue
		}
		go s.handleMessages(buf[:n])
	}
}

func (s *Server) tcpClientServer(c *net.TCPConn) error {
	r := bufio.NewReader(c)
	for {
		msgs, err := r.ReadString('\n')
		if err != nil {
			return err
		}

		go func() {
			for _, msg := range bytes.Split([]byte(msgs), []byte{'\n'}) {
				s.handleMessage(msg)
			}
		}()
	}
}

// If listener is non-nil, then it's used; otherwise listen on TCP using the given port.
func (s *Server) Start(port int, listener *net.TCPListener) error {
	if listener == nil {
		var err error
		ip, err := GetLocalIp()
		if err != nil {
			return err
		}
		addr := fmt.Sprintf("%s:%d", ip, port)
		tcpAddr, err := net.ResolveTCPAddr("tcp4", addr)
		if err != nil {
			return err
		}
		fmt.Println("Listening for debug TCP clients on", addr)
		listener, err = net.ListenTCP("tcp", tcpAddr)
		if err != nil {
			return err
		}
	}
	go func() {
		for {
			c, err := listener.AcceptTCP()
			if err != nil {
				continue
			}
			c.SetWriteDeadline(time.Now().Add(10 * time.Millisecond))
			c.SetKeepAlive(true)
			c.SetKeepAlivePeriod(tcpKeepAlivePeriod)
			c.SetNoDelay(true)
			go s.tcpClientServer(c)
		}
	}()
	return nil
}

func (s *Server) closeClient(client net.Conn) {
	client.Close()
	fmt.Println("Tcp client:%s disconnected.", client.RemoteAddr())
}

type tcpKeepAliveListener struct {
	*net.TCPListener
}

func (l tcpKeepAliveListener) Accept() (net.Conn, error) {
	c, err := l.AcceptTCP()
	if err != nil {
		return nil, err
	}
	if err := c.SetKeepAlive(true); err != nil {
		return nil, err
	}
	if err := c.SetKeepAlivePeriod(tcpKeepAlivePeriod); err != nil {
		return nil, err
	}
	return c, nil
}
