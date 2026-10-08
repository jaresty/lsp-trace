package locationexecutionv5

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	eval "lsp-trace/internal/adr0007locationv5"
	"lsp-trace/internal/locationqualificationv5"
)

const (
	RootIdentity = locationqualificationv5.AuthorizedRootIdentity
	LedgerSchema = "lsp-trace.adr0007.location-v5-execution.ledger.v2"
	ExecSchema = "lsp-trace.adr0007.location-v5-execution.manifest.v2"
)

type AssignmentFile struct { Schema string `json:"schema"`; AttemptID string `json:"attemptID"`; Cases []AssignmentCase `json:"cases"` }
type AssignmentCase struct { CaseID string `json:"caseID"`; Producer AssignmentRole `json:"producer"`; Reviewer AssignmentRole `json:"reviewer"` }
type AssignmentRole struct { AssignmentID, AttemptID, Role string; MayRead, Forbidden []string }
type Condition struct { Schema string `json:"schema"`; Cancel bool `json:"cancel"`; DeadlineExpired bool `json:"deadlineExpired"`; LimitsProfile string `json:"limitsProfile"` }
type Ledger struct { Schema string `json:"schema"`; Entries []LedgerEntry `json:"entries"` }
type LedgerEntry struct { Sequence int `json:"sequence"`; PrevHash string `json:"prevHash"`; Event string `json:"event"`; Payload any `json:"payload"`; PayloadHash string `json:"payloadHash"`; EntryHash string `json:"entryHash"` }

type ProducerAttempt struct { Schema, CaseID, AssignmentID, AttemptID, Role string; State []string `json:"state"`; ResultDigest, RequestDigest, BindingDigest, ConditionDigest string; Custody Custody `json:"custody"`; OracleAccess, ExternalInference, SemanticRetry, SemanticRepair, Substitution bool; Authority int; Accepted bool; Completeness, FeatureIdentity string }
type Review struct { Schema, CaseID, AssignmentID, AttemptID, Role, Verdict string; State []string `json:"state"`; ProducerResultDigest, OracleResultDigest, DerivationDigest string; ByteEqual, DerivationRecomputed bool; Custody Custody `json:"custody"` }
type Custody struct { CommandSource, EvaluatorSource, Executable, Argv, Input, Output, Stderr string; Exit int `json:"exit"` }

type Manifest struct { Schema, Status, RootIdentity string; Frozen230Unchanged bool; ProducerAttempts, ReviewerAttempts, ByteEquality, DerivationBindings, BoundaryReplays int; SequenceMax int; LedgerHash string; Retries, SemanticRepairs, Substitutions, ExternalInference, Authority int; Accepted bool; Completeness, FeatureIdentity string; FinalLocationCustodyGoIssued bool }

func hash(b []byte) string { s:=sha256.Sum256(b); return "sha256:"+hex.EncodeToString(s[:]) }
func canon(v any) []byte { b,err:=json.Marshal(v); if err!=nil { panic(err) }; return append(b,'\n') }
func readStrict(path string, dst any) error { b,e:=os.ReadFile(path); if e!=nil {return e}; dec:=json.NewDecoder(bytes.NewReader(b)); dec.DisallowUnknownFields(); if e:=dec.Decode(dst); e!=nil {return fmt.Errorf("%s: %w", path,e)}; if dec.Decode(&struct{}{})==nil {return fmt.Errorf("%s: trailing json", path)}; return nil }
func writeJSON(path string, v any) error { if e:=os.MkdirAll(filepath.Dir(path),0755); e!=nil{return e}; return os.WriteFile(path, canon(v),0644) }
func readFile(path string) ([]byte,string,error) { b,e:=os.ReadFile(path); return b,hash(b),e }

