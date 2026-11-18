<div align="center">

# Tenet

Tenet reverses the direction of TCP-style connections. One program connects to
another over a secure WebSocket, and that program then opens connections back
through the WebSocket, as if it could connect directly.

[![Documentation](https://img.shields.io/badge/go.dev-documentation-007d9c?&style=for-the-badge)](https://pkg.go.dev/github.com/dogmatiq/tenet)
[![Latest Version](https://img.shields.io/github/tag/dogmatiq/tenet.svg?&style=for-the-badge&label=semver)](https://github.com/dogmatiq/tenet/releases)
[![Build Status](https://img.shields.io/github/actions/workflow/status/dogmatiq/tenet/ci.yml?style=for-the-badge&branch=main)](https://github.com/dogmatiq/tenet/actions/workflows/ci.yml)
[![Code Coverage](https://img.shields.io/codecov/c/github/dogmatiq/tenet/main.svg?style=for-the-badge)](https://codecov.io/github/dogmatiq/tenet)

</div>

Tenet is for a program that can only make outbound connections, such as one in a
network that blocks everything inbound, but must still accept connections from
outside the network.

## Concepts

- The **client** is the program that connects out. It connects to the server
  over a WebSocket, and then accepts the connections that the server opens.
- The **server** is the program that the client connects to. It opens
  connections back to the client through the WebSocket.
- A **tunnel** is the WebSocket between a client and a server. Connections pass
  through it.
- A **connection** is a reliable, ordered stream of bytes, like a TCP
  connection, through a tunnel. The server opens it, and the client accepts it.
  Many connections share one tunnel.

## How it works

1. The client calls `tenet.Connect`, which returns a `Client` and, in the
   background, sends an HTTP request to a URL that the server serves.
2. The server's HTTP handler passes the request to `tenet.Accept`, which
   upgrades it to a WebSocket and returns a `Dialer`. The WebSocket starts the
   tunnel.
3. The client calls `Client.Listen` for each network and address that it accepts
   connections for. Each call returns a `Listener`, which is a `net.Listener`.
4. The server calls `Dialer.DialContext` with a network and an address. It
   returns the server's end of a new connection, and `Listener.Accept` returns
   the client's end, from the `Listener` for that network and address.
5. When the WebSocket ends, every connection through it ends, and the `Dialer`
   stops working. The `Client` connects again, so the tunnel starts again from
   step 2, with a new `Dialer`.

## Bounds

At most 256 connections wait for `Listener.Accept` to accept them. A connection
beyond that is reset, in the same way as TCP resets a connection beyond its
backlog. Each connection buffers at most 256 KB in each direction.

See the [documentation](https://pkg.go.dev/github.com/dogmatiq/tenet) for the
API and examples.
