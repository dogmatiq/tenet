// Package tenet lets a server that can only make outbound connections accept
// TCP-style connections from a client, by connecting to the client over a
// secure WebSocket.
//
// Some servers run in networks that allow only outbound connections, so a
// client outside the network can't reach them. tenet reverses the direction.
// The server connects to a WebSocket server that the client runs, and the
// client then dials its connections to the server through that WebSocket.
//
// Each connection is a reliable, ordered stream of bytes, like a TCP
// connection. Many connections share one WebSocket. A tunnel is one WebSocket,
// and the connections that it carries.
//
// # Use
//
// The server calls [Listen], which connects to the WebSocket server at a URL,
// and returns a [Listener]. A [Listener] is a [net.Listener], so the server can
// serve it in the same way as any other [net.Listener]:
//
//	l, err := tenet.Listen(ctx, httpClient, "wss://example.com/tunnel", header)
//	if err != nil {
//		return err
//	}
//	return server.Serve(l)
//
// The WebSocket server's [http.Handler] calls [Accept], which returns a
// [Dialer]. The [Dialer] dials connections through the WebSocket to the
// [Listener] at its other end:
//
//	d, err := tenet.Accept(w, r)
//	if err != nil {
//		return
//	}
//	conn, err := d.Dial(ctx)
//
// # Bounds
//
// At most 256 connections wait for [Listener.Accept] to accept them. A
// connection beyond that is reset, in the same way as TCP resets a connection
// beyond its backlog. Each connection buffers at most 256 KB in each direction.
package tenet
