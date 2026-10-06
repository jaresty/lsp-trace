package sessionruntime

import (
	"context"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
)

const (
	privateB4UnsupportedResponsePrefix = `{"jsonrpc":"2.0","id":`
	privateB4UnsupportedResponseSuffix = `,"error":{"code":-32601,"message":"Method not found"}}`
)

func writePrivateB4UnsupportedResponse(ctx context.Context, owner *ownedTransport, writer *lspwire.Writer, account *privateB4ByteAccountV2, frame []byte, observation lspwire.SuccessorReadObservation) (session.Failure, bool) {
	bodyStart := observation.BodyOffset
	idStart := bodyStart + observation.RequestIDStart
	idEnd := bodyStart + observation.RequestIDEnd
	if observation.RequestIDStart >= observation.RequestIDEnd || bodyStart > uint64(len(frame)) || idStart < bodyStart || idEnd < idStart || idEnd > uint64(len(frame)) {
		return session.SessionPoisoned, true
	}
	id := frame[idStart:idEnd]
	bodyCapacity := uint64(len(privateB4UnsupportedResponsePrefix)) + uint64(len(id)) + uint64(len(privateB4UnsupportedResponseSuffix))
	if bodyCapacity > uint64(^uint(0)>>1) {
		return session.ResourceExhausted, false
	}
	frameCapacity := canonicalRequestFrameBytes(int(bodyCapacity))
	if frameCapacity < int64(bodyCapacity) {
		return session.ResourceExhausted, false
	}
	headerCapacity := uint64(frameCapacity) - bodyCapacity
	var leases [2]privateB4ByteLeaseV2
	if _, failure := account.reserveMany([]uint64{bodyCapacity, headerCapacity}, leases[:]); failure != "" {
		return failure, false
	}
	defer leases[1].release()
	defer leases[0].release()

	body := make([]byte, int(bodyCapacity))
	n := copy(body, privateB4UnsupportedResponsePrefix)
	n += copy(body[n:], id)
	n += copy(body[n:], privateB4UnsupportedResponseSuffix)
	if n != len(body) {
		return session.SessionPoisoned, true
	}
	if err := owner.run(ctx, func() error { return writer.WriteEncodedBodyPrivate(body) }); err != nil {
		return session.SessionPoisoned, true
	}
	return "", false
}
