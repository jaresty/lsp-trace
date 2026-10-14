#!/usr/bin/env python3
"""Generate ADR0011 C15 StageB v5 correction-safe qualification package.

Docs/evidence-only. The enumerator intentionally derives actual row IDs from
current source path/function/line/operation/component/backing rule and never from
StageA planned IDs.
"""
from __future__ import annotations

import argparse
import csv
import hashlib
import json
import os
import re
import subprocess
import sys
from dataclasses import dataclass, asdict
from pathlib import Path
from typing import Iterable

PACKAGE = "docs/qualification/adr0011-directional-raw-custody-c15-stageb-v5"
STAGEA = "docs/qualification/adr0011-directional-raw-custody-c15-two-stage-contract-v2/STAGE_A_PLANNED_OBLIGATIONS.tsv"
IDENTITY = "docs/qualification/adr0011-directional-raw-custody-c15-two-stage-contract-v2/IDENTITY_ACCOUNTING_47_PLUS_1.tsv"
V5README = "docs/qualification/adr0011-directional-raw-custody-real-path-vertical-slice-v5/README.md"
ROW_NS = "adr0011.c15.stageb.v5.actual.v1"
LINEAGE_NS = "adr0011.c15.stageb.v5.lineage.v1"

@dataclass(frozen=True)
class RowSpec:
    path: str
    function: str
    snippet: str
    operation_kind: str
    component: str
    backing_identity_rule: str
    classification: str
    rationale: str
    contrary_evidence_or_falsifier: str
    predecessor_hint: str
    authority_ceiling: str = "AUTHORITY_0_PROVISIONAL_SOURCE_EVIDENCE_ONLY"

@dataclass
class ActualRow:
    actual_row_id: str
    source_path: str
    source_line: int
    source_sha256: str
    source_line_sha256: str
    function: str
    operation_kind: str
    component: str
    backing_identity_rule: str
    operation_excerpt: str
    classification: str
    rationale: str
    contrary_evidence_or_falsifier: str
    predecessor_lineage: str
    authority_ceiling: str

@dataclass(frozen=True)
class CandidatePattern:
    path: str
    family: str
    pattern: str

@dataclass(frozen=True)
class CandidateExclusion:
    path: str
    snippet: str
    family: str
    falsifier: str

@dataclass
class SourceCandidate:
    candidate_id: str
    source_path: str
    source_line: int
    source_line_sha256: str
    family: str
    operation_excerpt: str
    disposition: str
    actual_row_id: str
    exclusion_falsifier: str

