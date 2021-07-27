package multiplexer

import (
	"github.com/rs/zerolog"
	"github.com/zekker6/protoplex/protoplex/protocols"
	"net"
	"os"
	"time"
)

type TCPServer struct {
	protocols []*protocols.Protocol
	logger    zerolog.Logger
}

func NewTCPServer(p []*protocols.Protocol, logger zerolog.Logger) *TCPServer {
	logger = logger.With().Str("module", "listener").Str("type", "tcp").Logger()
	if len(p) == 0 {
		logger.Warn().Msg("No protocols defined.")
	} else {
		logger.Info().Msg("Protocol chain:")
		for _, proto := range p {
			logger.Info().Str("protocol", proto.Name).Str("target", proto.Target).Msgf("- %s @ %s", proto.Name, proto.Target)
		}
	}

	return &TCPServer{
		protocols: p,
		logger:    logger,
	}
}

func (s *TCPServer) Run(bind string) {
	listener, err := net.Listen("tcp", bind)
	if err != nil {
		s.logger.Fatal().Str("bind", bind).Err(err).Msg("Unable to create listener.")
		os.Exit(1)
	}
	defer listener.Close()
	s.logger.Info().Str("bind", listener.Addr().String()).Str("protocol", "tcp").Msg("Listening...")
	for {
		conn, err := listener.Accept()
		if err != nil {
			s.logger.Debug().Err(err).Msg("Error while accepting connection.")
		}
		go s.handle(conn)
	}
}

// handle connects a net.Conn with a proxy target given a list of protocols
func (s *TCPServer) handle(conn net.Conn) {
	defer conn.Close() // the connection must close after this goroutine exits

	localLogger := s.logger.With().Str("module", "handler").Str("ip", conn.RemoteAddr().String()).Logger()

	identifyBuffer := make([]byte, defaultBufSize) // at max 1KB buffer to identify payload

	// read the handshake into our buffer
	_ = conn.SetReadDeadline(time.Now().Add(15 * time.Second)) // 15-second timeout to identify
	n, err := conn.Read(identifyBuffer)
	if err != nil {
		localLogger.Debug().Err(err).Msg("Identify read error. Connection closed.")
		return
	}
	_ = conn.SetReadDeadline(time.Time{}) // reset our timeout

	// determine the protocol
	protocol := DetermineProtocol(identifyBuffer[:n], s.protocols)
	if protocol == nil { // unsuccessful protocol identify, close and forget
		localLogger.Debug().Msg("Protocol unrecognized. Connection closed.")
		return
	}

	localLogger = localLogger.With().Str("protocol", protocol.Name).Str("target", protocol.Target).Logger()
	localLogger.Debug().Msg("Protocol recognized.")

	// establish our connection with the target
	targetConn, err := net.Dial("tcp", protocol.Target)
	if err != nil {
		localLogger.Debug().Err(err).Msg("Remote connection unsuccessful.")
		return // we were unable to establish the connection with the proxy target
	}
	defer targetConn.Close()
	_, err = targetConn.Write(identifyBuffer[:n]) // tell them everything they just told us
	if err != nil {
		localLogger.Debug().Err(err).Msg("Remote disconnected us during identify.")
		return // remote rejected us?? okay.
	}

	// run the proxy readers
	closed := make(chan bool, 2)
	go s.proxy(conn, targetConn, closed)
	go s.proxy(targetConn, conn, closed)

	// wait for any connection to close
	<-closed
	localLogger.Debug().Msg("Connection closed.")
}

func (TCPServer) proxy(from net.Conn, to net.Conn, closed chan bool) {
	data := make([]byte, 4096) // 4KiB buffer

	for {
		n, err := from.Read(data)
		if err != nil {
			closed <- true
			return
		}
		_, err = to.Write(data[:n])
		if err != nil {
			closed <- true
			return
		}
	}
}
