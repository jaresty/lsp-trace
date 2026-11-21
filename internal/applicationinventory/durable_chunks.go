//go:build darwin || linux

package applicationinventory

import (
	"context"
	"encoding/json"
	"time"

	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
	"lsp-trace/internal/publication"
)

const inventoryChunkBytes = 32768

type inventoryChunkIndex struct {
	Version, RunID, TaskID, PreviousTaskID string
	Original                               publication.StoredObject
	Chunks                                 []publication.StoredObject
}

type inventoryDelivery struct {
	Index publication.StoredObject
	// Next is a funded-attempt cursor, NOT a receipt acknowledgement.
	Next int
}

func chunkCount(n int) int         { return (n + inventoryChunkBytes - 1) / inventoryChunkBytes }
func chunkIndexUpper(n int) uint64 { return 1024 + 128*uint64(chunkCount(n)) }
func chunkBuildQuote(n int) (uint64, error) {
	if n < 1 || n > 8<<20 {
		return 0, ErrDurableInventory
	}
	// Full original read/hash (6n), chunk publication and verified readback (8n),
	// index allocation/encoding/publication/readback/strict decoding (16i).
	return 14*uint64(n) + 16*chunkIndexUpper(n) + 32768, nil
}
func chunkPageQuote(total, indexBytes, offset, limit int) (uint64, int, error) {
	if total < 1 || total > 8<<20 || indexBytes < 1 || uint64(indexBytes) > chunkIndexUpper(total) || offset < 0 || offset >= total || limit < 1 || limit > inventoryChunkBytes {
		return 0, 0, ErrDurableInventory
	}
	count := min(limit, total-offset)
	first, last := offset/inventoryChunkBytes, (offset+count-1)/inventoryChunkBytes
	verified := min(total, (last+1)*inventoryChunkBytes) - first*inventoryChunkBytes
	return 8*uint64(indexBytes) + 6*uint64(verified) + 2*uint64(count) + 16384, count, nil
}
func chunkDeliveryQuote(total, indexBytes, offset int) (uint64, uint64, error) {
	if offset < 0 || offset > total || offset != total && offset%inventoryChunkBytes != 0 {
		return 0, 0, ErrDurableInventory
	}
	var n, packets uint64
	for at := offset; at < total; at += inventoryChunkBytes {
		q, _, e := chunkPageQuote(total, indexBytes, at, inventoryChunkBytes)
		if e != nil {
			return 0, 0, e
		}
		n += q
		packets++
	}
	return n, packets, nil
}
func (h durableInventoryHead) deliveryRemaining() (uint64, uint64, bool) {
	if h.Delivery.Index == (publication.StoredObject{}) {
		return 0, 0, h.Delivery.Next == 0
	}
	if !h.Delivery.Index.Valid() {
		return 0, 0, false
	}
	n, p, e := chunkDeliveryQuote(h.Input.Bytes, h.Delivery.Index.Bytes, h.Delivery.Next)
	return n, p, e == nil
}
func (s DurableStore) buildDelivery(ctx context.Context, h *durableInventoryHead, b *sc.CallerBudgetV3) error {
	h.Delivery = inventoryDelivery{}
	if h.Phase == "Published" {
		return nil
	}
	quote, e := chunkBuildQuote(h.Input.Bytes)
	if e != nil || b.Reserve(ctx, quote) != nil {
		return ErrDurableInventory
	}
	raw, e := publication.ReadObject(s.Root, h.InputPrefix, h.Input)
	if e != nil {
		return ErrDurableInventory
	}
	index := inventoryChunkIndex{Version: "lsp-trace.inventory-chunks.v1", RunID: h.RunID, TaskID: h.TaskID, PreviousTaskID: h.PreviousTaskID, Original: h.Input, Chunks: make([]publication.StoredObject, 0, chunkCount(len(raw)))}
	for at := 0; at < len(raw); at += inventoryChunkBytes {
		if ctx.Err() != nil {
			return ErrDurableInventory
		}
		ref, e := publication.StoreObject(s.Root, "inventory-chunk-", raw[at:min(len(raw), at+inventoryChunkBytes)])
		if e != nil {
			return ErrDurableInventory
		}
		index.Chunks = append(index.Chunks, ref)
	}
	encoded, e := json.Marshal(index)
	if e != nil || uint64(len(encoded)) > chunkIndexUpper(len(raw)) {
		return ErrDurableInventory
	}
	ref, e := publication.StoreObject(s.Root, "inventory-chunk-index-", encoded)
	if e != nil {
		return ErrDurableInventory
	}
	h.Delivery.Index = ref
	_, e = s.readChunkIndex(*h)
	return e
}
func (s DurableStore) readChunkIndex(h durableInventoryHead) (inventoryChunkIndex, error) {
	var v inventoryChunkIndex
	raw, e := publication.ReadObject(s.Root, "inventory-chunk-index-", h.Delivery.Index)
	if e != nil || strictInventory(raw, &v) != nil || v.Version != "lsp-trace.inventory-chunks.v1" || v.RunID != h.RunID || v.TaskID != h.TaskID || v.PreviousTaskID != h.PreviousTaskID || v.Original != h.Input || len(v.Chunks) != chunkCount(h.Input.Bytes) {
		return v, ErrDurableInventory
	}
	for i, r := range v.Chunks {
		if !r.Valid() || r.Bytes != min(inventoryChunkBytes, h.Input.Bytes-i*inventoryChunkBytes) {
			return v, ErrDurableInventory
		}
	}
	return v, nil
}

