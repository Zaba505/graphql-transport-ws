package gws

import (
	"context"
	"net/http"

	"nhooyr.io/websocket"
)

// Conn is a client connection that should be closed by the client.
type Conn struct {
	wc *websocket.Conn

	done chan struct{}
}

func newConn(wc *websocket.Conn) *Conn {
	c := &Conn{
		wc:   wc,
		done: make(chan struct{}, 1),
	}

	return c
}

func (c *Conn) read(ctx context.Context) ([]byte, error) {
	_, b, err := c.wc.Read(ctx)
	return b, err
}

func (c *Conn) write(ctx context.Context, msg operationMessage) error {
	b, err := msg.MarshalJSON()
	if err != nil {
		return err
	}

	return c.wc.Write(ctx, websocket.MessageBinary, b)
}

// Close closes the underlying WebSocket connection.
func (c *Conn) Close() error {
	close(c.done)

	err := c.write(context.Background(), operationMessage{Type: gqlConnectionTerminate})
	if err != nil {
		return err
	}

	return c.wc.Close(websocket.StatusNormalClosure, "closed")
}

type dialOpts struct {
	client      *http.Client
	headers     http.Header
	compression CompressionMode
	threshold   int
}

// DialOption configures how we set up the connection.
type DialOption interface {
	SetDial(*dialOpts)
}

type optionFn func(*dialOpts)

func (f optionFn) SetDial(opts *dialOpts) { f(opts) }

// CompressionMode represents the modes available to the deflate extension. See
// https://tools.ietf.org/html/rfc7692
//
// A compatibility layer is implemented for the older deflate-frame extension
// used by safari. See
// https://tools.ietf.org/html/draft-tyoshino-hybi-websocket-perframe-deflate-06
// It will work the same in every way except that we cannot signal to the peer
// we want to use no context takeover on our side, we can only signal that they
// should. It is however currently disabled due to Safari bugs. See
// https://github.com/nhooyr/websocket/issues/218
//
type CompressionMode websocket.CompressionMode

const (
	// CompressionNoContextTakeover grabs a new flate.Reader and flate.Writer as needed
	// for every message. This applies to both server and client side.
	//
	// This means less efficient compression as the sliding window from previous messages
	// will not be used but the memory overhead will be lower if the connections
	// are long lived and seldom used.
	//
	// The message will only be compressed if greater than 512 bytes.
	//
	CompressionNoContextTakeover CompressionMode = iota

	// CompressionContextTakeover uses a flate.Reader and flate.Writer per connection.
	// This enables reusing the sliding window from previous messages.
	// As most WebSocket protocols are repetitive, this can be very efficient.
	// It carries an overhead of 8 kB for every connection compared to CompressionNoContextTakeover.
	//
	// If the peer negotiates NoContextTakeover on the client or server side, it will be
	// used instead as this is required by the RFC.
	//
	CompressionContextTakeover

	// CompressionDisabled disables the deflate extension.
	//
	// Use this if you are using a predominantly binary protocol with very
	// little duplication in between messages or CPU and memory are more
	// important than bandwidth.
	//
	CompressionDisabled
)

// ConnOption represents a configuration that applies symmetrically
// on both sides, client and server.
//
type ConnOption interface {
	DialOption
	ServerOption
}

type compression struct {
	mode      CompressionMode
	threshold int
}

func (opt compression) SetDial(opts *dialOpts) {
	opts.compression = opt.mode
	opts.threshold = opt.threshold
}

func (opt compression) SetServer(opts *options) {
	opts.mode = opt.mode
	opts.threshold = opt.threshold
}

// WithCompression configures compression over the WebSocket.
// By default, compression is disabled and for now is considered
// an experimental feature.
//
func WithCompression(mode CompressionMode, threshold int) ConnOption {
	return compression{
		mode:      mode,
		threshold: threshold,
	}
}

// WithHTTPClient provides an http.Client to override the default one used.
func WithHTTPClient(client *http.Client) DialOption {
	return optionFn(func(opts *dialOpts) {
		opts.client = client
	})
}

// WithHeaders adds custom headers to every dial HTTP request.
func WithHeaders(headers http.Header) DialOption {
	return optionFn(func(opts *dialOpts) {
		opts.headers = headers
	})
}

// Dial creates a connection to the given endpoint. By default, it's a non-blocking
// dial (the function won't wait for connections to be established, and connecting
// happens in the background).
//
func Dial(ctx context.Context, endpoint string, opts ...DialOption) (*Conn, error) {
	dopts := &dialOpts{
		client: http.DefaultClient,
	}

	for _, opt := range opts {
		opt.SetDial(dopts)
	}

	d := &websocket.DialOptions{
		HTTPClient:           dopts.client,
		HTTPHeader:           dopts.headers,
		Subprotocols:         []string{"graphql-ws"},
		CompressionMode:      websocket.CompressionMode(dopts.compression),
		CompressionThreshold: dopts.threshold,
	}

	// TODO: Handle resp
	wc, _, err := websocket.Dial(ctx, endpoint, d)
	if err != nil {
		return nil, err
	}

	return newConn(wc), nil
}
