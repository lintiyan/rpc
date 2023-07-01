package transport

import (
	"io"
	"net"
)

const (
	TCPTransportType = iota
)

type TransportType byte

var transportMap = map[TransportType]ServerTransport{
	TCPTransportType: &ServerSocket{},
}

func NewServerTransport(transportType TransportType) ServerTransport {
	return transportMap[transportType]
}

type Transport interface {
	Dial(network, addr string) error
	RemoteAddr() net.Addr
	LocalAddr() net.Addr
	io.ReadWriteCloser
}

type Socket struct {
	conn net.Conn
}

func (s *Socket) Dial(network, addr string) error {
	conn, err := net.Dial(network, addr)
	if err != nil {
		return err
	}
	s.conn = conn
	return nil
}

func (s *Socket) Read(p []byte) (n int, err error) {
	return s.conn.Read(p)
}

func (s *Socket) Write(p []byte) (n int, err error) {
	return s.conn.Write(p)
}

func (s *Socket) Close() error {
	return s.conn.Close()
}
func (s *Socket) RemoteAddr() net.Addr {
	return s.conn.RemoteAddr()
}

func (s *Socket) LocalAddr() net.Addr {
	return s.conn.LocalAddr()
}

type ServerTransport interface {
	Listen(network, addr string) error
	Accept() (Transport, error)
	io.Closer
}

type ServerSocket struct {
	ln net.Listener
}

func (s *ServerSocket) Listen(network, addr string) error {

	listener, err := net.Listen(network, addr)
	if err != nil {
		return err
	}
	s.ln = listener
	return nil
}

func (s *ServerSocket) Accept() (Transport, error) {
	conn, err := s.ln.Accept()
	if err != nil {
		return &Socket{}, err
	}
	return &Socket{conn: conn}, nil
}

func (s *ServerSocket) Close() error {
	return s.ln.Close()
}