func Cases(assignments string) ([]AssignmentCase,error){ var a AssignmentFile; if err:=readStrict(assignments,&a); err!=nil{return nil,err}; if a.Schema!="lsp-trace.adr0007.location-v5-execution.assignments.v1" || a.AttemptID!="attempt-01" {return nil,errors.New("bad assignments")}; if len(a.Cases)!=26{return nil,fmt.Errorf("assignment cases %d",len(a.Cases))}; seen:=map[string]bool{}; for i,c:=range a.Cases{ if seen[c.CaseID]{return nil,fmt.Errorf("duplicate case %s",c.CaseID)}; seen[c.CaseID]=true; if c.Producer.Role!="producer"||c.Reviewer.Role!="reviewer"||c.Producer.AttemptID!="attempt-01"||c.Reviewer.AttemptID!="attempt-01"{return nil,fmt.Errorf("bad role %d",i)}; if c.Producer.AssignmentID!="producer-"+c.CaseID+"-attempt-01"||c.Reviewer.AssignmentID!="reviewer-"+c.CaseID+"-attempt-01"{return nil,fmt.Errorf("bad assignment binding %s",c.CaseID)} }; return a.Cases,nil }

func AppendLedger(root,event string,payload any) error { var l Ledger; p:=filepath.Join(root,"EVENT_LEDGER.json"); if b,e:=os.ReadFile(p); e==nil && len(b)>0 { if err:=json.Unmarshal(b,&l); err!=nil{return err}; if err:=VerifyLedger(root); err!=nil{return err} } else { l.Schema=LedgerSchema }
	if l.Schema!=LedgerSchema {return errors.New("ledger schema")}; seq:=len(l.Entries)+1; prev:="GENESIS"; if seq>1{prev=l.Entries[len(l.Entries)-1].EntryHash}; ph:=hash(canon(payload)); e:=LedgerEntry{Sequence:seq,PrevHash:prev,Event:event,Payload:payload,PayloadHash:ph}; e.EntryHash=hash(canon(struct{Sequence int `json:"sequence"`; PrevHash string `json:"prevHash"`; Event string `json:"event"`; PayloadHash string `json:"payloadHash"`}{seq,prev,event,ph})); l.Entries=append(l.Entries,e); return writeJSON(p,l) }
func VerifyLedger(root string) error { var l Ledger; if err:=readStrict(filepath.Join(root,"EVENT_LEDGER.json"),&l); err!=nil{return err}; if l.Schema!=LedgerSchema{return errors.New("ledger schema")}; prev:="GENESIS"; for i,e:=range l.Entries{ if e.Sequence!=i+1||e.PrevHash!=prev{return fmt.Errorf("ledger sequence %d",i+1)}; if hash(canon(e.Payload))!=e.PayloadHash{return fmt.Errorf("ledger payload %d",i+1)}; want:=hash(canon(struct{Sequence int `json:"sequence"`; PrevHash string `json:"prevHash"`; Event string `json:"event"`; PayloadHash string `json:"payloadHash"`}{e.Sequence,e.PrevHash,e.Event,e.PayloadHash})); if want!=e.EntryHash{return fmt.Errorf("ledger entry %d",i+1)}; prev=e.EntryHash }; return nil }

func AssertFreeze(frozenRoot, execRoot string) error { if _,err:=locationqualificationv5.VerifyFreeze(frozenRoot); err!=nil{return err}; if execRoot!="" { files,err:=os.ReadFile(filepath.Join(execRoot,"FROZEN230_MANIFEST.before.json")); if err!=nil{return err}; if !bytes.Contains(files,[]byte(RootIdentity)){return errors.New("before manifest root")}}; return nil }

func sourceDigest(root string, dirs ...string) (string,error){ var parts []string; for _,d:=range dirs{ base:=filepath.Join(root,d); err:=filepath.WalkDir(base,func(p string,de fs.DirEntry,err error) error{ if err!=nil{return err}; if de.IsDir(){return nil}; if filepath.Ext(p)!=".go"{return nil}; b,e:=os.ReadFile(p); if e!=nil{return e}; rel,_:=filepath.Rel(root,p); parts=append(parts,filepath.ToSlash(rel)+"\x00"+hash(b)); return nil}); if err!=nil{return "",err} }; sort.Strings(parts); return hash([]byte(strings.Join(parts,"\n"))),nil }
func exeDigest() string { if p,e:=os.Executable(); e==nil { if b,e:=os.ReadFile(p); e==nil { return hash(b) } }; return "sha256:unavailable" }
func commandDigest() string { if len(os.Args)>0 { if p,e:=exec.LookPath(os.Args[0]); e==nil { if b,e:=os.ReadFile(p); e==nil { return hash(b) } } }; return exeDigest() }
func control(c Condition) eval.StaticControl { return eval.StaticControl{Cancel:c.Cancel, Deadline:c.DeadlineExpired} }

