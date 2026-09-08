package hydratedevidence

import (
 "encoding/json"
 "fmt"
 "os"
 "strings"
 "testing"
)

const focusNode = "497028053845546f6d189a91469614a057556afee8e906937b98d94160e02075"
const focusEdge = "sha256:e16c80b01aa9de78ff896b411d93746a6bfa749c1a80bc7b1c3bd251df81fa4b"
const focusEdge2 = "sha256:42df1c10ff307e342f79d0a3d3f28cf115a501f6274619801c612e8705bef707"
func focusedFixture(t *testing.T) Input {
 t.Helper()
 raw, err := os.ReadFile("testdata/focused-fr20.v2.json")
 if err != nil { t.Fatal(err) }
 if Digest(raw) != "sha256:8913b3d062312be15531728f801e2f677f4a65852fd5fbeaf4ef1007ae1f83cf" { t.Fatal("original FR20 bytes changed") }
 return Input{Artifact: raw}
}
func focusedRun(t *testing.T, in Input, f FocusRequest) FocusResult {
 t.Helper()
 r, e := HydrateFocused(in, f)
 if e != nil { t.Fatal(e) }
 return r
}
func TestFocusedTypedJoin(t *testing.T) {
 in := focusedFixture(t); f := DefaultFocusRequest()
 f.NodeIDs = []string{focusNode}; f.RelationIDs = []string{focusEdge, focusEdge2}
 r := focusedRun(t,in,f)
 if len(r.Manifest.Origins)!=3 { t.Fatal("ASSERT_TYPED_JOIN: all node/edge groups required") }
 want := []string{"/graph/nodes/0/range", "/graph/edges/0/call_sites/0", "/graph/edges/1/call_sites/0"}
 for i, p := range want {
  o:=r.Manifest.Origins[i]
  if len(o.Sites)!=1 || o.Sites[0].Pointer!=p || len(o.Sites[0].OriginIDs)==0 { t.Fatalf("ASSERT_TYPED_JOIN: group %d: %+v",i,o) }
  if i>0 && (o.CallSiteCount!=1 || o.Sites[0].RelationID!=f.RelationIDs[i-1]) { t.Fatal("ASSERT_TYPED_JOIN: actual native callsites omitted") }
 }
 if r.Manifest.Origins[1].Sites[0].URI!=r.Manifest.Origins[0].Sites[0].URI { t.Fatal("ASSERT_TYPED_JOIN: callsite must use caller URI") }
 for _, s := range r.Request.Selections { if s.Mode!="RETAINED_RANGE" { t.Fatal("ASSERT_TYPED_JOIN: default expanded") } }
}
func TestFocusedUnknownDuplicate(t *testing.T) {
 in:=focusedFixture(t); f:=DefaultFocusRequest(); f.NodeIDs=[]string{"unknown",focusNode,focusNode}; f.RelationIDs=[]string{focusNode}
 r:=focusedRun(t,in,f)
 if len(r.Manifest.Origins)!=4 || r.Manifest.Origins[0].RequestedID!="unknown" || r.Manifest.Origins[0].Status!="UNKNOWN_ID" || r.Manifest.Origins[3].Status!="UNKNOWN_ID" || r.Manifest.DuplicatePolicy!="PRESERVE_OCCURRENCES" { t.Fatal("ASSERT_OCCURRENCES: unknown and duplicate IDs must remain typed and explicit") }
 a,b:=r.Manifest.Origins[1].Sites[0].OriginIDs[0],r.Manifest.Origins[2].Sites[0].OriginIDs[0]
 if a==b { t.Fatal("ASSERT_OCCURRENCES: duplicates silently collapsed") }
}
func TestFocusedCatalogNonSource(t *testing.T) {
 in:=focusedFixture(t); f:=DefaultFocusRequest(); f.RelationIDs=[]string{focusEdge}
 c,e:=Inspect(in,f.CorePolicy); if e!=nil { t.Fatal(e) }; r:=focusedRun(t,in,f)
 count:=0; forbidden:=map[string]bool{}
 for _,rec:=range c.Records { if rec.SourceAttribution=="NON_SOURCE" { count++; forbidden[rec.ID]=true } }
 if count==0 || r.Manifest.ExcludedNonSourceRecords!=count || !same(c.Sources,r.Bundle.Sources) { t.Fatal("ASSERT_CATALOG_NON_SOURCE: complete source catalog retained, bookkeeping excluded") }
 for _,s:=range r.Request.Selections { if forbidden[s.RecordID] { t.Fatal("ASSERT_CATALOG_NON_SOURCE: bookkeeping became source selection") } }
 for _,o:=range r.Bundle.Origins { if o.Status=="UNKNOWN_SOURCE" { t.Fatal("ASSERT_CATALOG_NON_SOURCE: false missing-source warning") } }
 if e:=Validate(in,r.Request,r.Bundle); e!=nil { t.Fatal(e) }
}
func TestFocusedPrivacyRangesVersions(t *testing.T) {
 in:=focusedFixture(t); f:=DefaultFocusRequest(); f.RelationIDs=[]string{focusEdge}
 r:=focusedRun(t,in,f)
 if len(r.Bundle.Origins)!=2 || len(r.Bundle.Spans)!=0 { t.Fatal("ASSERT_PRIVACY_RANGES: default body exclusion and both receipts") }
 for _,o:=range r.Bundle.Origins { if o.Status!="PRIVACY_EXCLUDED" || o.CoordinateAuthority!=Native { t.Fatal("ASSERT_PRIVACY_RANGES: retained coordinate authority") } }
 f.IncludeBodies=true; r=focusedRun(t,in,f)
 if len(r.Bundle.Spans)!=2 { t.Fatal("ASSERT_PRIVACY_RANGES: same-URI versions must remain distinct") }
 for _,s:=range r.Bundle.Spans { if s.Bytes!=(Interval{1,2}) || len(s.Content)!=1 { t.Fatal("ASSERT_PRIVACY_RANGES: retained range expanded") } }
 if r.Bundle.Spans[0].SourceID==r.Bundle.Spans[1].SourceID { t.Fatal("ASSERT_PRIVACY_RANGES: URI receipt collapse") }
 f.WholeFile=true; r=focusedRun(t,in,f)
 if len(r.Bundle.Spans)!=2 { t.Fatal("ASSERT_PRIVACY_RANGES: whole-file missing") }
 for _,s:=range r.Bundle.Spans { if s.Bytes!=(Interval{0,9}) || len(s.Content)!=9 || s.ContentHash!=Digest(s.Content) { t.Fatal("ASSERT_PRIVACY_RANGES: whole-file hash/bytes") } }
 for _,o:=range r.Bundle.Origins { if o.CoordinateAuthority!="UNAVAILABLE" || o.OriginalRange!=nil { t.Fatal("ASSERT_PRIVACY_RANGES: whole-file invented coordinates") } }
 if e:=ValidateFocused(in,f,r); e!=nil { t.Fatal(e) }
}
func TestFocusedEndpointPolicy(t *testing.T) {
 in:=focusedFixture(t); f:=DefaultFocusRequest(); f.RelationIDs=[]string{focusEdge}; f.EndpointContext=true
 r:=focusedRun(t,in,f)
 if len(r.Manifest.Origins)!=1 || len(r.Manifest.Origins[0].Sites)!=3 || r.Manifest.Origins[0].Sites[0].Role!="CALL_SITE" || r.Manifest.Origins[0].Sites[1].Pointer!="/graph/nodes/0/range" || r.Manifest.Origins[0].Sites[2].Pointer!="/graph/nodes/1/range" { t.Fatal("ASSERT_ENDPOINT_POLICY: explicit endpoints must accompany actual site") }
}
func TestFocusedTampering(t *testing.T) {
 in:=focusedFixture(t); f:=DefaultFocusRequest(); f.NodeIDs=[]string{"unknown"}; f.RelationIDs=[]string{focusEdge}; f.IncludeBodies=true
 for _, mutation:=range []string{"drop-requested", "drop-callsite", "authority", "resolved-unknown", "input", "policy"} {
 t.Run(mutation,func(t *testing.T) {
 r:=focusedRun(t,in,f)
 // The empty compiling stub is a present-but-wrong object, not a compile failure.
 if len(r.Manifest.Origins)<2 { if ValidateFocused(in,f,r)==nil { t.Fatal("ASSERT_MANIFEST_REJECTS_TAMPER: empty manifest accepted") }; return }
 switch mutation {
 case "drop-requested": r.Manifest.Origins=r.Manifest.Origins[1:]
 case "drop-callsite": r.Manifest.Origins[1].Sites=nil
 case "authority": r.Bundle.Origins[0].CoordinateAuthority=Caller; seal(&r.Bundle)
 case "resolved-unknown": r.Manifest.Origins[0].Status="MAPPED"
 case "input": r.Manifest.InputDigests=[]string{"sha256:wrong"}
 case "policy": r.Manifest.ReceiptPolicy="LATEST_URI"
 }
 r.Manifest.Digest=""; r.Manifest.Digest=valueDigest(r.Manifest)
 if ValidateFocused(in,f,r)==nil { t.Fatal("ASSERT_MANIFEST_REJECTS_TAMPER: rehashed tampering accepted") }
 }) }
}
func TestFocusedSidecarStates(t *testing.T) {
 in:=focusedFixture(t); f:=DefaultFocusRequest(); f.IncludeBodies=true
 b:=[]byte("abc"); h:=Digest(b)
 side:=Sidecar{SchemaVersion:SidecarVersion,ArtifactDigest:Digest(in.Artifact),Authority:Caller,Qualification:NonAuthoritative,Sources:[]AssertedSource{},Records:[]AssertedRecord{}}
 for _,state:=range []string{"RETAINED_BYTES","REFERENCE_ONLY","MISSING","TRUNCATED_INPUT"} {
 s:=AssertedSource{ID:state,URI:"file:///asserted",ReceiptReference:state,VersionReference:state,SourceEncoding:"utf-8",State:state}
 if state=="RETAINED_BYTES" || state=="TRUNCATED_INPUT" { s.Content=&b; s.ContentHash=&h }
 side.Sources=append(side.Sources,s)
 side.Records=append(side.Records,AssertedRecord{ID:state,Kind:"CLAIM",SourceIDs:[]string{state},Range:&Range{End:Position{Character:1}},Encoding:"utf-8"})
 }
 side.Records=append(side.Records,AssertedRecord{ID:"no-range",Kind:"CLAIM",SourceIDs:[]string{"RETAINED_BYTES"}},AssertedRecord{ID:"invalid",Kind:"CLAIM",SourceIDs:[]string{"RETAINED_BYTES"},Range:&Range{End:Position{Character:99}},Encoding:"utf-8"})
 in.Sidecars=[][]byte{encoded(t,side)}
 for _,rec:=range side.Records { f.SidecarRecordIDs=append(f.SidecarRecordIDs,"sidecar:"+Digest(in.Sidecars[0])+":"+rec.ID) }
 r:=focusedRun(t,in,f)
 if len(r.Bundle.Origins)!=6 { t.Fatal("ASSERT_SIDECAR_STATES: missing explicit asserted origins") }
 for i,w:=range []string{"EXPORTED","REFERENCE_ONLY","MISSING_BYTES","TRUNCATED_INPUT","UNKNOWN_BOUNDARY","INVALID_COORDINATES"} {
 o:=r.Bundle.Origins[i]; if o.Status!=w || o.Record.Authority!=Caller || o.Record.Qualification!=NonAuthoritative { t.Fatalf("ASSERT_SIDECAR_STATES: %d %+v want %s",i,o,w) }
 }
 if ValidateFocused(in,f,r)!=nil { t.Fatal("ASSERT_SIDECAR_STATES: asserted replay rejected") }
}
func TestFocusedUnsupported(t *testing.T) {
 if _,e:=HydrateFocused(Input{Artifact:[]byte(`{"schema_version":"unsupported.provider.v1"}`)},DefaultFocusRequest()); e==nil || !strings.Contains(e.Error(),"unsupported") { t.Fatal("ASSERT_UNSUPPORTED: no generic provider family") }
}
func TestFocusedActualFR20Measurement(t *testing.T) {
 in:=focusedFixture(t); f:=DefaultFocusRequest(); f.RelationIDs=[]string{focusEdge,focusEdge2}; f.IncludeBodies=true
 r:=focusedRun(t,in,f)
 if len(r.Manifest.Origins)!=2 { t.Fatal("ASSERT_ACTUAL_REPLAY: actual two edge identities must resolve") }
 if e:=ValidateFocused(in,f,r); e!=nil { t.Fatal(e) }
 text,e:=Text(in,r.Request,r.Bundle); if e!=nil { t.Fatal(e) }
 c,e:=Inspect(in,f.CorePolicy); if e!=nil { t.Fatal(e) }
 all:=Request{Policy:r.Request.Policy,Selections:[]Selection{}}
 for _,rec:=range c.Records {
 ids:=rec.SourceIDs; if len(ids)==0 { ids=[]string{""} }
 for _,sid:=range ids { all.Selections=append(all.Selections,Selection{ID:fmt.Sprint(len(all.Selections)),RecordID:rec.ID,SourceID:sid,Mode:"RETAINED_RANGE"}) }
 }
 b,e:=Hydrate(in,all); if e!=nil { t.Fatal(e) }; baseline,e:=Text(in,all,b); if e!=nil { t.Fatal(e) }
 unique:=map[string]bool{}; n:=0
 for _,s:=range r.Bundle.Spans { if !unique[s.ContentHash] { unique[s.ContentHash]=true; n+=len(s.Content) } }
 raw,_:=json.Marshal(r)
 t.Logf("MEASURE actual-fr20 sha256=8913b3d062312be15531728f801e2f677f4a65852fd5fbeaf4ef1007ae1f83cf selected_core_text_utf8_bytes=%d all_catalog_core_text_utf8_bytes=%d focused_json_bytes=%d unique_selected_content_bytes=%d catalog_sources=%d catalog_records=%d selected_origins=%d",len(text),len(baseline),len(raw),n,len(c.Sources),len(c.Records),len(r.Bundle.Origins))
 if len(text)>=len(baseline) { t.Fatal("ASSERT_ACTUAL_REPLAY: this fixture focused text did not shrink") }
}
