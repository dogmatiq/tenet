// Package tenet reverses the direction of TCP-style connections.
//
// One program connects to another over a secure WebSocket. That program then
// opens connections back through the WebSocket, as if it could connect
// directly.
package tenet