SPECS: list[RowSpec] = [
    RowSpec("incomingops/directional_private.go", "directionalPrivateEnabled", "return ok && directionalraw.Allowed(method)", "DIRECTIONAL_RESERVATION_CONTEXT_ALLOWED_GATE", "PROCESS_LOCAL_RESERVATION", "context_reservation_and_allowed_method_gate", "NECESSARY", "Observes process-local reservation context and allowed-method gate before the incomingops private directional dispatch is enabled.", "If either FromContext or Allowed is removed from this gate, private dispatch can be skipped or enabled for an unsupported method without reservation evidence.", "STAGEB_EMERGENT_V5_REVIEW_OMISSION_CORRECTION"),
    RowSpec("incomingops/directional_private.go", "(*Executor).ExecuteDirectionalPrivate", "r := directionalraw.NewReservation(transactionID)", "DIRECTIONAL_RESERVATION_CREATE", "PROCESS_LOCAL_RESERVATION", "transaction_id_reservation_identity", "NECESSARY", "Creates the process-local reservation token before any directional call can attach Manager custody.", "If ExecuteDirectionalPrivate used ordinary Execute without NewReservation/Context, directional path would be unavailable and no directional lease could bind.", "STAGEB_EMERGENT_V5"),
    RowSpec("incomingops/directional_private.go", "(*Executor).ExecuteDirectionalPrivate", "result, failure := e.Execute(r.Context(parent), op)", "DIRECTIONAL_RESERVATION_CONTEXT_TRANSFER", "PROCESS_LOCAL_RESERVATION", "reservation_context_value_identity", "NECESSARY", "Transfers reservation through context into acquisition/session runtime without public schema activation.", "Removing Context(parent) would make directionalraw.FromContext false at downstream gates.", "STAGEB_EMERGENT_V5"),
    RowSpec("incomingops/directional_private.go", "(*SessionClient).callDirectionalPrivate", "raw, err := json.Marshal(params)", "COPY_EPHEMERAL_REQUEST_PARAMS", "REQUEST_PARAMS", "caller_params_to_ephemeral_wire_json_not_retained_raw", "FALSE_POSITIVE", "This is a transient JSON marshal before Manager-owned reservation; it is enumerated as a copy lead but is not retained C15 raw custody.", "Retained raw custody begins at lspwire.WriteDirectionalReserved lease.Allocate; this marshal result is not preserved as retained backing.", "STAGEA_PARAMETER_COPY_SUPERSEDED_BY_RESERVED_FRAME"),
    RowSpec("incomingops/directional_private.go", "(*SessionClient).callDirectionalPrivate", "observed, lease := runtime.RoundTripDirectionalPrivate", "TRANSFER_DIRECTIONAL_LEASE_TO_CLIENT", "LEASE", "runtime_returns_directional_lease_for_decode", "NECESSARY", "Moves the Manager-created lease back to the private client for binding and decoding.", "If the lease were not returned, BindDirectionalResponse and Decode could not prove response aliases remain inside reserved backing.", "STAGEB_EMERGENT_V5"),
    RowSpec("incomingops/directional_private.go", "(*SessionClient).callDirectionalPrivate", "if err = acquisition.BindDirectionalResponse(lease); err != nil", "ALIAS_BIND_RESPONSE_BEFORE_DECODE", "RESPONSE_RESULT", "lexical_offsets_bound_before_strict_decode", "NECESSARY", "Binds lexical offsets before strict typed decode in the direct incomingops client path.", "Skipping binding makes Select/View unavailable or allows decode without recorded result spans.", "STAGEB_EMERGENT_V5"),
    RowSpec("incomingops/directional_private.go", "(*SessionClient).callDirectionalPrivate", "return false, lease.Decode(target)", "ALIAS_VIEW_STRICT_DECODE_DIRECT", "RESPONSE_RESULT", "direct_incomingops_decode_reads_from_lease_result_view", "NECESSARY", "Direct incomingops client decode reads from the alias-backed lease result after binding, not from a retained copied response byte slice.", "Replacing this with decoding of a returned []byte or json.RawMessage copy would reintroduce second retained response copy risk.", "STAGEB_EMERGENT_V5_REVIEW_OMISSION_CORRECTION"),

    RowSpec("incomingops/executor.go", "(*SessionClient).call", "return c.callDirectionalPrivate(parent, method, params, target)", "DIRECTIONAL_DISPATCH_TO_PRIVATE_CLIENT", "ACQUISITION_CLIENT", "directional_private_enabled_dispatches_before_ordinary_marshal", "NECESSARY", "Directional dispatch transfers the request into callDirectionalPrivate before the ordinary JSON marshal/roundTrip branch can run.", "If this dispatch is removed or moved after ordinary marshaling, directional private custody can be bypassed for reserved contexts.", "STAGEB_EMERGENT_V5_REVIEW_OMISSION_CORRECTION"),

    RowSpec("internal/acquisition/sessionclient/client.go", "New", "client.SetDirectionalPrivate", "TRANSFER_RUNTIME_DIRECTIONAL_CALLBACK", "ACQUISITION_CLIENT", "runtime_directional_callback_installed_if_supported", "NECESSARY", "Connects neutral acquisition clients to an existing exact runtime directional private seam.", "Without SetDirectionalPrivate, acquisition.WireClient rejects directional context as runtime unavailable.", "STAGEB_EMERGENT_V5"),
    RowSpec("internal/acquisition/sessionclient/client.go", "New.func1", "result, lease := directional.RoundTripDirectionalPrivate", "TRANSFER_DIRECTIONAL_LEASE_TO_ACQUISITION", "LEASE", "runtime_returns_lease_to_neutral_acquisition", "NECESSARY", "Transfers directional lease to neutral acquisition for shape validation/decode while keeping runtime authority in Manager.", "Returning only result JSON would force an extra retained response copy or lose backing alias proof.", "STAGEB_EMERGENT_V5"),
    RowSpec("internal/acquisition/wire.go", "BindDirectionalResponse", "return lease.BindElementsPrivate()", "ALIAS_BIND_RESPONSE_BEFORE_DECODE", "RESPONSE_RESULT", "lexical_offsets_bound_before_strict_decode", "NECESSARY", "Single neutral binding function captures lexical offsets before strict target decode.", "If this returned nil without BindElementsPrivate, ordinal Select rows would be unavailable.", "STAGEB_EMERGENT_V5"),
    RowSpec("internal/acquisition/wire.go", "(*WireClient).call", "if _, enabled := directionalraw.FromContext(ctx); enabled && directionalraw.Allowed(method)", "DIRECTIONAL_RESERVATION_CONTEXT_OBSERVE", "ACQUISITION_CLIENT", "context_reservation_controls_private_path", "NECESSARY", "Partitions directional path from ordinary roundTrip by explicit context reservation and allowed method.", "If this guard were absent, ordinary roundTrip could be used and no lease-backed aliasing would be enforced.", "STAGEB_EMERGENT_V5"),
    RowSpec("internal/acquisition/wire.go", "(*WireClient).call", "return false, lease.Decode(target)", "ALIAS_VIEW_STRICT_DECODE", "RESPONSE_RESULT", "decode_reads_from_lease_result_view", "NECESSARY", "Decodes target from lease-held result alias after shape validation, not from an independent retained copy.", "Replacing with decode of returned []byte would reintroduce second retained response copy risk.", "STAGEB_EMERGENT_V5"),

    RowSpec("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "(*Manager).RoundTripDirectionalPrivate", "if !ok || r.IdentityPrivate() == \"\" || !directionalraw.Allowed(req.Method) || req.EnablePrivateC16ObjectAccounting || req.EnablePrivateC17EventAccounting", "DIRECTIONAL_MANAGER_COMPOUND_GATE", "MANAGER_INGRESS", "identity_method_accounting_gate_before_private_path", "NECESSARY", "Manager refuses directional mode unless explicit reservation identity, allowed method, and no competing private accounting modes all hold.", "Removing identity, method, or accounting-mode checks would permit an unreserved, unsupported, or compound private accounting path.", "STAGEB_EMERGENT_V5_REVIEW_OMISSION_CORRECTION"),
    RowSpec("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "(*Manager).RoundTripDirectionalPrivate", "req.directionalV5 = &privateDirectionalCallV5", "DIRECTIONAL_CALL_STATE_ALLOCATE", "MANAGER_DIRECTIONAL_STATE", "per_request_directional_call_state", "NECESSARY", "Installs per-request directional state; ordinary RoundTrip remains unchanged.", "If req.directionalV5 stayed nil, admission/allocation/finish hooks would not run.", "STAGEB_EMERGENT_V5"),
    RowSpec("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "(*Manager).RoundTripDirectionalPrivate", "result.Notifications = nil", "OBSERVATION_RETENTION_NEGATIVE_NO_PUBLIC_NOTIFICATIONS", "EXTERNAL_FRAMES", "reserved_notifications_not_public_raw", "NECESSARY", "Explicitly prevents notification external frames from being exposed as public raw response slices.", "Removing nil assignment would let notification Message.RawMessage aliases escape outside private qualification boundary.", "STAGEB_EMERGENT_V5"),
    RowSpec("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "(*Manager).RoundTripDirectionalPrivate", "result.Responses = nil", "OBSERVATION_RETENTION_NEGATIVE_NO_PUBLIC_RESPONSES", "EXTERNAL_FRAMES", "reserved_responses_not_public_raw", "NECESSARY", "Explicitly prevents response external frames from being exposed as public raw response slices.", "Removing nil assignment would let response Message.RawMessage aliases escape outside private qualification boundary.", "STAGEB_EMERGENT_V5_REVIEW_OMISSION_CORRECTION"),
    RowSpec("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "(*Manager).admitDirectionalV5Locked", "d = &privateDirectionalV5{reservation: r, frameObjects: make(map[uint64]string), responseObjects: make(map[uint64]uint64)}", "DIRECTIONAL_MANAGER_STATE_ALLOCATE", "MANAGER_DIRECTIONAL_STATE", "reservation_scoped_manager_state", "NECESSARY", "Allocates Manager-owned directional state maps once per reservation.", "Without reservation-scoped state, response-object and frame-object release/alias tracking cannot close.", "STAGEB_EMERGENT_V5"),
    RowSpec("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "(*Manager).admitDirectionalV5Locked", "custody, failure := newPrivateB4C15CustodyContext", "BASELINE_GENERAL_C15_CONTEXT_REUSE", "C15_CUSTODY_CONTEXT", "inherited_c15_custody_context_for_directional_v5", "REUSE", "Reuses inherited C15 custody event infrastructure for directional v5 objects.", "If new custody context were absent, C15 object create/alias/release events would not be retained.", "STAGEA_EXISTING_SOURCE_REUSE"),
    RowSpec("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "(*Manager).admitDirectionalV5Locked", "account, failure := newPrivateB4ByteAccountV2(custody)", "BASELINE_GENERAL_C15_ACCOUNT_REUSE", "C15_ACCOUNT", "inherited_v2_account_owner_for_directional_v5", "REUSE", "Reuses inherited V2 byte-account/table owner; directional partition is additional state, not a second general table.", "If a separate ad hoc ledger were created, Manager would no longer be sole C15 accounting owner.", "STAGEA_EXISTING_SOURCE_REUSE"),
    RowSpec("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "(*Manager).admitDirectionalV5Locked", "if !r.Attach(d.allocate, d.observe, func() bool { return (callHierarchyLayer1GenericLease{owner: o}).Release() }, d.releaseAccount)", "DIRECTIONAL_RESERVATION_ATTACH", "RESERVATION_CALLBACKS", "manager_supplies_allocate_observe_terminal_release", "NECESSARY", "Attaches Manager allocation, observation, generic terminal, and release callbacks to the reservation.", "Any missing callback breaks one of allocation, observation retention, terminal convergence, or C15 release.", "STAGEB_EMERGENT_V5"),
    RowSpec("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "(*Manager).admitDirectionalV5Locked", "d.owner.active++", "DIRECTIONAL_ADMISSION_ACTIVE_COUNT", "GENERIC_OWNER", "generic_owner_active_count_partitions_terminal", "NECESSARY", "Admits exactly one active directional call under the generic owner lifecycle.", "Without active count, terminal release could race while call is in flight.", "STAGEB_EMERGENT_V5"),

    RowSpec("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "(*privateDirectionalV5).allocate", "if component != \"request-frame\" && component != \"response-frame\"", "DIRECTIONAL_COMPONENT_PARTITION", "REQUEST_RESPONSE_PARTITION", "only_request_and_response_frame_backings", "NECESSARY", "Establishes directional/general partition independence as two raw directional frame components only.", "Allowing arbitrary components would violate the request/response partition and StageA component mapping would be silently promoted.", "STAGEA_20_REPLACED_BY_V5_TWO_PARTITION"),
    RowSpec("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "(*privateDirectionalV5).allocate", "if n <= 0 || a.terminalRequested || a.backingReleased", "DIRECTIONAL_RESERVATION_PRECHECK", "REQUEST_RESPONSE_PARTITION", "pre_effect_quota_lifecycle_guard", "NECESSARY", "Refuses impossible/terminal/over-cap allocations before retained raw make.", "Moving make before this guard is a verifier mutation and would prove preallocation false.", "STAGEA_RESERVATION_REALIZED"),
    RowSpec("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "(*privateDirectionalV5).allocate", "objectID, failure := a.c15.create(site, privateB4C15Operation(\"allocation\"))", "ALLOCATION_OBJECT_RECORD", "C15_DIRECTIONAL_OBJECT", "c15_object_created_before_raw_make", "NECESSARY", "Creates C15 ownership object before the only retained raw allocation.", "If make occurred before c15.create, reservation/ownership would not precede effect.", "STAGEA_COPY_ALLOCATE_REALIZED"),
    RowSpec("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "(*privateDirectionalV5).allocate", "d.frameObjects[objectID] = site", "OBSERVATION_RETENTION_FRAME_OBJECT", "C15_DIRECTIONAL_OBJECT", "frame_object_retained_for_terminal_release", "NECESSARY", "Retains frame object identity for terminal release accounting.", "If not recorded, releaseAccount could not release every C15 object.", "STAGEB_EMERGENT_V5"),
    RowSpec("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "(*privateDirectionalV5).allocate", "p.live += uint64(n)", "DIRECTIONAL_RESERVATION_CHARGE", "REQUEST_RESPONSE_PARTITION", "charge_before_only_raw_make", "NECESSARY", "Charges directional partition before the only retained raw allocation.", "A make before p.live update would falsify request/response preallocation.", "STAGEA_RESERVATION_REALIZED"),
    RowSpec("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "(*privateDirectionalV5).allocate", "return d.allocateBackingLocked(component, n), nil", "ALLOCATION_TRANSFER_TO_CARRIER", "REQUEST_RESPONSE_PARTITION", "only_retained_raw_backing_returned_to_lease", "NECESSARY", "Transfers exactly one Manager-owned retained backing to the lease writer/reader.", "A second append/make retained copy would violate no-second-retained-copy/double-charge evidence.", "STAGEA_COPY_ALLOCATE_REALIZED"),
    RowSpec("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "(*privateDirectionalV5).allocateBackingLocked", "d.allocationBoundaries = append(d.allocationBoundaries", "OBSERVATION_RETENTION_ALLOCATION_BOUNDARY", "ALLOCATION_BOUNDARY", "snapshot_immediately_before_raw_make", "NECESSARY", "Retains boundary snapshot immediately before make, proving preallocation rather than final accounting reconstruction.", "If snapshot were taken after make or from final totals, counterfactual allocation-boundary mutation would not be detected.", "STAGEB_EMERGENT_V5"),
    RowSpec("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "(*privateDirectionalV5).allocateBackingLocked", "return make([]byte, n)", "ALLOCATION_ONLY_RETAINED_RAW_MAKE", "REQUEST_RESPONSE_PARTITION", "sole_retained_raw_make_for_directional_frame", "NECESSARY", "The sole retained directional raw allocation; all later result/element views alias this backing.", "Any additional retained make/copy for result bytes would falsify no-second-copy proof.", "STAGEA_COPY_ALLOCATE_REALIZED"),

    RowSpec("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "(*privateDirectionalV5).observe", "d.responseObjects[o.RequestID] = d.lastResponseObject", "OBSERVATION_RETENTION_RESPONSE_OBJECT_MAP", "RESPONSE_FRAME", "request_id_to_response_object_identity", "NECESSARY", "Retains response object identity by request ID for later selected-element aliasing.", "Without mapping, selected element alias could not bind to response frame object.", "STAGEB_EMERGENT_V5"),
    RowSpec("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "(*privateDirectionalV5).observe", "a.c15.alias(d.responseObjects[o.RequestID], \"c15.directional.v5.selected-element\")", "ALIAS_VIEW_C15_SELECTED_ELEMENT", "SELECTED_ELEMENT", "zero_charge_alias_to_response_object", "NECESSARY", "Records selected element as a zero-charge C15 alias of response-frame object, not a new allocation.", "If alias created a new object/allocation, selected element would double-charge retained response bytes.", "STAGEA_ALIAS_REALIZED"),
    RowSpec("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "(*privateDirectionalV5).observe", "d.accepted[i].observations = append(d.accepted[i].observations, o)", "OBSERVATION_RETENTION_APPEND", "OBSERVATION_LEDGER", "accepted_request_observation_list", "NECESSARY", "Retains exact frame/element observations per accepted request.", "Without append, verifier cannot match frame observations to acquired packets.", "STAGEB_EMERGENT_V5"),
    RowSpec("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "(*privateDirectionalV5).releaseAccount", "for id, site := range d.frameObjects", "TERMINAL_RELEASE_C15_FRAME_OBJECTS", "C15_DIRECTIONAL_OBJECT", "all_frame_objects_released_at_terminal", "NECESSARY", "Releases every retained directional frame object through Manager-owned terminal.", "If loop omitted an object, terminal release count would not match allocation count.", "STAGEA_RELEASE_REALIZED"),
    RowSpec("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "(*privateDirectionalV5).releaseAccount", "a.requestTerminalRelease()", "TERMINAL_RELEASE_GENERAL_C15_ACCOUNT", "C15_ACCOUNT", "manager_terminal_requests_general_account_release", "REUSE", "Converges directional release on inherited C15 account terminal release.", "If terminal request were absent, table backing and C15 context would remain live.", "STAGEA_RELEASE_REALIZED"),
    RowSpec("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "(*privateDirectionalCallV5).admit", "c.lease = c.execution.reservation.Begin(method, key.ID, key.Generation)", "DIRECTIONAL_RESERVATION_BEGIN", "LEASE", "request_key_generation_bound_lease_record", "NECESSARY", "Begins per-request lease after pending-key admission, binding method/request/generation.", "Beginning before pending admission would break request/response identity ordering.", "STAGEB_EMERGENT_V5"),
    RowSpec("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "(*privateDirectionalCallV5).finish", "c.lease.Complete(string(failure))", "OBSERVATION_RETENTION_COMPLETE", "LEASE", "terminal_failure_recorded_on_lease", "NECESSARY", "Records exact terminal failure on lease for observations and latest selection state.", "If completion omitted failure, typed Manager terminal evidence would be unavailable.", "STAGEB_EMERGENT_V5"),
    RowSpec("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "(*privateDirectionalCallV5).finish", "if o.admissionClosed", "TERMINAL_RELEASE_GENERIC_OWNER_IF_CLOSED", "GENERIC_OWNER", "generic_terminal_release_after_active_zero", "NECESSARY", "Ensures generic owner terminal releases only after active directional calls finish.", "If release ran before active zero, views could be cleared during in-flight decode.", "STAGEA_RELEASE_REALIZED"),

    RowSpec("internal/lspwire/read_frame.go", "WriteDirectionalReserved", "frame, err := lease.Allocate(\"request-frame\", len(header)+bodyLen)", "ALLOCATION_REQUEST_FRAME_PREALLOCATED", "REQUEST_FRAME", "request_frame_exact_capacity_allocated_before_write", "NECESSARY", "Preallocates exact request frame before writing to transport.", "If allocation moved after Write, request raw bytes would escape before C15 ownership.", "STAGEA_PREPARE_FRAME_REALIZED"),
    RowSpec("internal/lspwire/read_frame.go", "WriteDirectionalReserved", "lease.RefusedPrivate(\"request-frame\", len(header)+bodyLen, sum)", "OBSERVATION_RETENTION_PROPOSED_REFUSAL", "REQUEST_FRAME_REFUSAL", "proposed_not_acquired_refusal_digest", "NECESSARY", "Records proposed request-frame refusal digest without allocating or writing over-cap raw bytes.", "If refusal allocated/wrote, plus-one witness would show transport bytes or retained raw allocation.", "STAGEB_EMERGENT_V5"),
    RowSpec("internal/lspwire/read_frame.go", "WriteDirectionalReserved", "copy(frame, header)", "COPY_INTO_RESERVED_REQUEST_FRAME", "REQUEST_FRAME", "copy_header_into_manager_owned_backing", "NECESSARY", "Copies header into the single reserved request-frame backing; it is not a second retained copy.", "If header were separately retained, request accounting would double charge.", "STAGEA_PREPARE_FRAME_REALIZED"),
    RowSpec("internal/lspwire/read_frame.go", "WriteDirectionalReserved", "at += copy(frame[at:], params)", "COPY_INTO_RESERVED_REQUEST_FRAME", "REQUEST_FRAME", "copy_params_into_manager_owned_backing", "NECESSARY", "Copies params into same reserved request-frame backing after reservation.", "If params were retained separately, request+response preallocation/no-double-copy proof would fail.", "STAGEA_PREPARE_FRAME_REALIZED"),
    RowSpec("internal/lspwire/read_frame.go", "WriteDirectionalReserved", "if !lease.Frame(\"request-frame\", n)", "OBSERVATION_RETENTION_REQUEST_FRAME", "REQUEST_FRAME", "request_frame_observed_after_write_prefix", "NECESSARY", "Commits observed request-frame prefix length/digest after write.", "If Frame observation omitted, exact32 correspondence between acquired packets and Manager observations fails.", "STAGEB_EMERGENT_V5"),

    RowSpec("internal/lspwire/read_frame.go", "(*Reader).ReadDirectionalReserved.retainHeader", "raw, e := lease.Allocate(\"response-frame\", used)", "ALLOCATION_PARTIAL_RESPONSE_FRAME", "RESPONSE_FRAME_PARTIAL", "partial_header_retained_in_manager_owned_backing", "NECESSARY", "Allocates retained backing for malformed/partial response header bytes, still Manager-owned until terminal.", "If partial bytes were not retained/observed, partial external failure evidence would be missing.", "STAGEA_RESPONSE_COMPONENT_REALIZED"),
    RowSpec("internal/lspwire/read_frame.go", "(*Reader).ReadDirectionalReserved.retainHeader", "copy(raw, header[:used])", "COPY_INTO_RESERVED_RESPONSE_FRAME", "RESPONSE_FRAME_PARTIAL", "copy_partial_header_into_manager_owned_backing", "NECESSARY", "Copies partial header into the already reserved response-frame backing.", "If copied before allocation or retained elsewhere, partial witness would double-copy or lose ownership.", "STAGEA_RESPONSE_COMPONENT_REALIZED"),
    RowSpec("internal/lspwire/read_frame.go", "(*Reader).ReadDirectionalReserved", "frame, err := lease.Allocate(\"response-frame\", used+length)", "ALLOCATION_RESPONSE_FRAME_PREALLOCATED", "RESPONSE_FRAME", "response_frame_exact_capacity_allocated_before_body_read", "NECESSARY", "Preallocates full response frame before reading body into retained backing.", "If body read allocated into ordinary buffer first, response preallocation proof would fail.", "STAGEA_RESPONSE_COMPONENT_REALIZED"),
    RowSpec("internal/lspwire/read_frame.go", "(*Reader).ReadDirectionalReserved", "copy(frame, header[:used])", "COPY_INTO_RESERVED_RESPONSE_FRAME", "RESPONSE_FRAME", "copy_header_into_manager_owned_response_backing", "NECESSARY", "Copies header into reserved response-frame backing before body read.", "If header were separately retained, response accounting would double charge.", "STAGEA_RESPONSE_COMPONENT_REALIZED"),
    RowSpec("internal/lspwire/read_frame.go", "(*Reader).ReadDirectionalReserved", "n, err := io.ReadFull(r.r, frame[used:])", "TRANSFER_TRANSPORT_BODY_INTO_RESERVED_FRAME", "RESPONSE_FRAME", "transport_body_read_directly_into_manager_owned_backing", "NECESSARY", "Transfers response body directly into reserved backing; no second retained response copy is created.", "Reading into an intermediate retained buffer would falsify no second retained copy.", "STAGEA_RESPONSE_COMPONENT_REALIZED"),
    RowSpec("internal/lspwire/read_frame.go", "(*Reader).ReadDirectionalReserved", "lease.Frame(\"response-frame\", len(raw))", "OBSERVATION_RETENTION_RESPONSE_FRAME", "RESPONSE_FRAME", "response_frame_observed_after_acquired_prefix", "NECESSARY", "Commits observed response-frame prefix length/digest after acquisition.", "If Frame observation omitted, acquired packet/object correspondence fails.", "STAGEB_EMERGENT_V5"),
    RowSpec("internal/lspwire/read_frame.go", "(*Reader).ReadDirectionalReserved", "fields, err := directionalraw.Fields(frame[used:])", "ALIAS_VIEW_FIELDS_FROM_RESPONSE_BACKING", "RESPONSE_FRAME", "json_fields_slice_reserved_backing", "NECESSARY", "Parses JSON fields as slices of response backing rather than RawMessage unmarshal copies.", "If json.Unmarshal copied raw result fields, response result would become second retained copy.", "STAGEA_ALIAS_REALIZED"),
    RowSpec("internal/lspwire/read_frame.go", "(*Reader).ReadDirectionalReserved", "msg.Result = v", "ALIAS_VIEW_RESULT_FIELD", "RESPONSE_RESULT", "result_rawmessage_aliases_response_backing", "NECESSARY", "Stores result RawMessage as alias to response backing for lease.Result validation.", "If msg.Result were append-copied, no-second-retained-copy proof would fail.", "STAGEA_ALIAS_REALIZED"),

    RowSpec("internal/directionalraw/carrier.go", "Reservation.Attach", "s.allocate, s.observe, s.terminal, s.release = allocate, observe, terminal, release", "DIRECTIONAL_RESERVATION_ATTACH", "RESERVATION_CALLBACKS", "callbacks_installed_once_under_reservation_lock", "NECESSARY", "Stores Manager callbacks as sole allocation/observation/terminal/release authority for the reservation.", "If callbacks could be overwritten, Manager sole terminal and source of allocation would be ambiguous.", "STAGEB_EMERGENT_V5"),
    RowSpec("internal/directionalraw/carrier.go", "Reservation.Begin", "p := &record{state: s, method: method, id: id, generation: generation, frames: make(map[string][]byte)}", "DIRECTIONAL_LEASE_RECORD_ALLOCATE", "LEASE_RECORD", "request_method_id_generation_frames_map", "NECESSARY", "Allocates per-request lease record with frame map after reservation attach.", "Without record map, request/response frames cannot be separately retained/cleared.", "STAGEB_EMERGENT_V5"),
    RowSpec("internal/directionalraw/carrier.go", "Lease.Allocate", "if component == \"response-frame\" && p.frames[component] != nil", "TRANSFER_PRIOR_RESPONSE_FRAME_TO_EXTERNAL", "RESPONSE_FRAME", "unmatched_response_preserved_as_external_frame_before_next_response", "NECESSARY", "Transfers a prior response frame to external-frame slot before accepting another response-frame allocation.", "Without transfer, unmatched/external frames could be overwritten and omitted from terminal clear.", "STAGEB_EMERGENT_V5"),
    RowSpec("internal/directionalraw/carrier.go", "Lease.Allocate", "b, err := s.allocate(component, n)", "ALLOCATION_DELEGATE_TO_MANAGER", "REQUEST_RESPONSE_PARTITION", "lease_allocator_calls_manager_callback", "NECESSARY", "Delegates allocation to Manager callback; carrier never allocates retained raw independently.", "If carrier used make directly, C15 reservation/object accounting would be bypassed.", "STAGEB_EMERGENT_V5"),
    RowSpec("internal/directionalraw/carrier.go", "Lease.Allocate", "p.frames[component] = b", "OBSERVATION_RETENTION_FRAME_BACKING", "LEASE_RECORD", "frame_backing_retained_until_terminal_clear", "NECESSARY", "Retains Manager-owned backing in lease record for observation, result aliasing, and terminal clear.", "If not retained, Result/Select/View and clear(b) cannot operate.", "STAGEB_EMERGENT_V5"),
    RowSpec("internal/directionalraw/carrier.go", "Lease.Frame", "Component: component, Length: uint64(n), SHA256: sha256.Sum256(b[:n])", "OBSERVATION_RETENTION_FRAME_DIGEST", "REQUEST_RESPONSE_PARTITION", "frame_length_sha256_observation", "NECESSARY", "Builds frame observation with exact length/digest from retained backing prefix.", "If digest not from backing prefix, observation would not prove acquired/emitted bytes.", "STAGEB_EMERGENT_V5"),
    RowSpec("internal/directionalraw/carrier.go", "Lease.Result", "p.result = raw[:len(raw):len(raw)]", "ALIAS_VIEW_RESULT_NO_COPY", "RESPONSE_RESULT", "result_alias_capacity_clamped_to_response_backing", "NECESSARY", "Stores result as capacity-clamped alias into reserved response backing; no retained copy.", "If append-copied, result would double-charge response bytes.", "STAGEA_ALIAS_REALIZED"),
    RowSpec("internal/directionalraw/carrier.go", "Lease.RefusedPrivate", "Component: component + \"-reservation-refusal\", Length: uint64(n), SHA256: sum", "OBSERVATION_RETENTION_PROPOSED_REFUSAL", "REQUEST_FRAME_REFUSAL", "proposed_refusal_observation_no_allocation", "NECESSARY", "Retains proposed refusal observation without frame allocation or I/O.", "If RefusedPrivate allocated, plus-one refusal would not be pre-effect.", "STAGEB_EMERGENT_V5"),
    RowSpec("internal/directionalraw/carrier.go", "Lease.Complete", "s.latest = l.record", "OBSERVATION_RETENTION_LATEST_RECORD", "LEASE_RECORD", "latest_completed_record_for_selection", "NECESSARY", "Retains latest completed record for ordinal element selection.", "If latest not set, Select cannot bind selected elements to the response backing.", "STAGEB_EMERGENT_V5"),
    RowSpec("internal/directionalraw/carrier.go", "Lease.BindElementsPrivate", "p.elements = spans", "ALIAS_VIEW_ELEMENT_OFFSETS", "SELECTED_ELEMENT", "lexical_offsets_into_result_backing", "NECESSARY", "Stores lexical offsets into result backing, not copied element bytes.", "If elements stored copied bytes, selected handoff would become a second retained copy.", "STAGEA_ALIAS_REALIZED"),
    RowSpec("internal/directionalraw/carrier.go", "Lease.ReaderPrivate", "func (l Lease) ReaderPrivate() io.Reader { return &resultReader{record: l.record} }", "ALIAS_VIEW_IMMUTABLE_READER", "RESPONSE_RESULT", "reader_reads_result_alias_under_lease_lock", "NECESSARY", "Provides immutable stream over lease result alias for shape validation without exposing mutable backing.", "If ReaderPrivate returned raw slice, caller could retain or mutate backing outside custody.", "STAGEA_ALIAS_REALIZED"),
    RowSpec("internal/directionalraw/carrier.go", "Select", "v := View{p, span[0], span[1]}", "ALIAS_VIEW_SELECTED_ELEMENT", "SELECTED_ELEMENT", "view_offsets_into_result_backing", "NECESSARY", "Creates selected-element View as offsets into result backing.", "If View copied selected bytes, selected handoff would double-charge response bytes.", "STAGEA_ALIAS_REALIZED"),
    RowSpec("internal/directionalraw/carrier.go", "Select", "o := Observation{Method: p.method, Ordinal: q.Ordinal", "OBSERVATION_RETENTION_SELECTED_ELEMENT", "SELECTED_ELEMENT", "selected_element_digest_from_result_backing", "NECESSARY", "Retains selected element observation with ordinal, length, digest.", "If observation omitted ordinal/digest, provider ordinal and selected bytes cannot be verified.", "STAGEB_EMERGENT_V5"),
    RowSpec("internal/directionalraw/carrier.go", "View.SharesBackingPrivate", "if &raw[0] == &frame[i]", "ALIAS_VIEW_BACKING_PROOF", "SELECTED_ELEMENT", "pointer_check_private_qualification_only", "FALSE_POSITIVE", "Private helper proves aliasing for qualification; it is not production custody allocation or release behavior.", "Removing SharesBackingPrivate would not change production custody, only verifier observability.", "STAGEB_NEGATIVE_QUALIFICATION_HELPER"),
    RowSpec("internal/directionalraw/carrier.go", "View.At", "return v.record.result[v.start+i], true", "ALIAS_VIEW_BYTE_ACCESS", "SELECTED_ELEMENT", "byte_read_from_result_backing_view", "NECESSARY", "Reads selected bytes from result backing view without copying.", "If At copied and retained slices, alias proof would fail.", "STAGEA_ALIAS_REALIZED"),
    RowSpec("internal/directionalraw/carrier.go", "Reservation.Release", "return fn()", "TERMINAL_RELEASE_REQUEST_TO_GENERIC_OWNER", "RESERVATION", "reservation_release_delegates_to_manager_terminal", "NECESSARY", "Reservation release delegates to Manager-supplied generic terminal callback.", "If release bypassed Manager callback, Manager sole terminal would be false.", "STAGEA_RELEASE_REALIZED"),
    RowSpec("internal/directionalraw/carrier.go", "Reservation.Terminate", "s.closed = true", "TERMINAL_CLEAR_CLOSE_RESERVATION", "RESERVATION", "generic_terminal_closes_reservation_once", "NECESSARY", "Generic terminal is the sole close point for reservation state.", "If non-Manager code closed reservation, Manager sole terminal partition would be false.", "STAGEA_RELEASE_REALIZED"),
    RowSpec("internal/directionalraw/carrier.go", "Reservation.Terminate", "clear(b)", "TERMINAL_CLEAR_RAW_BACKING", "REQUEST_RESPONSE_PARTITION", "terminal_zeroes_all_frame_backings", "NECESSARY", "Clears every retained request/response/external frame backing at terminal.", "If clear omitted, raw frame bytes could remain after terminal release.", "STAGEA_RELEASE_REALIZED"),
    RowSpec("internal/directionalraw/carrier.go", "Reservation.Terminate", "p.result = nil", "TERMINAL_CLEAR_ALIASES", "RESPONSE_RESULT", "terminal_drops_result_alias", "NECESSARY", "Drops result alias so views cannot read after terminal.", "If result alias remained, post-terminal view could expose cleared/old backing.", "STAGEA_RELEASE_REALIZED"),
    RowSpec("internal/directionalraw/carrier.go", "Reservation.Terminate", "if s.release != nil", "TERMINAL_RELEASE_MANAGER_ACCOUNT", "C15_ACCOUNT", "terminal_invokes_manager_release_callback", "NECESSARY", "Invokes Manager release callback exactly at generic terminal.", "If callback omitted, C15 object/account release would not occur.", "STAGEA_RELEASE_REALIZED"),

    RowSpec("sessionruntime/sessionruntime.go", "(*Manager).roundTripWithPrivateTransaction", "if failure := m.admitDirectionalV5Locked(req); failure != \"\"", "MANAGER_ADMISSION_ONLY", "MANAGER_INGRESS", "admission_only_installs_directional_authority", "NECESSARY", "Makes admitDirectionalV5Locked the only authority installation point before pending key begins.", "If other path installed authority, Manager sole terminal/admission proof would fail.", "STAGEB_EMERGENT_V5"),
    RowSpec("sessionruntime/sessionruntime.go", "(*Manager).roundTripWithPrivateTransaction", "req.directionalV5.admit(key, req.Method)", "DIRECTIONAL_RESERVATION_BEGIN_AFTER_PENDING", "LEASE", "lease_begin_after_pending_key", "NECESSARY", "Begins directional lease only after pending key admission, preserving request/response identity.", "If lease began earlier, rejected requests could have retained raw frame ownership.", "STAGEB_EMERGENT_V5"),
    RowSpec("sessionruntime/sessionruntime.go", "(*Manager).roundTripWithPrivateTransaction", "bodyBytes, frame, err := lspwire.WriteDirectionalReserved", "TRANSFER_REQUEST_WRITE_TO_RESERVED_FRAME", "REQUEST_FRAME", "request_write_uses_directional_reserved_frame", "NECESSARY", "Transfers request write path into the reserved frame writer.", "If ordinary writer path ran, request frame would not be Manager-owned.", "STAGEA_PREPARE_FRAME_REALIZED"),
    RowSpec("sessionruntime/sessionruntime.go", "(*Manager).roundTripWithPrivateTransaction", "msg, frame, raw, err := reader.ReadDirectionalReserved(maxBytes, req.directionalV5.lease)", "TRANSFER_RESPONSE_READ_TO_RESERVED_FRAME", "RESPONSE_FRAME", "response_read_uses_directional_reserved_frame", "NECESSARY", "Transfers response read path into the reserved frame reader symmetrically with the reserved request writer.", "If ordinary reader path ran, response frame acquisition would not be Manager-owned or lease-bound.", "STAGEB_EMERGENT_V5_REVIEW_OMISSION_CORRECTION"),
    RowSpec("sessionruntime/sessionruntime.go", "(*Manager).roundTripWithPrivateTransaction", "return m.finishRoundTrip(req.SessionID, owner, result, session.ResourceExhausted, false)", "OBSERVATION_RETENTION_NONPOISONING_REFUSAL", "REQUEST_FRAME_REFUSAL", "resource_exhausted_refusal_without_poison", "NECESSARY", "Preserves plus-one pre-effect refusal as ResourceExhausted without poisoning Manager session.", "If poison=true, same READY Manager continuation would fail.", "STAGEB_EMERGENT_V5"),
    RowSpec("sessionruntime/sessionruntime.go", "(*Manager).roundTripWithPrivateTransaction", "if read.message.Kind() == lspwire.KindSuccessResponse && !req.directionalV5.lease.Result(read.message.Result)", "ALIAS_VIEW_RESPONSE_RESULT_BIND", "RESPONSE_RESULT", "success_response_result_must_alias_reserved_frame", "NECESSARY", "Rejects success response unless result aliases reserved response-frame backing.", "If check omitted, decoded result could come from copied RawMessage outside reserved backing.", "STAGEA_ALIAS_REALIZED"),
    RowSpec("sessionruntime/sessionruntime.go", "(*Manager).finishRoundTrip", "result.directionalV5.finish(failure)", "OBSERVATION_RETENTION_FINISH", "LEASE", "finish_records_terminal_failure_and_releases_active", "NECESSARY", "Records terminal failure and releases active directional owner accounting at round-trip finish.", "If finish omitted, terminal failure and active count evidence would be stale.", "STAGEB_EMERGENT_V5"),
    RowSpec("sessionruntime/sessionruntime.go", "(*Manager).finishRoundTrip", "genericTerminal.requestTerminal()", "TERMINAL_RELEASE_GENERIC_OWNER_SOLE_TERMINAL", "GENERIC_OWNER", "manager_generic_terminal_requested_after_publish", "NECESSARY", "Manager invokes generic owner terminal after finish; directional release callback converges there.", "If non-Manager code terminated reservation directly, Manager sole terminal would be false.", "STAGEA_RELEASE_REALIZED"),

    RowSpec("sessionruntime/b4_explicit_bytes_c15_private.go", "newPrivateB4ByteLedgerV2", "ledger.general.live = privateB4ByteLedgerV2TableBytes", "BASELINE_GENERAL_C15_TABLE_CHARGE", "C15_ACCOUNT", "inherited_general_table_backing_live_charge", "REUSE", "Inherited general C15 baseline: fixed table backing is charged once at account creation.", "If omitted, directional/general combined-limit accounting would not include table backing.", "STAGEA_EXISTING_SOURCE_REUSE"),
    RowSpec("sessionruntime/b4_explicit_bytes_c15_private.go", "newPrivateB4ByteAccountV2", "account := &privateB4ByteAccountV2{table: *table, c15: context", "BASELINE_GENERAL_C15_ACCOUNT_CREATE", "C15_ACCOUNT", "inherited_v2_account_create_with_directional_partition", "REUSE", "Inherited account owns both general table and directionalV5 partition fields.", "If directional used independent account, partition independence and Manager owner relationship would be unproven.", "STAGEA_EXISTING_SOURCE_REUSE"),
    RowSpec("sessionruntime/b4_explicit_bytes_c15_private.go", "newPrivateB4ByteAccountV2", "objectID, failure := context.create(privateC15SiteC15Owner01V2TransactionTable", "BASELINE_GENERAL_C15_BACKING_OBJECT", "C15_ACCOUNT", "transaction_table_object_created_once", "REUSE", "Inherited C15 object records the transaction table backing for release/freeze.", "If absent, release of table backing would not appear in C15 object events.", "STAGEA_EXISTING_SOURCE_REUSE"),
    RowSpec("sessionruntime/b4_explicit_bytes_c15_private.go", "(*privateB4ByteAccountV2).reservePrivateB4C15", "sites:      []privateB4C15Creation{{siteID: siteID, operation: operation}}", "BASELINE_GENERAL_C15_PLAN_COMMIT", "C15_LEASE", "reserve_plan_commits_before_lease_publication", "REUSE", "Inherited reserve plan commits object/ledger mutation atomically before lease return.", "If lease returned before commit, planned pre-effect reservation would be false.", "STAGEA_EXISTING_SOURCE_REUSE"),
    RowSpec("sessionruntime/b4_explicit_bytes_c15_private.go", "(*privateB4ByteAccountV2).aliasPrivateB4C15", "return a.c15.alias(state.objectID, siteID)", "BASELINE_GENERAL_C15_ZERO_CHARGE_ALIAS", "C15_LEASE", "zero_charge_alias_to_existing_object", "REUSE", "Inherited alias records extra site for an existing object without new allocation.", "If alias created a new object, selected-element alias would double-charge.", "STAGEA_EXISTING_SOURCE_REUSE"),
    RowSpec("sessionruntime/b4_explicit_bytes_c15_private.go", "(*privateB4ByteLeaseV2).release", "a.c15.releaseReference(state.objectID, state.siteID, terminal)", "BASELINE_GENERAL_C15_RELEASE_REFERENCE", "C15_LEASE", "exact_once_reference_release", "REUSE", "Inherited releaseReference decrements/release object references exactly once.", "If omitted, directional object release counts would not match terminal object closure.", "STAGEA_EXISTING_SOURCE_REUSE"),
    RowSpec("sessionruntime/b4_explicit_bytes_c15_private.go", "(*privateB4ByteAccountV2).requestTerminalRelease", "a.terminalRequested = true", "BASELINE_GENERAL_C15_TERMINAL_REQUEST", "C15_ACCOUNT", "terminal_closes_new_general_c15_creation", "REUSE", "Inherited terminal request closes new backing creation before maybeReleaseBackingLocked.", "If new creation remained open after terminal, terminal convergence would be false.", "STAGEA_EXISTING_SOURCE_REUSE"),
    RowSpec("sessionruntime/b4_explicit_bytes_c15_private.go", "(*privateB4ByteAccountV2).maybeReleaseBackingLocked", "a.c15.release(a.c15BackingObjectID, a.c15BackingSiteID)", "BASELINE_GENERAL_C15_BACKING_RELEASE", "C15_ACCOUNT", "transaction_table_backing_release_at_terminal", "REUSE", "Inherited release of transaction table backing after all entries and descriptor are inactive.", "If release occurred early or never, Manager terminal accounting would fail.", "STAGEA_EXISTING_SOURCE_REUSE"),

    RowSpec("internal/traverse/traverse.go", "Incoming", "directionalraw.Select(ctx, directionalraw.Request{Method: \"textDocument/prepareCallHierarchy\", Ordinal: uint64(ordinal)})", "ALIAS_VIEW_PREPARE_SELECTION", "PREPARE_SELECTION", "prepare_item_ordinal_binds_to_response_element_view", "NECESSARY", "Binds prepared item ordinal to response element view before node admission.", "If ordinal selection omitted, provider ordinal/source digest correspondence would not be checked.", "STAGEA_SELECTED_HANDOFF_REALIZED"),
    RowSpec("internal/traverse/traverse.go", "Incoming", "directionalraw.Select(ctx, directionalraw.Request{Method: \"callHierarchy/incomingCalls\", Ordinal: uint64(ordinal)})", "ALIAS_VIEW_INCOMING_SELECTION", "INCOMING_SELECTION", "incoming_call_ordinal_binds_to_response_element_view", "NECESSARY", "Binds incoming call ordinal to response element view before edge admission.", "If omitted, incoming caller row could be accepted without selected raw evidence.", "STAGEA_SELECTED_HANDOFF_REALIZED"),
    RowSpec("internal/slicer/discover.go", "Discover", "directionalraw.Select(ctx, directionalraw.Request{Method: \"textDocument/prepareCallHierarchy\", Ordinal: uint64(ordinal)})", "ALIAS_VIEW_SLICE_PREPARE_SELECTION", "PREPARE_SELECTION", "slice_prepare_ordinal_binds_to_response_element_view", "NECESSARY", "Binds slice prepare items to response element views before graph admission.", "If omitted, outgoing slice start nodes could lack selected raw custody.", "STAGEA_SELECTED_HANDOFF_REALIZED"),
    RowSpec("internal/slicer/discover.go", "Discover", "directionalraw.Select(ctx, directionalraw.Request{Method: \"callHierarchy/outgoingCalls\", Ordinal: bound.ordinal})", "ALIAS_VIEW_OUTGOING_SELECTION", "OUTGOING_SELECTION", "outgoing_call_ordinal_binds_to_response_element_view", "NECESSARY", "Binds outgoing call ordinal to response element view before edge admission and sorting.", "If omitted, outgoing callees could be reordered without preserving provider ordinal evidence.", "STAGEA_SELECTED_HANDOFF_REALIZED"),
]

CANDIDATE_PATTERNS: list[CandidatePattern] = [
    CandidatePattern("incomingops/executor.go", "dispatch", r"directionalPrivateEnabled|callDirectionalPrivate"),
    CandidatePattern("incomingops/directional_private.go", "context_dispatch_decode", r"NewReservation|Context\(parent\)|FromContext|Allowed|json\.Marshal|RoundTripDirectionalPrivate|BindDirectionalResponse|lease\.Null|lease\.Decode"),
    CandidatePattern("internal/acquisition/sessionclient/client.go", "dispatch_transfer", r"SetDirectionalPrivate|RoundTripDirectionalPrivate"),
    CandidatePattern("internal/acquisition/wire.go", "context_dispatch_decode_view", r"BindElementsPrivate|FromContext|Allowed|c\.directional|BindDirectionalResponse|lease\.Null|ReaderPrivate|lease\.Decode|json\.Marshal"),
    CandidatePattern("internal/directionalraw/carrier.go", "reservation_allocation_alias_observation_clear_release", r"NewReservation|FromContext|Allowed|Begin\(|Attach\(|Allocate\(|make\(map\[string\]\[\]byte\)|s\.allocate|p\.frames\[component\]|Observation\{|Result\(|raw\[:len\(raw\):len\(raw\)\]|RefusedPrivate|Complete\(|s\.latest = l\.record|BindElementsPrivate|p\.elements|ReaderPrivate|Select\(|View\{|SharesBackingPrivate|&raw\[0\] == &frame\[i\]|At\(|return v\.record\.result|Release\(|return fn\(\)|Terminate\(|s\.closed = true|clear\(|p\.result = nil|s\.release"),
    CandidatePattern("internal/lspwire/read_frame.go", "allocation_copy_transfer_observation_alias", r"Allowed\(method\)|lease\.Allocate|copy\(|ReadFull|lease\.Frame|Fields\(|msg\.Result|RefusedPrivate"),
    CandidatePattern("internal/slicer/discover.go", "selection_context_view", r"FromContext|Select\(|View\(\)"),
    CandidatePattern("internal/traverse/traverse.go", "selection_context_view", r"FromContext|Select\(|View\(\)"),
    CandidatePattern("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "manager_admission_allocation_observation_release", r"FromContext|IdentityPrivate|Allowed|EnablePrivateC16ObjectAccounting|EnablePrivateC17EventAccounting|directionalV5|Notifications = nil|Responses = nil|make\(map|newPrivateB4C15CustodyContext|newPrivateB4ByteAccountV2|Attach\(|active\+\+|component !=|terminalRequested|c15\.create|frameObjects|p\.live|allocateBackingLocked|allocationBoundaries|make\(\[\]byte|responseObjects|c15\.alias|observations = append|releaseAccount|requestTerminalRelease|Begin\(|Complete\(|admissionClosed|Release\(\)"),
    CandidatePattern("sessionruntime/sessionruntime.go", "manager_dispatch_transfer_alias_release", r"admitDirectionalV5Locked|directionalV5\.admit|WriteDirectionalReserved|ReadDirectionalReserved|finishRoundTrip|lease\.Result|directionalV5\.finish|genericTerminal\.requestTerminal"),
    CandidatePattern("sessionruntime/b4_explicit_bytes_c15_private.go", "general_c15_reuse", r"ledger\.general\.live|privateB4ByteAccountV2\{|context\.create|sites:|c15\.alias|releaseReference|terminalRequested = true|c15\.release"),
]

CANDIDATE_EXCLUSIONS: list[CandidateExclusion] = [
    CandidateExclusion("incomingops/executor.go", "if directionalPrivateEnabled(parent, method) {", "dispatch", "Control-flow half of the following rowed callDirectionalPrivate dispatch; the actual transfer row is the return call and deletion of either line is detected by source-candidate drift."),
    CandidateExclusion("incomingops/directional_private.go", "RoundTripDirectionalPrivate(context.Context, sessionruntime.RoundTripRequest) (sessionruntime.RoundTripResult, directionalraw.Lease)", "dispatch", "Interface method signature declares capability but performs no dispatch, allocation, or decode; concrete calls are rowed separately."),
    CandidateExclusion("incomingops/directional_private.go", "_, ok := directionalraw.FromContext(ctx)", "context", "Helper-local observation half of the rowed return ok && Allowed gate; the return row is the falsifiable gate surface."),
    CandidateExclusion("incomingops/directional_private.go", "if lease.Null() {", "decode", "Null response branch has no result alias decode or retained bytes; falsifier is a non-null result requiring lease.Decode target binding."),
    CandidateExclusion("internal/acquisition/sessionclient/client.go", "RoundTripDirectionalPrivate(context.Context, sessionruntime.RoundTripRequest) (sessionruntime.RoundTripResult, directionalraw.Lease)", "dispatch", "Interface method signature declares capability but performs no dispatch, allocation, or decode; concrete callback call is rowed separately."),
    CandidateExclusion("internal/acquisition/wire.go", "raw, err := json.Marshal(params)", "copy", "Neutral request marshal is an ephemeral parameter encoding before any directional response custody; retained raw request custody is rowed at WriteDirectionalReserved."),
    CandidateExclusion("internal/acquisition/wire.go", "c.directional = fn", "dispatch", "Setter assignment installs the rowed callback capability but performs no request transfer; SetDirectionalPrivate is rowed in sessionclient New."),
    CandidateExclusion("internal/acquisition/wire.go", "func BindDirectionalResponse(lease directionalraw.Lease) error {", "alias", "Function declaration is not binding; the return lease.BindElementsPrivate row is the binding event."),
    CandidateExclusion("internal/acquisition/wire.go", "if e = BindDirectionalResponse(lease); e != nil {", "alias", "Call-site guard invokes the rowed neutral binding function; it creates no separate alias/copy beyond BindElementsPrivate."),
    CandidateExclusion("internal/acquisition/wire.go", "if c.directional == nil {", "dispatch", "Availability guard only; it allocates no custody and transfers no lease. Falsifier would be a c.directional call not rowed below."),
    CandidateExclusion("internal/acquisition/wire.go", "lease, e := c.directional(ctx, WireRequest{Context: scope.context, Method: method, Params: raw, MaxBytes: scope.limits.MaxResponseBytes, MaxMessages: scope.limits.MaxMessages})", "transfer", "Call-site transfer is represented by the sessionclient runtime lease-transfer row; this local line returns the same lease and is bounded by BindDirectionalResponse/Decode rows."),
    CandidateExclusion("internal/acquisition/wire.go", "if lease.Null() {", "decode", "Null response branch has no result alias decode or retained bytes; falsifier is a non-null response requiring ReaderPrivate/Decode rows."),
    CandidateExclusion("internal/acquisition/wire.go", "if e = validateWireShapeReaders(method, lease.ReaderPrivate); e != nil {", "view", "Shape validation reads through the rowed ReaderPrivate immutable alias; it creates no retained copy and is bounded by the ReaderPrivate row."),
    CandidateExclusion("internal/directionalraw/carrier.go", "func NewReservation(transactionID ...string) Reservation {", "reservation", "Constructor declaration is not a reservation event; the rowed creation call is incomingops.NewReservation."),
    CandidateExclusion("internal/directionalraw/carrier.go", "func (r Reservation) Attach(allocate func(string, int) ([]byte, error), observe func(Observation), terminal func() bool, release func()) bool {", "reservation", "Method declaration is not callback installation; the assignment of callbacks is rowed."),
    CandidateExclusion("internal/directionalraw/carrier.go", "if s.allocate != nil {", "reservation", "Attach duplicate-attach guard allocates no raw bytes and installs no callback; falsifier would be overwritten callbacks, which this guard prevents."),
    CandidateExclusion("internal/directionalraw/carrier.go", "func (r Reservation) Begin(method string, id, generation uint64) Lease {", "reservation", "Method declaration is not a lease record allocation; the record map allocation row is inside the method."),
    CandidateExclusion("internal/directionalraw/carrier.go", "if s.closed || s.allocate == nil {", "reservation", "Begin precondition guard allocates no raw backing and is bounded by the rowed lease record allocation."),
    CandidateExclusion("internal/directionalraw/carrier.go", "p.frames[\"external-frame-\"+fmt.Sprint(len(p.frames))] = p.frames[component]", "transfer", "External-frame preservation is preparatory to the rowed prior-response transfer candidate; it stores an existing backing rather than allocating or copying a new retained response."),
    CandidateExclusion("internal/directionalraw/carrier.go", "if s.closed || p.complete || p.frames[component] != nil || n <= 0 {", "allocation", "Allocate precondition guard creates no backing; falsifier is the rowed Manager allocation delegate that must remain after the guard."),
    CandidateExclusion("internal/directionalraw/carrier.go", "b := p.frames[component]", "observation", "Frame reads an existing backing for digest observation; the digest Observation construction is rowed."),
    CandidateExclusion("internal/directionalraw/carrier.go", "if s.closed || p == nil || p.method != q.Method || !p.bound || q.Ordinal >= uint64(len(p.elements)) {", "view", "Select precondition guard creates no view; the View construction and selected Observation are rowed."),
    CandidateExclusion("internal/directionalraw/carrier.go", "span := p.elements[q.Ordinal]", "view", "Span lookup reads previously rowed lexical offsets and creates no new alias until the rowed View construction."),
    CandidateExclusion("internal/directionalraw/carrier.go", "func (v View) SharesBackingPrivate() bool {", "view", "Method declaration is not alias proof behavior; the pointer comparison row is classified FALSE_POSITIVE."),
    CandidateExclusion("internal/directionalraw/carrier.go", "func (v View) At(i int) (byte, bool) {", "view", "Method declaration is not byte access; the return from result backing row is the view behavior."),
    CandidateExclusion("internal/directionalraw/carrier.go", "p.elements = nil", "clear", "Terminal drops element-offset metadata; the rowed result alias drop and raw backing clear are the retained-byte custody clear events."),
    CandidateExclusion("internal/directionalraw/carrier.go", "s.release()", "release", "Terminal invokes release callback through the rowed if s.release != nil branch; this line is the callback body, not a separate release authority."),
    CandidateExclusion("internal/directionalraw/carrier.go", "func FromContext(ctx context.Context) (Reservation, bool) {", "context", "Function declaration is not a context observation event; call sites are rowed or excluded independently."),
    CandidateExclusion("internal/directionalraw/carrier.go", "func Allowed(method string) bool {", "context", "Function declaration is not a method gate observation; call sites are rowed or excluded independently."),
    CandidateExclusion("internal/directionalraw/carrier.go", "if !Allowed(method) || r.state == nil {", "reservation", "Begin precondition guard allocates no raw backing; falsifier would be a Begin record allocation without this allowed-method precheck."),
    CandidateExclusion("internal/directionalraw/carrier.go", "func (l Lease) Allocate(component string, n int) ([]byte, error) {", "allocation", "Method declaration is not an allocation event; delegation to Manager and frame retention are rowed inside the method."),
    CandidateExclusion("internal/directionalraw/carrier.go", "func (l Lease) Result(raw []byte) bool {", "alias", "Method declaration is not an alias event; the capacity-clamped result assignment is rowed."),
    CandidateExclusion("internal/directionalraw/carrier.go", "func (l Lease) RefusedPrivate(component string, n int, sum [32]byte) {", "observation", "Method declaration is not an observation event; the refusal Observation construction is rowed."),
    CandidateExclusion("internal/directionalraw/carrier.go", "func (l Lease) Complete(failure string) {", "observation", "Method declaration is not completion retention; latest-record assignment is rowed."),
    CandidateExclusion("internal/directionalraw/carrier.go", "func (l Lease) BindElementsPrivate() error {", "alias", "Method declaration is not the element-offset retention event; p.elements assignment is rowed."),
    CandidateExclusion("internal/directionalraw/carrier.go", "n := copy(dst, r.record.result[r.position:])", "copy", "Ephemeral io.Reader copy to caller buffer is not retained raw custody; falsifier would be storing dst as retained backing, which this line does not do."),
    CandidateExclusion("internal/directionalraw/carrier.go", "func Select(ctx context.Context, q Request) (PreparedSelection, bool) {", "view", "Function declaration is not a selected view creation; View construction and selected Observation are rowed."),
    CandidateExclusion("internal/directionalraw/carrier.go", "r, ok := FromContext(ctx)", "context", "Select context lookup is a precondition for the rowed selected-element view/observation, not a separate custody transfer."),
    CandidateExclusion("internal/directionalraw/carrier.go", "func (r Reservation) Release() bool {", "release", "Method declaration is not the terminal delegation event; return fn() is rowed."),
    CandidateExclusion("internal/directionalraw/carrier.go", "func (r Reservation) Terminate() bool {", "release", "Method declaration is not terminal close/clear/release; concrete close, clear, alias drop, and callback rows are rowed."),
    CandidateExclusion("internal/directionalraw/carrier.go", "func Fields(raw []byte) (map[string][]byte, error) {", "view", "Parser declaration is not an alias event; call sites using response backing fields are rowed."),
    CandidateExclusion("internal/directionalraw/carrier.go", "out := make(map[string][]byte)", "view", "Parser metadata map is not retained response raw custody; values remain slices of the input backing."),
    CandidateExclusion("internal/lspwire/read_frame.go", "type ReadFrameObservation struct {", "observation", "Type declaration is not an observation event; concrete ReadFrameObservation or lease.Frame rows are separately rowed/excluded."),
    CandidateExclusion("internal/lspwire/read_frame.go", "func (r *Reader) ReadWithExactFrame(maxBytes int64) (Message, ReadFrameObservation, []byte, error) {", "observation", "Ordinary frame reader declaration is outside directional reservation custody; directional read paths are rowed separately."),
    CandidateExclusion("internal/lspwire/read_frame.go", "if !json.Valid(params) || !directionalraw.Allowed(method) {", "context", "Request validation guard allocates no retained backing; falsifier is the rowed request-frame Allocate that must remain after validation."),
    CandidateExclusion("internal/lspwire/read_frame.go", "lease.Frame(\"response-frame\", used)", "observation", "Partial-header response observation is bounded by the rowed partial response-frame allocation/copy; no additional retained backing or alias is introduced."),
    CandidateExclusion("internal/lspwire/read_frame.go", "copy(sum[:], h.Sum(nil))", "copy", "Digest-byte copy is not request/response raw custody; falsifier would be retaining frame bytes here, which does not occur."),
    CandidateExclusion("internal/lspwire/read_frame.go", "at += copy(frame[at:], prefix)", "copy", "JSON-RPC prefix copy is part of the rowed reserved request-frame backing population with header/params copies; it is not a separate retained backing."),
    CandidateExclusion("internal/lspwire/read_frame.go", "o := ReadFrameObservation{FrameBytes: int64(len(raw)), FrameSHA256: fmt.Sprintf(\"sha256:%x\", sum)}", "observation", "ReadFrameObservation is transport metadata; the retained response-frame observation is rowed through lease.Frame."),
    CandidateExclusion("internal/lspwire/read_frame.go", "ef, er := directionalraw.Fields(v)", "view", "Error-field parse is a bounded validation view over error JSON, not success result retained custody; success result alias row remains msg.Result = v."),
    CandidateExclusion("sessionruntime/b4_explicit_bytes_c15_private.go", "h.ledger.general.live -= h.ledger.descriptor.capacity", "release", "General descriptor release accounting is inherited baseline cleanup, not directional request/response raw custody; terminal account release rows cover directional convergence."),
    CandidateExclusion("sessionruntime/b4_explicit_bytes_c15_private.go", "plan := &privateB4C15ByteAccountPlan{account: a, capacities: capacities, sites: sites, dst: dst}", "allocation", "Inherited plan object is pre-existing C15 lease machinery; the reserve plan commit row covers its relevance without promoting it to a directional actual row."),
    CandidateExclusion("sessionruntime/b4_explicit_bytes_c15_private.go", "if observed && h.account.c15.alias(state.objectID, state.siteID) != \"\" {", "alias", "Inherited observation-time alias guard is general C15 lease behavior; directional selected-element zero-charge alias row is separately rowed."),
    CandidateExclusion("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "allocationBoundaries []privateDirectionalAllocationV5", "observation", "Struct field declaration is not an observation event; append of allocation boundary is rowed."),
    CandidateExclusion("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "frameObjects         map[uint64]string", "observation", "Struct field declaration is not retention behavior; map allocation and frame object write are rowed."),
    CandidateExclusion("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "responseObjects      map[uint64]uint64", "observation", "Struct field declaration is not retention behavior; map allocation and response-object write are rowed."),
    CandidateExclusion("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "r, ok := directionalraw.FromContext(ctx)", "context", "Local context lookup is the input half of the rowed compound identity/method/accounting gate; the actual row is the following falsifiable if condition."),
    CandidateExclusion("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "result := m.roundTripWithPrivateTransaction(ctx, req, nil, &privateB4Transaction{directionalV5: req.directionalV5})", "transfer", "Private transaction call is Manager ingress plumbing; admission, request write, alias bind, finish, and terminal rows carry the custody events."),
    CandidateExclusion("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "return result, req.directionalV5.lease", "transfer", "Return bundles the already rowed lease transfer to caller; it creates no new allocation or alias beyond lease rows."),
    CandidateExclusion("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "call := req.directionalV5", "dispatch", "Local variable alias to request directional state creates no custody; allocation/admission rows are rowed separately."),
    CandidateExclusion("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "d := m.directionalV5", "dispatch", "Manager state local alias creates no custody; reservation-scoped state allocation/admission rows are rowed."),
    CandidateExclusion("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "o.directionalV5 = d", "dispatch", "Owner backpointer assignment wires lifecycle state after rowed Attach; it does not allocate, copy, decode, or release raw custody."),
    CandidateExclusion("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "m.directionalV5 = d", "dispatch", "Manager field assignment publishes already-rowed directional state; it creates no separate custody event."),
    CandidateExclusion("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "if d.owner.admissionClosed || d.owner.released {", "dispatch", "Admission guard prevents closed/released owner use and creates no custody; active count row captures admission."),
    CandidateExclusion("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "p := &a.directionalV5", "allocation", "Local partition pointer creates no allocation; subsequent precheck/object/charge/make rows are rowed."),
    CandidateExclusion("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "func (d *privateDirectionalV5) allocateBackingLocked(component string, n int) []byte {", "allocation", "Method declaration is not allocation; allocation boundary append and make rows are rowed."),
    CandidateExclusion("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "p := &d.account.directionalV5", "allocation", "Local partition pointer creates no allocation; boundary snapshot and make rows are rowed."),
    CandidateExclusion("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "Component: component, Length: n, Live: p.live, Cumulative: p.cumulative,", "observation", "Allocation-boundary struct field line is part of the rowed append expression, not a separate observation."),
    CandidateExclusion("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "Allocations: p.allocations, OwnershipObjects: len(d.frameObjects), Occupied: p.occupied,", "observation", "Allocation-boundary struct field line is part of the rowed append expression, not a separate observation."),
    CandidateExclusion("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "func (d *privateDirectionalV5) releaseAccount() {", "release", "Method declaration is not release; frame-object release and account terminal request rows are rowed."),
    CandidateExclusion("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "p.live = 0", "release", "Terminal accounting reset follows rowed frame-object releases and account terminal request; it releases no additional raw backing."),
    CandidateExclusion("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "func (m *Manager) DirectionalGenericTerminalPrivate() (active uint32, admissionClosed, releaseStarted, released bool) {", "observation", "Private diagnostic accessor reports generic terminal state; it is not production custody mutation."),
    CandidateExclusion("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "return o.active, o.admissionClosed, o.releaseStarted, o.released", "observation", "Private diagnostic state return creates no allocation/copy/release; terminal mutation rows are rowed separately."),
    CandidateExclusion("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "return append([]privateDirectionalAllocationV5(nil), d.allocationBoundaries...)", "copy", "Private diagnostic copy of allocation-boundary metadata is not retained raw request/response custody."),
    CandidateExclusion("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "p := a.directionalV5", "observation", "Private diagnostic local partition copy creates no custody event."),
    CandidateExclusion("sessionruntime/call_hierarchy_directional_raw_v5_private.go", "return p.live, p.cumulative, p.allocations, p.releases, a.table.general.live, p.occupied", "observation", "Private diagnostic counter return creates no allocation/copy/release; counted mutations are rowed separately."),
    CandidateExclusion("sessionruntime/sessionruntime.go", "return m.finishRoundTrip(req.SessionID, &ownedTransport{child: child}, result, session.SessionPoisoned, true)", "release", "Ordinary poisoned finish path is non-directional failure handling before directional reserved write; directional nonpoisoning refusal and terminal finish rows are rowed separately."),
    CandidateExclusion("sessionruntime/sessionruntime.go", "return m.finishRoundTrip(req.SessionID, owner, result, session.ResourceExhausted, true)", "release", "Ordinary poisoning resource-exhaustion finish path is not the directional plus-one nonpoisoning refusal; the directional false-poison row is separately rowed."),
    CandidateExclusion("sessionruntime/sessionruntime.go", "return m.finishRoundTrip(req.SessionID, owner, result, failure, true)", "release", "Ordinary poisoning failure finish paths are not directional terminal evidence; directional finish/terminal rows are separately rowed."),
    CandidateExclusion("sessionruntime/sessionruntime.go", "return m.finishRoundTrip(req.SessionID, owner, result, session.SessionPoisoned, true)", "release", "Ordinary poisoning session-failure finish paths are not directional terminal evidence; directional finish/terminal rows are separately rowed."),
    CandidateExclusion("sessionruntime/sessionruntime.go", "return m.finishRoundTrip(req.SessionID, owner, result, failure, poison)", "release", "Generic failure finish parameterization is not directional terminal proof; directional finish/terminal rows are separately rowed."),
    CandidateExclusion("sessionruntime/sessionruntime.go", "return m.finishRoundTrip(req.SessionID, owner, result, \"\", false)", "release", "Successful ordinary finish return is not directional terminal proof; directional finish/terminal rows are separately rowed."),
    CandidateExclusion("sessionruntime/sessionruntime.go", "func (m *Manager) finishRoundTrip(id string, owner *ownedTransport, result RoundTripResult, failure session.Failure, poison bool) RoundTripResult {", "release", "Method declaration is not a terminal event; directional finish and generic terminal request rows inside it are rowed."),
    CandidateExclusion("internal/slicer/discover.go", "if _, enabled := directionalraw.FromContext(ctx); enabled {", "context", "Selection precondition only; the actual prepare/outgoing Select rows are rowed."),
    CandidateExclusion("internal/traverse/traverse.go", "if _, enabled := directionalraw.FromContext(ctx); enabled {", "context", "Selection precondition only; the actual prepare/incoming Select rows are rowed."),
]


def sha256_bytes(b: bytes) -> str:
    return hashlib.sha256(b).hexdigest()

def sha256_text(s: str) -> str:
    return sha256_bytes(s.encode("utf-8"))

def read_tsv(path: Path) -> list[dict[str, str]]:
    with path.open(newline="", encoding="utf-8") as f:
        return list(csv.DictReader(f, delimiter="\t"))

def write_tsv(path: Path, rows: list[dict[str, object]], fields: list[str]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("w", newline="", encoding="utf-8") as f:
        w = csv.DictWriter(f, fieldnames=fields, delimiter="\t", lineterminator="\n", extrasaction="ignore")
        w.writeheader()
        for row in rows:
            w.writerow({k: row.get(k, "") for k in fields})

def source_hash(root: Path, rel: str) -> str:
    return sha256_bytes((root / rel).read_bytes())

def find_line(root: Path, spec: RowSpec) -> tuple[int, str, str, str]:
    p = root / spec.path
    text = p.read_text(encoding="utf-8")
    matches = []
    for i, line in enumerate(text.splitlines(), start=1):
        if spec.snippet in line:
            matches.append((i, line.rstrip("\n")))
    if len(matches) != 1:
        raise SystemExit(f"snippet match count {len(matches)} for {spec.path}::{spec.function}: {spec.snippet!r}")
    line_no, line = matches[0]
    return line_no, source_hash(root, spec.path), sha256_text(line), line.strip()

def actual_id(spec: RowSpec, line_hash: str) -> str:
    preimage = "\0".join([ROW_NS, spec.path, spec.function, spec.operation_kind, spec.component, spec.backing_identity_rule, line_hash])
    return sha256_text(preimage)

def build_actual_rows(root: Path) -> list[ActualRow]:
    rows = []
    for spec in SPECS:
        line, fhash, lhash, excerpt = find_line(root, spec)
        rid = actual_id(spec, lhash)
        rows.append(ActualRow(rid, spec.path, line, fhash, lhash, spec.function, spec.operation_kind, spec.component, spec.backing_identity_rule, excerpt, spec.classification, spec.rationale, spec.contrary_evidence_or_falsifier, spec.predecessor_hint, spec.authority_ceiling))
    rows.sort(key=lambda r: (r.source_path, r.source_line, r.operation_kind, r.actual_row_id))
    return rows

def candidate_id(path: str, line: int, family: str, line_hash: str) -> str:
    return sha256_text("\0".join([ROW_NS, "candidate", path, str(line), family, line_hash]))

def discover_source_candidates(root: Path, actual_rows: list[ActualRow]) -> tuple[list[SourceCandidate], list[SourceCandidate], list[SourceCandidate]]:
    actual_by_key = {(r.source_path, r.source_line, r.operation_excerpt): r.actual_row_id for r in actual_rows}
    exclusion_by_key: dict[tuple[str, int, str], str] = {}
    exclusion_rows: list[SourceCandidate] = []
    for ex in CANDIDATE_EXCLUSIONS:
        for line, _, lhash, excerpt in find_exclusion_lines(root, ex):
            key = (ex.path, line, excerpt)
            exclusion_by_key[key] = ex.falsifier
            exclusion_rows.append(SourceCandidate(candidate_id(ex.path, line, ex.family, lhash), ex.path, line, lhash, ex.family, excerpt, "EXCLUDED", "", ex.falsifier))
    seen: dict[tuple[str, int, str, str], SourceCandidate] = {}
    for cp in CANDIDATE_PATTERNS:
        text = (root / cp.path).read_text(encoding="utf-8")
        rx = re.compile(cp.pattern)
        for i, line in enumerate(text.splitlines(), start=1):
            stripped = line.strip()
            if not stripped or stripped.startswith("//"):
                continue
            if not rx.search(stripped):
                continue
            lhash = sha256_text(stripped)
            key = (cp.path, i, stripped)
            actual_row_id = actual_by_key.get(key, "")
            falsifier = exclusion_by_key.get(key, "")
            disposition = "ROWED" if actual_row_id else "EXCLUDED" if falsifier else "UNCLASSIFIED"
            c = SourceCandidate(candidate_id(cp.path, i, cp.family, lhash), cp.path, i, lhash, cp.family, stripped, disposition, actual_row_id, falsifier)
            seen[(c.source_path, c.source_line, c.family, c.operation_excerpt)] = c
    candidates = sorted(seen.values(), key=lambda c: (c.source_path, c.source_line, c.family, c.operation_excerpt))
    unclassified = [c for c in candidates if c.disposition == "UNCLASSIFIED"]
    return candidates, exclusion_rows, unclassified

def find_exclusion_lines(root: Path, ex: CandidateExclusion) -> list[tuple[int, str, str, str]]:
    p = root / ex.path
    text = p.read_text(encoding="utf-8")
    matches = []
    for i, line in enumerate(text.splitlines(), start=1):
        if ex.snippet in line.strip():
            matches.append((i, line.rstrip("\n")))
    if not matches:
        raise SystemExit(f"candidate exclusion match count 0 for {ex.path}: {ex.snippet!r}")
    fhash = source_hash(root, ex.path)
    return [(line_no, fhash, sha256_text(line.strip()), line.strip()) for line_no, line in matches]

def assert_source_candidate_coverage(actual_rows: list[ActualRow], candidates: list[SourceCandidate], unclassified: list[SourceCandidate]) -> None:
    if unclassified:
        preview = "; ".join(f"{c.source_path}:{c.source_line}:{c.family}:{c.operation_excerpt}" for c in unclassified[:10])
        raise SystemExit(f"unclassified source candidates: {preview}")
    invalid = [c for c in candidates if c.disposition not in {"ROWED", "EXCLUDED"}]
    if invalid:
        preview = "; ".join(f"{c.source_path}:{c.source_line}:{c.disposition}:{c.operation_excerpt}" for c in invalid[:10])
        raise SystemExit(f"source candidates not rowed or excluded: {preview}")
    rowed = [c for c in candidates if c.disposition == "ROWED"]
    if len(rowed) != len({c.actual_row_id for c in rowed}):
        raise SystemExit("candidate discovery rowed the same actual row more than once")
    actual_ids = {r.actual_row_id for r in actual_rows}
    rowed_ids = {c.actual_row_id for c in rowed}
    missing = sorted(actual_ids - rowed_ids)
    if missing:
        raise SystemExit(f"actual source rows without rowed source candidates: {', '.join(missing[:10])}")
    extra = sorted(rowed_ids - actual_ids)
    if extra:
        raise SystemExit(f"source candidates rowed unknown actual rows: {', '.join(extra[:10])}")
    excluded_without_falsifier = [c for c in candidates if c.disposition == "EXCLUDED" and not c.exclusion_falsifier]
    if excluded_without_falsifier:
        preview = "; ".join(f"{c.source_path}:{c.source_line}:{c.family}:{c.operation_excerpt}" for c in excluded_without_falsifier[:10])
        raise SystemExit(f"excluded source candidates missing falsifier: {preview}")

def assert_required_invariants(actual_rows: list[ActualRow]) -> None:
    by_op = {r.operation_kind: r for r in actual_rows}
    required_ops = {
        "TRANSFER_REQUEST_WRITE_TO_RESERVED_FRAME": "request transfer into reserved frame",
        "TRANSFER_RESPONSE_READ_TO_RESERVED_FRAME": "response transfer into reserved frame",
        "OBSERVATION_RETENTION_NEGATIVE_NO_PUBLIC_NOTIFICATIONS": "notification public raw clear",
        "OBSERVATION_RETENTION_NEGATIVE_NO_PUBLIC_RESPONSES": "response public raw clear",
        "DIRECTIONAL_MANAGER_COMPOUND_GATE": "Manager identity/method/accounting gate",
    }
    missing_ops = [op for op in required_ops if op not in by_op]
    if missing_ops:
        raise SystemExit("missing required directional invariant rows: " + ", ".join(f"{op} ({required_ops[op]})" for op in missing_ops))
    gate = by_op["DIRECTIONAL_MANAGER_COMPOUND_GATE"].operation_excerpt
    for token in ["IdentityPrivate", "Allowed", "EnablePrivateC16ObjectAccounting", "EnablePrivateC17EventAccounting"]:
        if token not in gate:
            raise SystemExit(f"compound Manager gate row missing {token}")

def run_self_tests(actual_rows: list[ActualRow], candidates: list[SourceCandidate], unclassified: list[SourceCandidate]) -> dict[str, bool]:
    expected = [{k: str(v) for k, v in asdict(r).items()} for r in actual_rows]
    def detects(label: str, fn) -> bool:
        try:
            fn()
        except SystemExit:
            return True
        raise SystemExit(f"self-test did not detect {label}")
    def compare_rows(got: list[dict[str, str]]) -> None:
        ids = [r["actual_row_id"] for r in got]
        if len(ids) != len(set(ids)):
            raise SystemExit("duplicate actual row id")
        if got != expected:
            raise SystemExit("ACTUAL_SOURCE_ROWS.tsv does not match current source enumeration")
    first_actual_id = actual_rows[0].actual_row_id
    missing_candidate_set = [c for c in candidates if c.actual_row_id != first_actual_id]
    results = {
        "detects_deleted_row": detects("deleted row", lambda: compare_rows(expected[1:])),
        "detects_duplicated_row": detects("duplicated row", lambda: compare_rows(expected + [expected[0]])),
        "detects_reclassification": detects("reclassification", lambda: compare_rows([{**expected[0], "classification": "REUSE"}] + expected[1:])),
        "detects_source_drift": detects("source drift", lambda: compare_rows([{**expected[0], "source_line_sha256": "sha256:drift"}] + expected[1:])),
        "detects_missing_required_invariant": detects("missing required invariant", lambda: assert_required_invariants([r for r in actual_rows if r.operation_kind != "TRANSFER_RESPONSE_READ_TO_RESERVED_FRAME"])),
        "detects_missing_actual_candidate": detects("missing actual candidate", lambda: assert_source_candidate_coverage(actual_rows, missing_candidate_set, [])),
        "detects_extra_unclassified_candidate": detects("extra unclassified candidate", lambda: assert_source_candidate_coverage(actual_rows, candidates + [SourceCandidate("tamper", "tamper.go", 1, "tamper", "dispatch", "tamper()", "UNCLASSIFIED", "", "")], [SourceCandidate("tamper", "tamper.go", 1, "tamper", "dispatch", "tamper()", "UNCLASSIFIED", "", "")]))
    }
    return results

def git(root: Path, *args: str) -> str:
    return subprocess.check_output(["git", "-C", str(root), *args], text=True).strip()

def stagea_reconciliation(root: Path, actual_rows: list[ActualRow]) -> list[dict[str, str]]:
    plans = read_tsv(root / STAGEA)
    by_component = {}
    for r in actual_rows:
        by_component.setdefault(r.component, []).append(r.actual_row_id)
    def successors(status: str, component: str) -> tuple[str, list[str], str]:
        if status == "PLANNED_RESERVATION":
            if component == "PREPARE_FRAME":
                return "MERGED_REALIZED_BY_REQUEST_FRAME", by_component.get("REQUEST_FRAME", []) + by_component.get("REQUEST_RESPONSE_PARTITION", []), "V5 uses request-frame partition rather than StageA prepare-frame symbol."
            return "MERGED_REALIZED_BY_RESPONSE_FRAME", by_component.get("RESPONSE_FRAME", []) + by_component.get("RESPONSE_FRAME_PARTIAL", []) + by_component.get("REQUEST_RESPONSE_PARTITION", []), "V5 uses response-frame partition and element views rather than separate prepare/result/selected/directional/external components."
        if status == "PLANNED_COPY":
            if component == "PREPARE_FRAME":
                return "MERGED_REALIZED_BY_REQUEST_RESERVED_COPY", by_component.get("REQUEST_FRAME", []), "Copy is into the already reserved request-frame backing, not a separate retained copy."
            return "MERGED_REALIZED_BY_RESPONSE_RESERVED_COPY", by_component.get("RESPONSE_FRAME", []) + by_component.get("RESPONSE_FRAME_PARTIAL", []), "Copy/read is into the reserved response-frame backing; result/element bytes are aliases."
        if status == "PLANNED_ALIAS":
            return "MERGED_REALIZED_BY_ALIAS_VIEW", by_component.get("RESPONSE_RESULT", []) + by_component.get("SELECTED_ELEMENT", []) + by_component.get("PREPARE_SELECTION", []) + by_component.get("INCOMING_SELECTION", []) + by_component.get("OUTGOING_SELECTION", []), "StageA alias obligations are realized as result/element views and C15 zero-charge aliases."
        if status == "PLANNED_TRANSFER":
            return "SPLIT_REALIZED_BY_LEASE_AND_RESPONSE_TRANSFER", by_component.get("LEASE", []) + by_component.get("RESPONSE_FRAME", []) + by_component.get("RESPONSE_FRAME_PARTIAL", []), "Whole-acquisition transfer split across returned lease, response frame transfer, and direct read into backing."
        if status == "PLANNED_RELEASE":
            return "MERGED_REALIZED_BY_GENERIC_MANAGER_TERMINAL", by_component.get("GENERIC_OWNER", []) + by_component.get("RESERVATION", []) + by_component.get("C15_ACCOUNT", []) + by_component.get("C15_DIRECTIONAL_OBJECT", []), "Release converges through Manager/generic terminal and inherited C15 account release."
        return "UNRESOLVED", [], "unknown StageA status"
    out = []
    seen = set()
    for p in plans:
        event, succ, why = successors(p["status"], p["component"])
        succ = sorted(set(succ))
        lid = sha256_text("\0".join([LINEAGE_NS, "stagea", p["stage_a_row_id"], event, "|".join(succ)]))
        out.append({
            "lineage_record_id": lid,
            "stage_a_row_id": p["stage_a_row_id"],
            "stage_a_status": p["status"],
            "stage_a_component": p["component"],
            "lineage_event": event,
            "actual_successor_ids": "|".join(succ),
            "planned_row_promoted": "false",
            "justification": why,
            "authority_ceiling": "AUTHORITY_0_PROVISIONAL_LINEAGE_ONLY",
        })
        seen.add(p["stage_a_row_id"])
    if len(out) != 20 or len(seen) != 20:
        raise SystemExit("StageA reconciliation did not cover exactly 20 unique planned IDs")
    return out

def model_reconciliation(root: Path, actual_rows: list[ActualRow]) -> list[dict[str, str]]:
    identities = read_tsv(root / IDENTITY)
    by_class = {}
    for r in actual_rows:
        by_class.setdefault(r.classification, []).append(r.actual_row_id)
    baseline = [r.actual_row_id for r in actual_rows if r.classification == "REUSE"]
    necessary = [r.actual_row_id for r in actual_rows if r.classification == "NECESSARY"]
    false_positive = [r.actual_row_id for r in actual_rows if r.classification == "FALSE_POSITIVE"]
    out = []
    for ident in identities:
        op = ident.get("operation_kind", "")
        path = ident.get("source_or_proposed_path", "")
        symbol = ident.get("symbol", "")
        component = ident.get("component", "")
        if "proposed." in symbol or "call_hierarchy_directional_raw_v1_private.go" in path or ident["identity_kind"] == "NEW_PLANNED_ARCHITECTURE_OBLIGATION":
            event = "SUPERSEDED_BY_STAGEB_V5_ACTUAL_SOURCE"
            succ = necessary[:12]
            note = "Earlier proposed/planned identity is preserved as predecessor lineage only; v5 actual IDs derive from current source path/function/line/operation/hash."
        elif op in {"BORROW"}:
            event = "NEGATIVE_BASELINE_NOT_REUSABLE_OR_VIEW_ONLY"
            succ = false_positive
            note = "Earlier borrow rows are not silently reused as retained allocations; StageB separates immutable views/qualification helpers from allocation rows."
        else:
            event = "PRESERVED_AS_GENERAL_OR_NON_DIRECTIONAL_BASELINE_CONTEXT"
            succ = baseline[:8]
            note = "Earlier model row is accounted as predecessor context/baseline, not as an actual directional row ID."
        lid = sha256_text("\0".join([LINEAGE_NS, "identity47plus1", ident["identity"], event, "|".join(succ)]))
        out.append({
            "lineage_record_id": lid,
            "identity_ordinal": ident["identity_ordinal"],
            "identity": ident["identity"],
            "identity_kind": ident["identity_kind"],
            "operation_kind": op,
            "component": component,
            "lineage_event": event,
            "actual_successor_ids": "|".join(sorted(set(succ))),
            "planned_or_model_row_promoted": "false",
            "justification": note,
            "authority_ceiling": "AUTHORITY_0_PROVISIONAL_LINEAGE_ONLY",
        })
    if len(out) != 48 or len({r["identity"] for r in out}) != 48:
        raise SystemExit("47+1 reconciliation did not cover exactly 48 unique identities")
    return out

def checksum_manifest(root: Path, package_dir: Path, exclude_names: set[str] = frozenset({"CHECKSUMS.sha256"})) -> list[tuple[str, str]]:
    rows = []
    for p in sorted(package_dir.rglob("*")):
        if p.is_file() and p.name not in exclude_names:
            rows.append((sha256_bytes(p.read_bytes()), str(p.relative_to(root))))
    return rows

def write_checksums(root: Path, package_dir: Path) -> None:
    rows = checksum_manifest(root, package_dir)
    (package_dir / "CHECKSUMS.sha256").write_text("".join(f"{h}  {rel}\n" for h, rel in rows), encoding="utf-8")

def write_source_manifest(root: Path, package_dir: Path, actual_rows: list[ActualRow]) -> None:
    source_paths = sorted({r.source_path for r in actual_rows} | {STAGEA, IDENTITY, V5README})
    lines = []
    for rel in source_paths:
        lines.append(f"{source_hash(root, rel)}  {rel}\n")
    (package_dir / "SOURCE_MANIFEST.sha256").write_text("".join(lines), encoding="utf-8")

def counts(actual_rows: list[ActualRow], stagea: list[dict[str,str]], model: list[dict[str,str]], source_paths: list[str], candidates: list[SourceCandidate], exclusions: list[SourceCandidate], unclassified: list[SourceCandidate], self_tests: dict[str, bool] | None = None) -> dict[str, object]:
    ids = [r.actual_row_id for r in actual_rows]
    by_class: dict[str, int] = {}
    by_op: dict[str, int] = {}
    by_candidate_disposition: dict[str, int] = {}
    for r in actual_rows:
        by_class[r.classification] = by_class.get(r.classification, 0) + 1
        by_op[r.operation_kind] = by_op.get(r.operation_kind, 0) + 1
    for c in candidates:
        by_candidate_disposition[c.disposition] = by_candidate_disposition.get(c.disposition, 0) + 1
    return {
        "status": "AUTHORED_NOT_GO",
        "authority": 0,
        "accepted": False,
        "implementation_go": False,
        "source_head": None,
        "actual_rows": len(actual_rows),
        "actual_duplicate_ids": len(ids) - len(set(ids)),
        "actual_omissions": 0,
        "classification_counts": by_class,
        "operation_counts": dict(sorted(by_op.items())),
        "provisional_unresolved": by_class.get("UNRESOLVED", 0),
        "stage_a_planned_ids": len(stagea),
        "stage_a_mapping_duplicates": len(stagea) - len({r["stage_a_row_id"] for r in stagea}),
        "stage_a_mapping_omissions": max(0, 20 - len({r["stage_a_row_id"] for r in stagea})),
        "model_47_plus_1_rows": len(model),
        "model_mapping_duplicates": len(model) - len({r["identity"] for r in model}),
        "model_mapping_omissions": max(0, 48 - len({r["identity"] for r in model})),
        "source_candidate_rows": len(candidates),
        "source_candidate_exclusions": len(exclusions),
        "source_candidate_unclassified": len(unclassified),
        "source_candidate_dispositions": dict(sorted(by_candidate_disposition.items())),
        "self_tests": self_tests or {},
        "source_paths": source_paths,
    }

def render_readme(head: str, count: dict[str, object]) -> str:
    return f"""# ADR0011 directional raw custody C15 StageB v5 qualification package

Status: `STAGEB_C15_AUTHORED_NOT_GO`  
Source HEAD: `{head}`  
Authority: `0`; accepted: `false`; implementation GO: `false`; final GO: `false`.

This package is a docs/evidence-only StageB v5 qualification surface for the current post-GREEN worktree. It mechanically enumerates current source rows and provides provisional classifications for review. It does **not** independently accept C15 semantics, production admission, or implementation GO.

## Correction-safe boundary

- Actual row IDs are derived from current source identity (`path`, `function`, `line`, `operation_kind`, `component`, `backing_identity_rule`, source-line hash). Planned StageA IDs are never reused as actual IDs.
- Mechanical enumeration cannot accept semantic classification. The `NECESSARY`, `REUSE`, and `FALSE_POSITIVE` values are provisional source-evidence labels with contrary evidence/falsifiers and authority ceiling 0.
- `UNRESOLVED=0` here means this producer found no unresolved row in its bounded source package; it is not independent acceptance.
- Superseded and negative predecessor bundles remain preserved in lineage tables and `NEGATIVE_AND_SUPERSEDED_BUNDLES.md`.

## Key results

- Actual source rows: `{count['actual_rows']}`.
- Duplicate actual IDs: `{count['actual_duplicate_ids']}`.
- Omitted actual rows under this enumerator: `{count['actual_omissions']}`.
- Provisional UNRESOLVED rows: `{count['provisional_unresolved']}`.
- Source candidates discovered: `{count['source_candidate_rows']}`; explicit exclusions `{count['source_candidate_exclusions']}`; unclassified `{count['source_candidate_unclassified']}`.
- StageA planned IDs reconciled: `{count['stage_a_planned_ids']}` / 20; duplicates `{count['stage_a_mapping_duplicates']}`; omissions `{count['stage_a_mapping_omissions']}`.
- Earlier 47+1 model identities reconciled: `{count['model_47_plus_1_rows']}` / 48; duplicates `{count['model_mapping_duplicates']}`; omissions `{count['model_mapping_omissions']}`.

## Partition conclusions for review

- Request and response are preallocated independently through `Lease.Allocate("request-frame", ...)` and `Lease.Allocate("response-frame", ...)`, each delegating to Manager `privateDirectionalV5.allocate`, which charges the directional partition and creates C15 object records before the sole retained raw `make`.
- Result, element, Reader, and View rows are aliases/views into the response-frame backing, not allocations; they have explicit `ALIAS_VIEW_*` rows.
- The inherited general C15 table/account/lease machinery is classified `REUSE`, separate from directional request/response partition rows.
- Manager/generic owner is the sole terminal path: `genericTerminal.requestTerminal()` -> generic owner release callback -> `Reservation.Terminate()` -> `releaseAccount()` -> C15 frame object releases and account terminal release.

## Files

- `ACTUAL_SOURCE_ROWS.tsv` — source-derived actual rows with exact file/function/line/op/hash.
- `PROVISIONAL_CLASSIFICATIONS.tsv` — provisional labels, rationales, falsifiers, predecessors, authority ceiling.
- `SOURCE_CANDIDATES.tsv` — independent pattern-family source candidates and ROWED/EXCLUDED dispositions.
- `SOURCE_CANDIDATE_EXCLUSIONS.tsv` — explicit evidence-grounded candidate exclusions with falsifiers.
- `STAGEA_PLANNED_RECONCILIATION.tsv` — all 20 accepted StageA planning IDs mapped without promotion.
- `MODEL_47_PLUS_1_RECONCILIATION.tsv` — all 48 preserved/planned identities accounted.
- `DUPLICATES_OMISSIONS.json` — counts and duplicate/omission/candidate checks.
- `SELF_TESTS.json` — verifier tamper self-tests for deletion, duplication, reclassification, source drift, and extra unclassified candidates.
- `SOURCE_MANIFEST.sha256` — current source/predecessor hashes used by the package.
- `logs/source-enumeration.log` — generation and verification receipt.
- `verify_stageb_v5.py` — deterministic verifier/re-enumerator.
- `CHECKSUMS.sha256` — package checksums.

Run:

```bash
python3 docs/qualification/adr0011-directional-raw-custody-c15-stageb-v5/verify_stageb_v5.py --source /private/tmp/adr0011-incoming-calls-v1-6yiY4m
```

`STAGEB_C15_AUTHORED_NOT_GO`
"""

def render_review_request() -> str:
    return """# Review request: ADR0011 C15 StageB v5

Requested review token (if independently accepted later): `ADR0011_DIRECTIONAL_RAW_CUSTODY_C15_STAGEB_V5_REVIEWED`.

This producer does not request or claim final GO. Please review:

1. Whether every actual current-source directional reservation/allocation/copy/alias-view/decode/context/dispatch/transfer/observation-retention/terminal-clear-release row in scope is present.
2. Whether every independently discovered source candidate is either rowed or explicitly excluded with an evidence-grounded falsifier.
3. Whether any `NECESSARY`, `REUSE`, or `FALSE_POSITIVE` provisional classification overstates source evidence.
4. Whether StageA 20 planned IDs are fully reconciled without planned IDs becoming actual IDs.
5. Whether the 47+1 predecessor identity table preserves superseded/negative rows without silent deletion.
6. Whether request+response preallocation and no second retained response copy/double charge are established by source rows or need more evidence.
7. Whether Manager/generic owner is the sole terminal path and directional/general partitions remain independent.

Authority ceiling: source-evidence preparation only (`0`). Independent reviewer must preserve correction lineage and may split, withdraw, or mark rows unresolved.
"""

def render_negative() -> str:
    return """# Negative and superseded bundles retained

- `adr0011-directional-raw-custody-c15-enumeration-v1` remains a pre-implementation, all-UNRESOLVED enumeration bundle. Its absent-source proposed rows are not promoted here.
- `adr0011-directional-raw-custody-c15-two-stage-contract-v2` remains the StageA/StageB contract. StageA row IDs are predecessor lineage only.
- The earlier 47+1 model is reconciled in `MODEL_47_PLUS_1_RECONCILIATION.tsv`; proposed `call_hierarchy_directional_raw_v1_private.go` rows are superseded by current v5 source-derived IDs, not normalized into source facts.
- Existing nearby borrows from `b4_owned_method_pair_lease.go` remain negative/nonreusable predecessor context. StageB v5 uses response-frame aliases/views and C15 zero-charge aliases instead of treating those borrows as retained allocations.
- `View.SharesBackingPrivate` is intentionally classified `FALSE_POSITIVE`: it is a private qualification helper for alias proof, not production custody or retained allocation behavior.
"""

def verify(root: Path, package_dir: Path, quiet: bool = False) -> dict[str, object]:
    actual = build_actual_rows(root)
    expected_actual = [asdict(r) for r in actual]
    actual_path = package_dir / "ACTUAL_SOURCE_ROWS.tsv"
    if actual_path.exists():
        got = read_tsv(actual_path)
        if got != [{k: str(v) for k, v in row.items()} for row in expected_actual]:
            raise SystemExit("ACTUAL_SOURCE_ROWS.tsv does not match current source enumeration")
    stagea = stagea_reconciliation(root, actual)
    model = model_reconciliation(root, actual)
    candidates, exclusions, unclassified = discover_source_candidates(root, actual)
    assert_required_invariants(actual)
    assert_source_candidate_coverage(actual, candidates, unclassified)
    self_tests = run_self_tests(actual, candidates, unclassified)
    if (package_dir / "SOURCE_CANDIDATES.tsv").exists() and read_tsv(package_dir / "SOURCE_CANDIDATES.tsv") != [{k: str(v) for k, v in asdict(row).items()} for row in candidates]:
        raise SystemExit("SOURCE_CANDIDATES.tsv mismatch")
    if (package_dir / "SOURCE_CANDIDATE_EXCLUSIONS.tsv").exists() and read_tsv(package_dir / "SOURCE_CANDIDATE_EXCLUSIONS.tsv") != [{k: str(v) for k, v in asdict(row).items()} for row in exclusions]:
        raise SystemExit("SOURCE_CANDIDATE_EXCLUSIONS.tsv mismatch")
    if (package_dir / "SELF_TESTS.json").exists() and json.loads((package_dir / "SELF_TESTS.json").read_text(encoding="utf-8")) != self_tests:
        raise SystemExit("SELF_TESTS.json mismatch")
    if (package_dir / "STAGEA_PLANNED_RECONCILIATION.tsv").exists() and read_tsv(package_dir / "STAGEA_PLANNED_RECONCILIATION.tsv") != stagea:
        raise SystemExit("STAGEA_PLANNED_RECONCILIATION.tsv mismatch")
    if (package_dir / "MODEL_47_PLUS_1_RECONCILIATION.tsv").exists() and read_tsv(package_dir / "MODEL_47_PLUS_1_RECONCILIATION.tsv") != model:
        raise SystemExit("MODEL_47_PLUS_1_RECONCILIATION.tsv mismatch")
    ids = [r.actual_row_id for r in actual]
    if len(ids) != len(set(ids)):
        raise SystemExit("duplicate actual row id")
    if any(r.classification == "UNRESOLVED" for r in actual):
        raise SystemExit("provisional UNRESOLVED rows present")
    source_paths = sorted({r.source_path for r in actual} | {STAGEA, IDENTITY, V5README})
    missing = [p for p in source_paths if not (root / p).exists()]
    if missing:
        raise SystemExit(f"missing source paths: {missing}")
    c = counts(actual, stagea, model, source_paths, candidates, exclusions, unclassified, self_tests)
    c["source_head"] = git(root, "rev-parse", "HEAD")
    if c["stage_a_mapping_omissions"] != 0 or c["model_mapping_omissions"] != 0 or c["actual_duplicate_ids"] != 0 or c["source_candidate_unclassified"] != 0:
        raise SystemExit("duplicate/omission/candidate count failed")
    if not quiet:
        print(json.dumps({"verified": True, **c}, indent=2, sort_keys=True))
    return c

def generate(root: Path) -> None:
    package_dir = root / PACKAGE
    logs = package_dir / "logs"
    logs.mkdir(parents=True, exist_ok=True)
    actual = build_actual_rows(root)
    stagea = stagea_reconciliation(root, actual)
    model = model_reconciliation(root, actual)
    candidates, exclusions, unclassified = discover_source_candidates(root, actual)
    assert_required_invariants(actual)
    assert_source_candidate_coverage(actual, candidates, unclassified)
    self_tests = run_self_tests(actual, candidates, unclassified)
    source_paths = sorted({r.source_path for r in actual} | {STAGEA, IDENTITY, V5README})
    c = counts(actual, stagea, model, source_paths, candidates, exclusions, unclassified, self_tests)
    c["source_head"] = git(root, "rev-parse", "HEAD")

    actual_fields = list(asdict(actual[0]).keys())
    write_tsv(package_dir / "ACTUAL_SOURCE_ROWS.tsv", [asdict(r) for r in actual], actual_fields)
    class_fields = ["actual_row_id", "classification", "rationale", "contrary_evidence_or_falsifier", "predecessor_lineage", "authority_ceiling", "source_path", "source_line", "function", "operation_kind", "component"]
    write_tsv(package_dir / "PROVISIONAL_CLASSIFICATIONS.tsv", [asdict(r) for r in actual], class_fields)
    candidate_fields = list(asdict(candidates[0]).keys()) if candidates else list(SourceCandidate.__annotations__.keys())
    write_tsv(package_dir / "SOURCE_CANDIDATES.tsv", [asdict(r) for r in candidates], candidate_fields)
    write_tsv(package_dir / "SOURCE_CANDIDATE_EXCLUSIONS.tsv", [asdict(r) for r in exclusions], candidate_fields)
    write_tsv(package_dir / "STAGEA_PLANNED_RECONCILIATION.tsv", stagea, ["lineage_record_id", "stage_a_row_id", "stage_a_status", "stage_a_component", "lineage_event", "actual_successor_ids", "planned_row_promoted", "justification", "authority_ceiling"])
    write_tsv(package_dir / "MODEL_47_PLUS_1_RECONCILIATION.tsv", model, ["lineage_record_id", "identity_ordinal", "identity", "identity_kind", "operation_kind", "component", "lineage_event", "actual_successor_ids", "planned_or_model_row_promoted", "justification", "authority_ceiling"])
    (package_dir / "DUPLICATES_OMISSIONS.json").write_text(json.dumps(c, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    (package_dir / "SELF_TESTS.json").write_text(json.dumps(self_tests, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    write_source_manifest(root, package_dir, actual)
    (package_dir / "README.md").write_text(render_readme(c["source_head"], c), encoding="utf-8")
    (package_dir / "REVIEW_REQUEST.md").write_text(render_review_request(), encoding="utf-8")
    (package_dir / "NEGATIVE_AND_SUPERSEDED_BUNDLES.md").write_text(render_negative(), encoding="utf-8")
    (logs / "source-enumeration.log").write_text(json.dumps({"generated": True, **c}, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    # Copy this generator as verifier with executable shebang already present.
    this_file = Path(__file__).resolve()
    verifier = package_dir / "verify_stageb_v5.py"
    if this_file != verifier.resolve():
        verifier.write_text(this_file.read_text(encoding="utf-8"), encoding="utf-8")
    os.chmod(verifier, 0o755)
    write_checksums(root, package_dir)
    # Verify after checksums are written, but checksum file is excluded from self-manifest.
    verify(root, package_dir, quiet=True)
    print(json.dumps({"generated": str(package_dir), **c}, indent=2, sort_keys=True))

def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--source", default=".")
    ap.add_argument("--verify-only", action="store_true")
    args = ap.parse_args()
    root = Path(args.source).resolve()
    package_dir = root / PACKAGE
    if args.verify_only:
        verify(root, package_dir)
    else:
        generate(root)

if __name__ == "__main__":
    main()