func (s DurableStore) reserveChunkRead(ctx context.Context, h durableInventoryHead, old []byte, offset, limit int) (durableInventoryHead, int, error) {
	bad := func() (durableInventoryHead, int, error) { return h, 0, ErrDurableInventory }
	quote, count, e := chunkPageQuote(h.Input.Bytes, h.Delivery.Index.Bytes, offset, limit)
	if e != nil || ctx == nil || ctx.Err() != nil || h.Status != "READY" || s.accepted(h) != nil {
		return bad()
	}
	required := offset == h.Delivery.Next && count == min(inventoryChunkBytes, h.Input.Bytes-offset)
	if !required {
		h, _, e = s.reserve(ctx, h, old, quote, false, false)
	} else {
		if !h.completionValid() || quote > h.Limit-h.Used || h.Packets >= 64 {
			return bad()
		}
		ctx, cancel := context.WithDeadline(ctx, time.Unix(0, h.operationDeadline()))
		defer cancel()
		h.Delivery.Next += count
		h.DeliverySpent += quote
		h.Used += quote
		h.Work += 32 * quote
		h.Packets++
		_, e = s.change(ctx, old, h)
	}
	if e != nil {
		return bad()
	}
	return h, count, nil
}

func (s DurableStore) readChunks(ctx context.Context, h durableInventoryHead, old []byte, offset, limit int) (sc.DurableDescribePageV1, error) {
	bad := func() (sc.DurableDescribePageV1, error) { return sc.DurableDescribePageV1{}, ErrDurableInventory }
	h, count, e := s.reserveChunkRead(ctx, h, old, offset, limit)
	if e != nil {
		return bad()
	}
	// Reservation is durable before index/chunk access. Any failure or lost output
	// spends this attempt. No retry loop or automatic CAS rebase exists here.
	index, e := s.readChunkIndex(h)
	if e != nil {
		return bad()
	}
	data := make([]byte, 0, count)
	end := offset + count
	for i := offset / inventoryChunkBytes; i <= (end-1)/inventoryChunkBytes; i++ {
		raw, e := publication.ReadObject(s.Root, "inventory-chunk-", index.Chunks[i])
		if e != nil {
			return bad()
		}
		lo := max(offset-i*inventoryChunkBytes, 0)
		hi := min(end-i*inventoryChunkBytes, len(raw))
		data = append(data, raw[lo:hi]...)
	}
	if len(data) != count || ctx.Err() != nil || time.Now().UnixNano() >= h.operationDeadline() {
		return bad()
	}
	return sc.DurableDescribePageV1{RunID: h.RunID, TaskID: h.TaskID, ArtifactID: "input", Offset: offset, Total: h.Input.Bytes, Data: data}, nil
}
