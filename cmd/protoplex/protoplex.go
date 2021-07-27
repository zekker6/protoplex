package main

import (
	"fmt"
	"github.com/zekker6/protoplex/protoplex/multiplexer"
	"github.com/zekker6/protoplex/protoplex/protocols"
	"os"
	"sync"

	"github.com/rs/zerolog"
	"gopkg.in/alecthomas/kingpin.v2"
)

var version string

var (
	app    = kingpin.New("protoplex", "A fast and simple protocol multiplexer.")
	logger = zerolog.New(os.Stdout).With().Timestamp().Logger()

	versionFlag = app.Flag("version", "Prints the current program version").Short('V').Bool()

	bind    = app.Flag("bind", "The address to bind to").Short('b').Default("0.0.0.0:8443").String()
	verbose = app.Flag("verbose", "Enables debug logging").Short('v').Bool()
	pretty  = app.Flag("pretty", "Enables pretty logging").Short('p').Bool()

	ssh     = app.Flag("ssh", "The SSH server address").String()
	tls     = app.Flag("tls", "The TLS/HTTPS server address").String()
	openvpn = app.Flag("ovpn", "The OpenVPN server address").String()
	http    = app.Flag("http", "The HTTP server address").String()
	socks5  = app.Flag("socks5", "The SOCKS5 server address").String()
	socks4  = app.Flag("socks4", "The SOCKS4 server address").String()

	wireguard = app.Flag("wireguard", "The Wireguard server address").String()
	// stRelay := flag.String("strelay", "", "The Syncthing Relay server address")
)

func printVersion() {
	if version == "" {
		fmt.Println("Version has not been set.")
		os.Exit(1)
		return
	}
	fmt.Println(version)
	os.Exit(0)
}

type protocolConfig struct {
	flagValue   *string
	constructor func(string) *protocols.Protocol
}

func buildProtocolsConfig(conf []protocolConfig) []*protocols.Protocol {
	matchProtocols := make([]*protocols.Protocol, 0, len(conf))

	for _, config := range conf {
		v := *config.flagValue
		if v == "" {
			continue
		}

		matchProtocols = append(matchProtocols, config.constructor(v))
	}

	return matchProtocols
}

func runTcpServer() {
	protocolsConfig := buildProtocolsConfig([]protocolConfig{
		// contain-bytes-matched protocols (usually ALPNs) take priority
		// (due to start-bytes-matching overriding some of them)
		// {
		// 	stRelay, protocols.NewSTRelayProtocol,
		// },
		// start-bytes-matched protocols are the next most efficient approach
		{
			tls, protocols.NewTLSProtocol,
		},
		{
			ssh, protocols.NewSSHProtocol,
		},
		{
			socks5, protocols.NewSOCKS5Protocol,
		},
		{
			socks4, protocols.NewTLSProtocol,
		},

		{
			socks4, protocols.NewSOCKS4Protocol,
		},
		// regex protocols come at the end of the chain as they'll be expensive anyway if used
		{
			openvpn, protocols.NewOpenVPNProtocol,
		},
		{
			http, protocols.NewHTTPProtocol,
		},
	})

	multiplexer.NewTCPServer(protocolsConfig, logger).Run(*bind)
}

func runUdpServer() {
	protocolsConfig := buildProtocolsConfig([]protocolConfig{
		{
			wireguard, protocols.NewWireguardProtocol,
		},
	})

	multiplexer.NewUDPServer(protocolsConfig, logger).Run(*bind)
}

func main() {
	_, _ = app.Parse(os.Args[1:])

	if *versionFlag {
		printVersion()
	}

	if *pretty {
		logger = logger.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	if *verbose {
		logger = logger.Level(zerolog.DebugLevel)
	} else {
		logger = logger.Level(zerolog.InfoLevel)
	}

	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		runTcpServer()
		wg.Done()
	}()

	go func() {
		runUdpServer()
		wg.Done()
	}()

	wg.Wait()
}
