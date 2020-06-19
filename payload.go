package gws

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

type reqType string

const (
	// Client -> Server
	gqlConnectionInit      reqType = "connection_init"
	gqlStart                       = "start"
	gqlStop                        = "stop"
	gqlConnectionTerminate         = "connection_terminate"

	// Server -> Client
	gqlConnectionError     = "connection_error"
	gqlConnectionAck       = "connection_ack"
	gqlData                = "data"
	gqlError               = "error"
	gqlComplete            = "complete"
	gqlConnectionKeepAlive = "connection_keep_alive"
)

// Request represents a payload sent from the client.
type Request struct {
	Query         string                 `json:"query"`
	Variables     map[string]interface{} `json:"variables,omitempty"`
	OperationName string                 `json:"operationName,omitempty"`
}

// Response represents a payload returned from the server. It supports
// lazy decoding by leaving the inner data for the user to decode.
//
type Response struct {
	Data   json.RawMessage   `json:"data"`
	Errors []json.RawMessage `json:"errors,omitempty"`
}

// ServerError represents a payload which is sent by the server if
// it encounters a non-GraphQL resolver error.
//
type ServerError struct {
	Msg string `json:"msg"`
}

// Error implements the error interface.
func (e *ServerError) Error() string {
	return fmt.Sprintf("internal server error: %s", e.Msg)
}

// payload represents either a Client or Server payload
type payload interface {
	isPayload()
}

func (*Request) isPayload()     {}
func (*Response) isPayload()    {}
func (*ServerError) isPayload() {}

type unknown map[string]interface{}

func (unknown) isPayload() {}

// opID represents a unique id per user request
type opID string

// operationMessage represents an Apollo "GraphQL over WebSockets Protocol" message
type operationMessage struct {
	ID      opID    `json:"id,omitempty"`
	Type    reqType `json:"type"`
	Payload payload `json:"payload,omitempty"`
}

func (m *operationMessage) UnmarshalJSON(b []byte) error {
	var raw struct {
		ID      opID            `json:"id,omitempty"`
		Type    reqType         `json:"type"`
		Payload json.RawMessage `json:"payload,omitempty"`
	}
	err := json.Unmarshal(b, &raw)
	if err != nil {
		return err
	}

	m.Type = raw.Type
	if raw.ID != "" {
		m.ID = raw.ID
	}

	if len(raw.Payload) == 0 {
		return nil
	}

	switch raw.Type {
	case gqlConnectionInit, gqlStart, gqlStop, gqlConnectionTerminate:
		req := new(Request)
		m.Payload = req
		return json.Unmarshal(raw.Payload, req)
	case gqlConnectionError, gqlConnectionAck, gqlData, gqlComplete, gqlConnectionKeepAlive:
		resp := new(Response)
		m.Payload = resp
		return json.Unmarshal(raw.Payload, resp)
	case gqlError:
		serr := new(ServerError)
		m.Payload = serr
		return json.Unmarshal(raw.Payload, serr)
	default:
		return fmt.Errorf("unsupported message type: %s", raw.Type)
	}
}

func (m *operationMessage) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer

	b.Write([]byte("{"))

	if m.ID != "" {
		b.Write([]byte(`"id":"`))
		b.Write([]byte(m.ID))
		b.Write([]byte(`",`))
	}

	b.Write([]byte(`"type":"`))
	b.Write([]byte(m.Type))
	b.Write([]byte("\""))

	if m.Payload != nil {
		b.Write([]byte(`,"payload":`))
		marshalPayload(&b, m.Payload)
	}

	b.Write([]byte("}"))

	return b.Bytes(), nil
}

func marshalPayload(w io.Writer, payload payload) error {
	switch v := payload.(type) {
	case *Request:
		w.Write([]byte(`{"query":"`))

		w.Write([]byte(v.Query))
		w.Write([]byte("\""))

		if v.Variables != nil {
			w.Write([]byte(`,"variables":`))
			b, err := json.Marshal(v.Variables)
			if err != nil {
				return err
			}

			w.Write(b)
		}

		if v.OperationName != "" {
			w.Write([]byte(`,"operationName":"`))
			w.Write([]byte(v.OperationName))
			w.Write([]byte("\""))
		}

		w.Write([]byte("}"))
	case *Response:
		w.Write([]byte(`{"data":`))
		w.Write(v.Data)

		if v.Errors != nil {
			w.Write([]byte(`,"errors":[`))

			l := len(v.Errors) - 1
			for i, err := range v.Errors {
				w.Write(err)

				if i < l {
					w.Write([]byte(","))
				}
			}

			w.Write([]byte("]"))
		}

		w.Write([]byte("}"))
	case *ServerError:
		w.Write([]byte(`{"msg":"`))
		w.Write([]byte(v.Msg))
		w.Write([]byte(`"}`))
	}

	return nil
}
