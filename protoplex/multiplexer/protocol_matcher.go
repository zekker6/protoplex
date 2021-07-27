package multiplexer

import (
	"bytes"
	"github.com/Pandentia/protoplex/protoplex/protocols"
)

// DetermineProtocol determines a Protocol based on a given handshake
func DetermineProtocol(data []byte, p []*protocols.Protocol) *protocols.Protocol {
	dataLength := len(data)
	for _, protocol := range p {
		// since every protocol is different, let's limit the way we match things
		if (protocol.NoComparisonBeforeBytes != 0 && dataLength < protocol.NoComparisonBeforeBytes) ||
			(protocol.NoComparisonAfterBytes != 0 && dataLength > protocol.NoComparisonAfterBytes) {
			continue // avoids unnecessary comparisons
		}

		// compare against bytestrings first for efficiency
		// first "contains" (due to ALPNs we can't match against TLS start bytes first)
		for _, byteSlice := range protocol.MatchBytes {
			byteSliceLength := len(byteSlice)
			if dataLength < byteSliceLength {
				continue
			}
			if bytes.Contains(data, byteSlice) {
				return protocol
			}
		}
		// then against prefixes
		for _, byteSlice := range protocol.MatchStartBytes {
			byteSliceLength := len(byteSlice)
			if dataLength < byteSliceLength {
				continue
			}
			if bytes.Equal(byteSlice, data[:byteSliceLength]) {
				return protocol
			}
		}

		// let's use regex matching as a last resort
		for _, regex := range protocol.MatchRegexes {
			if regex.Match(data) {
				return protocol
			}
		}
	}
	return nil
}
