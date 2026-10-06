package sessionruntime

import (
	"encoding/json"
	"errors"
	"runtime"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
)

const privateB4QualifiedGoVersion = "go1.26.5"

var errPrivateB4EncoderShape = errors.New("unsupported private B4 encoding/json shape")

type privateB4EncoderSink struct {
	account       *privateB4ByteAccountV2
	maxBody       int64
	beforeReserve func(*privateB4ByteAccountV2)
	body          []byte
	lease         privateB4ByteLeaseV2
	failure       session.Failure
	calls         int
}

func (s *privateB4EncoderSink) Write(p []byte) (int, error) {
	s.calls++
	if s.calls != 1 || len(p) == 0 || p[len(p)-1] != '\n' {
		return 0, errPrivateB4EncoderShape
	}
	bodyLen := len(p) - 1
	if int64(bodyLen) > s.maxBody {
		s.failure = session.ResourceExhausted
		return 0, errPrivateB4EncoderShape
	}
	if s.beforeReserve != nil {
		s.beforeReserve(s.account)
	}
	lease, failure := s.account.reservePrivateB4OutboundTransport(bodyLen)
	if failure != "" {
		s.failure = failure
		return 0, errPrivateB4EncoderShape
	}
	s.lease = lease
	s.body = make([]byte, bodyLen)
	copy(s.body, p[:bodyLen])
	return len(p), nil
}

func encodePrivateB4Request(message lspwire.Message, account *privateB4ByteAccountV2, limits lspwire.Limits, beforeReserve func(*privateB4ByteAccountV2)) ([]byte, privateB4ByteLeaseV2, session.Failure, error) {
	if runtime.Version() != privateB4QualifiedGoVersion {
		return nil, privateB4ByteLeaseV2{}, session.ResourceExhausted, errPrivateB4EncoderShape
	}
	maxBody := limits.MaxBodyBytes
	if maxBody <= 0 {
		maxBody = lspwire.DefaultLimits().MaxBodyBytes
	}
	sink := &privateB4EncoderSink{account: account, maxBody: maxBody, beforeReserve: beforeReserve}
	err := json.NewEncoder(sink).Encode(message)
	if err != nil || sink.calls != 1 || sink.body == nil {
		sink.lease.release()
		if sink.failure == "" {
			sink.failure = session.ResourceExhausted
		}
		return nil, privateB4ByteLeaseV2{}, sink.failure, err
	}
	return sink.body, sink.lease, "", nil
}
