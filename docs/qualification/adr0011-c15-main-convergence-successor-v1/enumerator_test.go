package sessionruntime

import (
 "bytes"
 "crypto/sha256"
 "encoding/csv"
 "fmt"
 "go/ast"
 "go/build"
 "go/parser"
 "go/token"
 "os"
 "path/filepath"
 "sort"
 "strings"
 "testing"
)

// TestADR0011C15MainConvergenceSuccessorEnumerate is the retained, reproducible
// candidate-specific enumerator. The regeneration script installs this file
// temporarily into sessionruntime, runs it twice, and removes it.
func TestADR0011C15MainConvergenceSuccessorEnumerate(t *testing.T) {
 out := os.Getenv("ADR0011_C15_SUCCESSOR_OUTPUT")
 if out == "" { t.Fatal("ADR0011_C15_SUCCESSOR_OUTPUT unset") }
 entries, err := os.ReadDir(out)
 if err != nil || len(entries) != 0 { t.Fatalf("output unavailable/nonempty: err=%v entries=%d", err, len(entries)) }
 mapping := readPrivateB4C15Mapping(t)
 if len(mapping.Rows) != 44 || mapping.Counts.Total != 44 { t.Fatalf("mapping rows=%d total=%d want=44", len(mapping.Rows), mapping.Counts.Total) }
 repo := filepath.Dir(packageDir(t))
 if _, err := privateC15VerifyPersistedMapping(repo, mapping, privateC15SemanticContracts, nil); err != nil { t.Fatal(err) }
 discovered1 := discoverPrivateB4C15Candidates(t)
 candidates1, classes1 := generatePrivateB4C15Catalog(t, mapping)
 discovered2 := discoverPrivateB4C15Candidates(t)
 candidates2, classes2 := generatePrivateB4C15Catalog(t, mapping)
 if !bytes.Equal(candidates1,candidates2) || !bytes.Equal(classes1,classes2) || len(discovered1)!=len(discovered2) { t.Fatal("nondeterministic canonical enumeration") }
 if len(discovered1)+len(mapping.Rows) != 4155 { t.Fatalf("rows=%d discovered=%d mapping=%d want=4155",len(discovered1)+len(mapping.Rows),len(discovered1),len(mapping.Rows)) }
 ids:=map[string]bool{}
 for _,c:=range discovered1 { if ids[c.ID] { t.Fatalf("duplicate candidate id=%s",c.ID) }; ids[c.ID]=true }
 for _,r:=range mapping.Rows { if ids[r.SiteID] { t.Fatalf("duplicate mapping id=%s",r.SiteID) }; ids[r.SiteID]=true }
 if len(ids)!=4155 { t.Fatalf("unique=%d want=4155",len(ids)) }
 locations:=successorEnumerateLocations(t,repo,discovered1)
 var b bytes.Buffer
 w:=csv.NewWriter(&b); w.Comma='\t'
 _=w.Write([]string{"candidate_id","source_path","source_span","enclosing","ast_kind","display","ast_digest","provenance"})
 for _,c:=range discovered1 { l:=locations[c.ID]; _=w.Write([]string{c.ID,"sessionruntime/"+l.path,l.span,l.enclosing,c.Kind,c.Display,c.Digest,"canonical-production-enumeration"}) }
 rows:=append([]privateB4C15MappingRow(nil),mapping.Rows...)
 sort.Slice(rows,func(i,j int)bool{return rows[i].SiteID<rows[j].SiteID})
 for _,r:=range rows { _=w.Write([]string{r.SiteID,r.Path,"verified-operation",r.BindingFunction,r.OperationKind,r.LogicalSeam,r.ASTDigest,"accepted-44-row-allocation-mapping"}) }
 w.Flush(); if w.Error()!=nil { t.Fatal(w.Error()) }
 outputs:=map[string][]byte{"candidates.tsv":candidates1,"classifications.tsv":classes1,"all-source-locations.tsv":b.Bytes()}
 for name,data:=range outputs {
  if err:=os.WriteFile(filepath.Join(out,name),data,0644);err!=nil{t.Fatal(err)}
  side:=fmt.Sprintf("%x\n",sha256.Sum256(data)); if err:=os.WriteFile(filepath.Join(out,name+".sha256"),[]byte(side),0644);err!=nil{t.Fatal(err)}
 }
 t.Logf("SUCCESS rows=4155 duplicates=0 mapping=44 locations=4155")
}

type successorEnumerationLocation struct{ path,span,enclosing string }
func successorEnumerateLocations(t *testing.T,repo string,canonical []privateB4C15Candidate) map[string]successorEnumerationLocation {
 dir:=filepath.Join(repo,"sessionruntime"); entries,err:=os.ReadDir(dir);if err!=nil{t.Fatal(err)}
 var names []string
 for _,e:=range entries { if e.IsDir()||!strings.HasSuffix(e.Name(),".go")||strings.HasSuffix(e.Name(),"_test.go"){continue}; ok,err:=build.Default.MatchFile(dir,e.Name());if err!=nil{t.Fatal(err)};if ok{names=append(names,e.Name())} }
 sort.Strings(names)
 byID:=map[string]privateB4C15Candidate{};for _,c:=range canonical{byID[c.ID]=c}
 out:=map[string]successorEnumerationLocation{};counts:=map[string]int{}
 for _,name:=range names {
  src,err:=os.ReadFile(filepath.Join(dir,name));if err!=nil{t.Fatal(err)}
  fs:=token.NewFileSet();f,err:=parser.ParseFile(fs,name,src,parser.AllErrors);if err!=nil{t.Fatal(err)}
  fileKey:=strings.TrimSuffix(strings.ReplaceAll(name,"_","."),".go")
  ast.Inspect(f,func(n ast.Node)bool{
   var kind,display string
   switch x:=n.(type){case *ast.CallExpr: display=privateB4C15ExprName(x.Fun);kind="call";if id,ok:=x.Fun.(*ast.Ident);ok&&(id.Name=="make"||id.Name=="append"){kind="builtin-"+id.Name};case *ast.CompositeLit:kind,display="composite-literal","composite.literal";default:return true}
   enclosing:=strings.ToLower(enclosingFunction(f,n.Pos()));base:="alloc."+fileKey+"."+enclosing+"."+kind+"."+strings.ToLower(strings.ReplaceAll(display,"_","."));counts[base]++;id:=fmt.Sprintf("%s.%d",base,counts[base])
   c,ok:=byID[id];if !ok{t.Fatalf("location noncanonical id=%s",id)}
   digest:=fmt.Sprintf("%x",sha256.Sum256([]byte(normalizedNodeSource(fs,src,n))));if digest!=c.Digest{t.Fatalf("digest mismatch id=%s",id)}
   sp,ep:=fs.Position(n.Pos()),fs.Position(n.End());out[id]=successorEnumerationLocation{name,fmt.Sprintf("%d:%d-%d:%d",sp.Line,sp.Column,ep.Line,ep.Column),enclosing};return true
  })
 }
 if len(out)!=len(canonical){t.Fatalf("locations=%d want=%d",len(out),len(canonical))};return out
}
