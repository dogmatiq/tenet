package tenet

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"syscall"
)

// addr is a network and an address that a [Listener] listens on, and that
// [Dialer.DialContext] dials. It is a [net.Addr].
type addr struct {
	network string
	address string
}

const (
	// maxNetworkLen is the length of the longest network, in bytes.
	maxNetworkLen = 64

	// maxAddressLen is the length of the longest address, in bytes.
	maxAddressLen = 448
)

func (a addr) Network() string {
	return a.network
}

func (a addr) String() string {
	return a.address
}

// unnamed returns an address with the same network as a, and an empty
// string as its address. It is the address of a connection's end that is not
// the [Listener]'s end, as the unnamed end of a Unix socket connection is.
func (a addr) unnamed() addr {
	return addr{network: a.network}
}

// validate returns an error that wraps [syscall.EINVAL] if the network or the
// address is too long.
func (a addr) validate() error {
	if len(a.network) > maxNetworkLen {
		return fmt.Errorf("the network is longer than %d bytes: %w", maxNetworkLen, syscall.EINVAL)
	}
	if len(a.address) > maxAddressLen {
		return fmt.Errorf("the address is longer than %d bytes: %w", maxAddressLen, syscall.EINVAL)
	}
	return nil
}

// exchange writes a to s, a stream that the dialer has just opened, and reads
// the client's answer. It is the dialer's side of the exchange that starts
// each connection.
//
// Each connection is one yamux stream, which [Dialer.DialContext] opens. Only
// the dialer opens streams, and the server ends the tunnel if the client opens
// one. The dialer writes the [addr] that it dials first: the network and then the
// address, each as its length in bytes, as a 2-byte big-endian unsigned
// integer, followed by its bytes. The client answers with one byte, an
// [answer]. If the answer is [accepted], every later byte in each direction
// belongs to the connection. Otherwise, the client closes the stream.
//
// The client closes the stream without an answer if the network or the address
// is longer than [maxNetworkLen] or [maxAddressLen].
//
// The version of this format is the version in the WebSocket subprotocol.
func exchange(s io.ReadWriter, a addr) (answer, error) {
	if err := writeAddr(s, a); err != nil {
		return 0, err
	}
	return readAnswer(s)
}

// writeAddr writes a to w, as the dialer does when it opens a stream.
func writeAddr(w io.Writer, a addr) error {
	buf := make([]byte, 0, 2+len(a.network)+2+len(a.address))
	buf = binary.BigEndian.AppendUint16(buf, uint16(len(a.network)))
	buf = append(buf, a.network...)
	buf = binary.BigEndian.AppendUint16(buf, uint16(len(a.address)))
	buf = append(buf, a.address...)

	_, err := w.Write(buf)
	return err
}

// readAddr reads the [addr] that the dialer writes when it opens a stream.
func readAddr(r io.Reader) (addr, error) {
	network, err := readString(r, maxNetworkLen)
	if err != nil {
		return addr{}, err
	}

	address, err := readString(r, maxAddressLen)
	if err != nil {
		return addr{}, err
	}

	return addr{network, address}, nil
}

// readString reads a string that is written as its length, as a 2-byte
// big-endian unsigned integer, followed by its bytes.
func readString(r io.Reader, maxLen int) (string, error) {
	var n [2]byte
	if _, err := io.ReadFull(r, n[:]); err != nil {
		return "", err
	}

	size := int(binary.BigEndian.Uint16(n[:]))
	if size > maxLen {
		return "", errors.New("the string is too long")
	}

	buf := make([]byte, size)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}

	return string(buf), nil
}

// answer is the byte that the client writes to answer each stream that the
// dialer opens.
type answer byte

const (
	// accepted means that the stream is queued for the [Listener] for its
	// [addr].
	accepted answer = 0

	// refused means that no [Listener] has the stream's [addr].
	refused answer = 1

	// backlogFull means that the [Listener] for the stream's [addr] already
	// has as many streams waiting to be accepted as it can hold.
	backlogFull answer = 2
)

// writeAnswer writes ans to w.
func writeAnswer(w io.Writer, ans answer) error {
	_, err := w.Write([]byte{byte(ans)})
	return err
}

// readAnswer reads an [answer] from r.
func readAnswer(r io.Reader) (answer, error) {
	var buf [1]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return 0, err
	}

	ans := answer(buf[0])
	switch ans {
	case accepted, refused, backlogFull:
		return ans, nil
	default:
		return 0, fmt.Errorf("the client gave an unknown answer, %d", ans)
	}
}
