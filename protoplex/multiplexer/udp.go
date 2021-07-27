package multiplexer

import (
	"github.com/rs/zerolog"
	"github.com/zekker6/protoplex/protoplex/protocols"
	"net"
	"os"
	"strconv"
	"strings"
)

type UDPServer struct {
	protocols []*protocols.Protocol
	logger    zerolog.Logger

	state    *TTLMap
	listener *net.UDPConn
}

type ConnState struct {
	Protocol        *protocols.Protocol
	Read            []byte
	ProxyConnection *net.UDPConn
}

func NewUDPServer(p []*protocols.Protocol, logger zerolog.Logger) *UDPServer {
	logger = logger.With().Str("module", "listener").Str("type", "udp").Logger()
	if len(p) == 0 {
		logger.Warn().Msg("No protocols defined.")
	} else {
		logger.Info().Msg("Protocol chain:")
		for _, proto := range p {
			logger.Info().Str("protocol", proto.Name).Str("target", proto.Target).Msgf("- %s @ %s", proto.Name, proto.Target)
		}
	}

	return &UDPServer{
		protocols: p,
		logger:    logger,
		state:     NewTTlMap(100, 60*20),
	}
}

func (s *UDPServer) parseIP(ip string) *net.UDPAddr {
	parts := strings.Split(ip, ":")
	if len(parts) != 2 {
		s.logger.Error().Str("bind", ip).Msg("Cannot parse address for bind")
		return nil
	}

	port, err := strconv.Atoi(parts[1])
	if err != nil {
		s.logger.Error().Str("bind", ip).Err(err).Msg("Failed to parse bind port")
		return nil
	}

	addr := &net.UDPAddr{
		Port: port,
		IP:   net.ParseIP(parts[0]),
	}

	return addr
}

func (s *UDPServer) Run(bind string) {
	addr := s.parseIP(bind)

	listener, err := net.ListenUDP("udp", addr)
	s.listener = listener

	if err != nil {
		s.logger.Fatal().Str("bind", bind).Err(err).Msg("Unable to create listener.")
		os.Exit(1)
	}

	defer listener.Close()
	s.logger.Info().Str("protocol", "udp").Str("bind", addr.String()).Msg("Listening...")
	for {
		buffer := make([]byte, defaultBufSize)
		readBytes, addr, err := listener.ReadFromUDP(buffer)
		if err != nil {
			s.logger.Debug().Err(err).Msg("Error while accepting connection.")
		}

		s.handle(buffer, readBytes, addr)
	}
}

func (s *UDPServer) handle(buffer []byte, readBytes int, addr *net.UDPAddr) {
	buffer = buffer[:readBytes]

	key := addr.String()
	localLogger := s.logger.With().Str("addr", key).Logger()

	if s.state.Has(key) {
		localLogger.Debug().Msg("using cached connection")
		state := s.state.Get(key)

		_, err := state.ProxyConnection.Write(buffer)
		if err != nil {
			localLogger.Err(err).Msg("failed to send data to target")
		}
		return
	}
	localLogger.Debug().Msg("building new connection ")

	protocol := DetermineProtocol(buffer, s.protocols)
	if protocol == nil {
		localLogger.Warn().Msg("failed to detect protocol")
		return
	}
	remoteAddr := s.parseIP(protocol.Target)

	proxyConnection, err := net.DialUDP("udp", nil, remoteAddr)

	if err != nil {
		localLogger.Err(err).Str("target", protocol.Target).Msg("failed to connect to target")
		return
	}

	state := ConnState{
		Protocol:        protocol,
		ProxyConnection: proxyConnection,
	}

	s.state.Put(key, state)

	go s.proxy(proxyConnection, addr, localLogger.With().Str("direction", "s->c").Logger())

	s.handle(buffer, readBytes, addr)
	return
}

func (s *UDPServer) proxy(src *net.UDPConn, dstAddr *net.UDPAddr, logger zerolog.Logger) {
	logger = logger.With().Str("module", "udp-proxy").Logger()

	for {
		buf := make([]byte, defaultBufSize)

		read, _, err := src.ReadFromUDP(buf)
		logger.Debug().Str("addr", dstAddr.String()).Msg("sending data")
		if err != nil {
			logger.Err(err).Msg("failed to read data from client")
			return
		}

		wrote, err := s.listener.WriteTo(buf[:read], dstAddr)
		if err != nil {
			logger.Err(err).Msg("failed to read data from client")
			return
		}

		if read != wrote {
			logger.Warn().Int("wrote", wrote).Int("read", read).Msg("mismatched written and read data")
		}
	}

}