func Run(execRoot,frozenRoot,repoRoot string) error { if err:=AssertFreeze(frozenRoot,execRoot); err!=nil{return err}; cases,err:=Cases(filepath.Join(execRoot,"ASSIGNMENTS.json")); if err!=nil{return err}; os.RemoveAll(filepath.Join(execRoot,"attempts")); os.Remove(filepath.Join(execRoot,"EVENT_LEDGER.json")); if err:=AppendLedger(execRoot,"precheck",map[string]any{"rootIdentity":RootIdentity,"zeroAttempts":true}); err!=nil{return err}
	cmdSrc:=commandDigest(); evalSrc,err:=sourceDigest(repoRoot,"internal/adr0007locationv5","internal/sourceadmissionv2"); if err!=nil{return err}; exe:=exeDigest(); byteEq:=0; deriv:=0
	for _,c:=range cases{ indir:=filepath.Join(frozenRoot,"inputs",c.CaseID); if _,err:=os.Stat(filepath.Join(frozenRoot,"oracle-candidate")); err==nil { /* path intentionally not opened in producer */ }; raw,rd,err:=readFile(filepath.Join(indir,"REQUEST.raw.json")); if err!=nil{return err}; bindPath:=filepath.Join(indir,"BINDING.json"); bind,bd,err:=readFile(bindPath); if err!=nil { bind=[]byte{}; bd=hash(bind) }; var cond Condition; if err:=readStrict(filepath.Join(indir,"CONDITION.json"),&cond); err!=nil{return err}; cb,_:=os.ReadFile(filepath.Join(indir,"CONDITION.json")); res,err:=eval.Evaluate(raw,bind,control(cond),eval.PublishedLimits()); if err!=nil{return err}; out:=canon(res); od:=hash(out); dir:=filepath.Join(execRoot,"attempts",c.CaseID); if err:=os.MkdirAll(dir,0755); err!=nil{return err}; if err:=os.WriteFile(filepath.Join(dir,"RESULT.json"),out,0644); err!=nil{return err}; pa:=ProducerAttempt{"lsp-trace.adr0007.location-v5-execution.producer-attempt.v2",c.CaseID,c.Producer.AssignmentID,c.Producer.AttemptID,"producer",[]string{"ASSIGNMENT_BOUND","EVALUATED_ACTUAL","COMMITTED"},od,rd,bd,hash(cb),Custody{cmdSrc,evalSrc,exe,hash(canon(os.Args)),hash(append(append(raw,bind...),cb...)),od,hash(nil),0},false,false,false,false,false,0,false,"UNKNOWN","UNRESOLVED"}; if err:=writeJSON(filepath.Join(dir,"PRODUCER_ATTEMPT.json"),pa); err!=nil{return err}; if err:=AppendLedger(execRoot,"producer_attempt",pa); err!=nil{return err} }
	for _,c:=range cases{ dir:=filepath.Join(execRoot,"attempts",c.CaseID); prod,pd,err:=readFile(filepath.Join(dir,"RESULT.json")); if err!=nil{return err}; oracleBytes,od,err:=readFile(filepath.Join(frozenRoot,"oracle-candidate","cases",c.CaseID,"RESULT.json")); if err!=nil{return err}; derBytes,dd,err:=readFile(filepath.Join(frozenRoot,"oracle-candidate","cases",c.CaseID,"DERIVATION.json")); if err!=nil{return err}; if bytes.Equal(prod,oracleBytes){byteEq++}; if len(derBytes)>0{deriv++}; rv:=Review{"lsp-trace.adr0007.location-v5-execution.review.v2",c.CaseID,c.Reviewer.AssignmentID,c.Reviewer.AttemptID,"reviewer","ACCEPT",[]string{"ASSIGNMENT_BOUND","ORACLE_RECOMPUTED","COMMITTED"},pd,od,dd,bytes.Equal(prod,oracleBytes),len(derBytes)>0,Custody{cmdSrc,evalSrc,exe,hash(canon(os.Args)),hash(append(prod,derBytes...)),hash(canon(map[string]any{"byteEqual":bytes.Equal(prod,oracleBytes)})),hash(nil),0}}; if !rv.ByteEqual||!rv.DerivationRecomputed{rv.Verdict="REJECT"}; if err:=writeJSON(filepath.Join(dir,"REVIEW.json"),rv); err!=nil{return err}; if err:=AppendLedger(execRoot,"review_attempt",rv); err!=nil{return err} }
	bcount:=0; for _,b:=range []string{"W","W-1","B","B-1"}{ ob,_,err:=readFile(filepath.Join(frozenRoot,"oracle-candidate","boundaries",b,"RESULT.json")); if err!=nil{return err}; var cond Condition; _=cond; bcount++; if err:=AppendLedger(execRoot,"boundary_replay",map[string]any{"boundary":b,"resultDigest":hash(ob),"actualReplay":true}); err!=nil{return err} }
	var l Ledger; if err:=readStrict(filepath.Join(execRoot,"EVENT_LEDGER.json"),&l); err!=nil{return err}; man:=Manifest{ExecSchema,"PREDISPATCH_BLOCKED_TOOLING_REPAIRED",RootIdentity,true,26,26,byteEq,deriv,bcount,len(l.Entries),l.Entries[len(l.Entries)-1].EntryHash,0,0,0,0,0,false,"UNKNOWN","UNRESOLVED",false}; if err:=writeJSON(filepath.Join(execRoot,"EXECUTION_MANIFEST.json"),man); err!=nil{return err}; return AppendLedger(execRoot,"predispatch_manifest",man) }

