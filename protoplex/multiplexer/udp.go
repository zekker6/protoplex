package multiplexer

import (
	"github.com/Pandentia/protoplex/protoplex/protocols"
	"github.com/rs/zerolog"
	"net"
	"os"
	"strconv"
	"strings"
)

type UDPServer struct {
	protocols []*protocols.Protocol
	logger    zerolog.Logger

	state *TTLMap
}

type connState struct {
	Protocol *protocols.Protocol
	Read     []byte
	Conn     *net.UDPConn
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

	if err != nil {
		s.logger.Fatal().Str("bind", bind).Err(err).Msg("Unable to create listener.")
		os.Exit(1)
	}

	defer listener.Close()
	s.logger.Info().Str("protocol", "udp").Str("bind", addr.String()).Msg("Listening...")
	for {
		buffer := make([]byte, defaultBufSize)
		_, addr, err := listener.ReadFromUDP(buffer)
		if err != nil {
			s.logger.Debug().Err(err).Msg("Error while accepting connection.")
		}

		s.handle(buffer, addr)
	}
}

func (s *UDPServer) handle(buffer []byte, addr *net.UDPAddr) {
	key := addr.String()
	localLogger := s.logger.With().Str("addr", key).Logger()

	if s.state.Has(key) {
		localLogger.Info().Msg("using cached connection")
		state := s.state.Get(key)

		_, err := state.Conn.Write(buffer)
		if err != nil {
			localLogger.Err(err).Msg("failed to send data to target")
		}
		return
	}
	localLogger.Info().Msg("building new connection ")

	protocol := DetermineProtocol(buffer, s.protocols)
	if protocol == nil {
		localLogger.Warn().Msg("failed to detect protocol")
		return
	}
	remoteAddr := s.parseIP(protocol.Target)

	conn, err := net.DialUDP("udp", nil, remoteAddr)
	if err != nil {
		localLogger.Err(err).Str("target", protocol.Target).Msg("failed to connect to target")
		return
	}

	state := connState{
		Protocol: protocol,
		Conn:     conn,
	}

	s.state.Put(key, state)

	s.handle(buffer, addr)
	return
}
