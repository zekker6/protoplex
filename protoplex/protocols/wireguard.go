package protocols

// NewWireguardProtocol initializes a Protocol with a Wireguard signature.
func NewWireguardProtocol(targetAddress string) *Protocol {
	return &Protocol{
		Name:            "Wireguard",
		Target:          targetAddress,
		MatchStartBytes: [][]byte{{0x01, 0x00, 0x00, 0x00}},
	}
}
