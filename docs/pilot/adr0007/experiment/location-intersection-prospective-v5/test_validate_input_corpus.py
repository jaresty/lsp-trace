#!/usr/bin/env python3
import importlib.util, json, shutil, tempfile, unittest
from pathlib import Path
HERE=Path(__file__).parent
spec=importlib.util.spec_from_file_location('validator',HERE/'validate_input_corpus.py'); v=importlib.util.module_from_spec(spec); spec.loader.exec_module(v)
class ValidatorTest(unittest.TestCase):
 def setUp(self):
  self.t=Path(tempfile.mkdtemp()); shutil.copytree(HERE/'inputs',self.t/'inputs'); shutil.copy2(HERE/'INPUT_CORPUS.json',self.t/'INPUT_CORPUS.json')
  self.old=(v.ROOT,v.INPUTS,v.CENSUS); v.ROOT=self.t; v.INPUTS=self.t/'inputs'; v.CENSUS=self.t/'INPUT_CORPUS.json'
 def tearDown(self): v.ROOT,v.INPUTS,v.CENSUS=self.old; shutil.rmtree(self.t)
 def reject(self): self.assertEqual(v.validate(),1)
 def test_accepts_corpus(self): self.assertEqual(v.validate(),0)
 def test_rejects_wrong_count(self): shutil.rmtree(v.INPUTS/'30-deadline-only'); self.reject()
 def test_rejects_stale_census(self): p=v.INPUTS/'01-exact-intersects'/'CASE.json'; p.write_bytes(p.read_bytes()+b' '); self.reject()
 def test_rejects_noncanonical_json(self): p=v.INPUTS/'01-exact-intersects'/'CASE.json'; x=json.loads(p.read_bytes()); p.write_text(json.dumps(x,indent=2)+'\n'); self.reject()
 def test_rejects_field_order(self): p=v.INPUTS/'01-exact-intersects'/'CASE.json'; x=json.loads(p.read_bytes()); p.write_text(json.dumps({'caseId':x['caseId'],'schema':x['schema'],'causalPerturbation':x['causalPerturbation'],'specCitations':x['specCitations']},separators=(',',':'))+'\n'); self.reject()
 def test_rejects_request_schema(self): p=v.INPUTS/'01-exact-intersects'/'REQUEST.raw.json'; x=json.loads(p.read_bytes()); x['unexpected']=True; p.write_text(json.dumps(x,separators=(',',':'))+'\n'); self.reject()
 def test_rejects_binding_schema(self): p=v.INPUTS/'01-exact-intersects'/'BINDING.json'; x=json.loads(p.read_bytes()); del x['binding']['schema']; p.write_text(json.dumps(x,separators=(',',':'))+'\n'); self.reject()
 def test_rejects_case_metadata_schema(self): p=v.INPUTS/'01-exact-intersects'/'CASE.json'; x=json.loads(p.read_bytes()); x['specCitations']=[]; p.write_text(json.dumps(x,separators=(',',':'))+'\n'); self.reject()
 def test_rejects_condition_metadata_schema(self): p=v.INPUTS/'01-exact-intersects'/'CONDITION.json'; x=json.loads(p.read_bytes()); x['cancel']='false'; p.write_text(json.dumps(x,separators=(',',':'))+'\n'); self.reject()
 def test_rejects_absent_bytes(self): (v.INPUTS/'09-binding-absent'/'BINDING.ABSENT').write_bytes(b'ABSENT'); self.reject()
 def test_rejects_prohibited_filename(self): (v.INPUTS/'01-exact-intersects'/'ORACLE.json').write_text('{}\n'); self.reject()
 def test_rejects_prohibited_content(self): p=v.INPUTS/'01-exact-intersects'/'CASE.json'; x=json.loads(p.read_bytes()); x['causalPerturbation']='V5_EVALUATOR'; p.write_text(json.dumps(x,separators=(',',':'))+'\n'); self.reject()
 def test_rejects_noncanonical_base64(self): p=v.INPUTS/'01-exact-intersects'/'BINDING.json'; x=json.loads(p.read_bytes()); x['binding']['sources'][0]['bytes']='YQ'; p.write_text(json.dumps(x,separators=(',',':'))+'\n'); self.reject()
 def test_rejects_typed_branch_mismatch(self): p=v.INPUTS/'11-typed-invalid-request'/'BINDING.json'; x=json.loads(p.read_bytes()); x['input']=[json.loads((v.INPUTS/'12-typed-invalid-source'/'BINDING.json').read_bytes())['input'][0]]; x['input'][0]['fileDigest']=x['input'][0]['objectDigest']; p.write_text(json.dumps(x,separators=(',',':'))+'\n'); self.reject()
 def test_rejects_wrong_malformed_class(self): (v.INPUTS/'08-malformed-json'/'REQUEST.raw.json').write_bytes(b'{'); self.reject()
if __name__=='__main__': unittest.main()