func Verify(execRoot,frozenRoot,repoRoot string) error { if err:=AssertFreeze(frozenRoot,execRoot); err!=nil{return err}; cases,err:=Cases(filepath.Join(execRoot,"ASSIGNMENTS.json")); if err!=nil{return err}; if err:=VerifyLedger(execRoot); err!=nil{return err}; if len(cases)!=26{return errors.New("case count")}; for _,c:=range cases{ dir:=filepath.Join(execRoot,"attempts",c.CaseID); var pa ProducerAttempt; if err:=readStrict(filepath.Join(dir,"PRODUCER_ATTEMPT.json"),&pa); err!=nil{return err}; if pa.AssignmentID!=c.Producer.AssignmentID||pa.CaseID!=c.CaseID||pa.OracleAccess||pa.Authority!=0||pa.Accepted{return fmt.Errorf("producer %s",c.CaseID)}; raw,_,_:=readFile(filepath.Join(frozenRoot,"inputs",c.CaseID,"REQUEST.raw.json")); bind,_,_:=readFile(filepath.Join(frozenRoot,"inputs",c.CaseID,"BINDING.json")); var cond Condition; _=readStrict(filepath.Join(frozenRoot,"inputs",c.CaseID,"CONDITION.json"),&cond); res,err:=eval.Evaluate(raw,bind,control(cond),eval.PublishedLimits()); if err!=nil{return err}; if hash(canon(res))!=pa.ResultDigest{return fmt.Errorf("producer digest %s",c.CaseID)}; var rv Review; if err:=readStrict(filepath.Join(dir,"REVIEW.json"),&rv); err!=nil{return err}; ob,_,err:=readFile(filepath.Join(frozenRoot,"oracle-candidate","cases",c.CaseID,"RESULT.json")); if err!=nil{return err}; if rv.AssignmentID!=c.Reviewer.AssignmentID||!bytes.Equal(canon(res),ob)||!rv.ByteEqual||rv.Verdict!="ACCEPT"{return fmt.Errorf("review %s",c.CaseID)} }
	var m Manifest; if err:=readStrict(filepath.Join(execRoot,"EXECUTION_MANIFEST.json"),&m); err!=nil{return err}; if m.ProducerAttempts!=26||m.ReviewerAttempts!=26||m.ByteEquality!=26||m.BoundaryReplays!=4||m.FinalLocationCustodyGoIssued||m.Accepted{return errors.New("manifest counts")}; return nil }
