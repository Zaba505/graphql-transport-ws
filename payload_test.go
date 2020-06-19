package gws

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
)

func TestOpMessage_Unmarshal(t *testing.T) {
	testCases := []struct {
		Name    string
		JSON    string
		Payload payload
		Err     error
	}{
		{
			Name: "NoPayload",
			JSON: `
{
  "type": "connection_init"
}
`,
		},
		{
			Name: "WithRequest",
			JSON: `
{
  "id": "1",
  "type": "start",
  "payload": {
    "query": "{ hello { world } }"
  }
}`,
			Payload: &Request{Query: "{ hello { world } }"},
		},
		{
			Name: "WithResponse",
			JSON: `
{
  "id": "1",
  "type": "data",
  "payload": {
    "data": {"hello":{"world":"this is a test"}}
  }
}
`,
			Payload: &Response{Data: json.RawMessage([]byte(`{"hello":{"world":"this is a test"}}`))},
		},
		{
			Name: "Unordered",
			JSON: `
{
  "id": "1",
  "payload": {
    "query": "{ hello { world } }"
  },
  "type": "start"
}`,
			Payload: &Request{Query: "{ hello { world } }"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(subT *testing.T) {
			msg := new(operationMessage)

			err := json.Unmarshal([]byte(testCase.JSON), msg)
			if err != nil && testCase.Err == nil {
				subT.Errorf("unexpected error when unmarshaling: %s", err)
				return
			}
			if err != nil && err != testCase.Err {
				subT.Logf("expected error: %s, but got: %s", testCase.Err, err)
				subT.Fail()
				return
			}

			if testCase.Payload == nil {
				return
			}

			if msg.Payload == nil {
				subT.Logf("expected payload: %v, but got nothing", testCase.Payload)
				subT.Fail()
				return
			}

			comparePayloads(subT, testCase.Payload, msg.Payload)
		})
	}
}

func TestOpMessage_MarshalJSON(t *testing.T) {
	testCases := []struct {
		Name string
		Msg  operationMessage
		Ex   string
	}{
		{
			Name: "Missing Payload",
			Msg:  operationMessage{ID: "1", Type: gqlComplete},
			Ex:   `{"id":"1","type":"complete"}`,
		},
		{
			Name: "Only Type",
			Msg:  operationMessage{Type: gqlConnectionInit},
			Ex:   `{"type":"connection_init"}`,
		},
		{
			Name: "Just Query",
			Msg:  operationMessage{ID: "1", Type: gqlStart, Payload: &Request{Query: "{ hello { world } }"}},
			Ex:   `{"id":"1","type":"start","payload":{"query":"{ hello { world } }"}}`,
		},
		{
			Name: "With Variables",
			Msg:  operationMessage{ID: "1", Type: gqlStart, Payload: &Request{Query: "{ hello { world } }", Variables: map[string]interface{}{"hello": "world"}}},
			Ex:   `{"id":"1","type":"start","payload":{"query":"{ hello { world } }","variables":{"hello":"world"}}}`,
		},
		{
			Name: "Full Request",
			Msg:  operationMessage{ID: "1", Type: gqlStart, Payload: &Request{Query: "{ hello { world } }", Variables: map[string]interface{}{"hello": "world"}, OperationName: "Test"}},
			Ex:   `{"id":"1","type":"start","payload":{"query":"{ hello { world } }","variables":{"hello":"world"},"operationName":"Test"}}`,
		},
		{
			Name: "With Errors",
			Msg:  operationMessage{ID: "1", Type: gqlData, Payload: &Response{Data: []byte(`{"hello":{"world":"test"}}`), Errors: []json.RawMessage{[]byte(`{"msg":"test"}`), []byte(`{"msg":"test"}`)}}},
			Ex:   `{"id":"1","type":"data","payload":{"data":{"hello":{"world":"test"}},"errors":[{"msg":"test"},{"msg":"test"}]}}`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(subT *testing.T) {
			b, err := json.Marshal(&testCase.Msg)
			if err != nil {
				subT.Error(err)
				return
			}

			if !bytes.Equal(b, []byte(testCase.Ex)) {
				subT.Logf("expected: %s, got: %s", string(testCase.Ex), string(b))
				subT.Fail()
				return
			}
		})
	}
}

func TestOpMessage_RoundTrip(t *testing.T) {
	testCases := []struct {
		Name string
		Msg  operationMessage
		Ex   string
	}{
		{
			Name: "All Fields",
			Msg:  operationMessage{ID: "1", Type: gqlStart, Payload: &Request{Query: "{ hello { world } }"}},
			Ex:   `{"id":"1","type":"start","payload":{"query":"{ hello { world } }"}}`,
		},
		{
			Name: "Missing Payload",
			Msg:  operationMessage{ID: "1", Type: gqlComplete},
			Ex:   `{"id":"1","type":"complete"}`,
		},
		{
			Name: "Only Type",
			Msg:  operationMessage{Type: gqlConnectionInit},
			Ex:   `{"type":"connection_init"}`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(subT *testing.T) {
			subT.Parallel()

			b, err := json.Marshal(&testCase.Msg)
			if err != nil {
				subT.Error(err)
				return
			}

			var msg operationMessage
			err = json.Unmarshal(b, &msg)
			if err != nil {
				subT.Error(err)
				return
			}

			if !reflect.DeepEqual(msg, testCase.Msg) {
				subT.Logf("expected: %#v, got: %#v", testCase.Msg, msg)
				subT.Fail()
				return
			}
		})
	}
}

const benchReq = `{
  "id": "1",
  "type": "start",
  "payload": {
    "query": "{ hello { world } }"
  }
}`

func BenchmarkOpMessage_UnmarshalJSON(b *testing.B) {
	b.Run("Via UnmarshalJSON", func(subB *testing.B) {
		for i := 0; i < subB.N; i++ {
			msg := new(operationMessage)
			err := msg.UnmarshalJSON([]byte(benchReq))
			if err != nil {
				subB.Error(err)
			}
		}
	})

	b.Run("Via json.Unmarshal", func(subB *testing.B) {
		for i := 0; i < subB.N; i++ {
			msg := new(operationMessage)
			err := json.Unmarshal([]byte(benchReq), msg)
			if err != nil {
				subB.Error(err)
			}
		}
	})
}

func comparePayloads(t *testing.T, ex, out payload) {
	t.Helper()

	switch u := ex.(type) {
	case *Request:
		v, ok := out.(*Request)
		if !ok {
			t.Logf("expected payload: request, but got: %#v", out)
			t.Fail()
			return
		}

		if v.Query != u.Query || v.OperationName != u.OperationName {
			t.Logf("requests aren't equal: %v::%v", u, v)
			t.Fail()
			return
		}
	case *Response:
		v, ok := out.(*Response)
		if !ok {
			t.Logf("expected payload: request, but got: %#v", out)
			t.Fail()
			return
		}

		if !bytes.Equal(u.Data, v.Data) {
			t.Logf("expected data: %s, but got: %s", string(u.Data), string(v.Data))
			t.Fail()
			return
		}
	case unknown:
		t.Log("expected payload is: unknown which should never happen")
		t.Fail()
	}
}
