#!/usr/bin/env python3
import base64, importlib.util, json, shutil, tempfile, unittest
from pathlib import Path
HERE=Path(__file__).parent
spec=importlib.util.spec_from_file_location('validator',HERE/'validate_input_corpus.py'); v=importlib.util.module_from_spec(spec); spec.loader.exec_module(v)
class ValidatorTest(unittest.TestCase):
 def setUp(self):
  self.t=Path(tempfile.mkdtemp()); shutil.copytree(HERE/'inputs',self.t/'inputs'); shutil.copy2(HERE/'INPUT_CORPUS.json',self.t/'INPUT_CORPUS.json')
  self.old=(v.ROOT,v.INPUTS,v.CENSUS); v.ROOT=self.t; v.INPUTS=self.t/'inputs'; v.CENSUS=self.t/'INPUT_CORPUS.json'
 def tearDown(self): v.ROOT,v.INPUTS,v.CENSUS=self.old; shutil.rmtree(self.t)
 def test_accepts_corpus(self): self.assertEqual(v.validate(),0)
 def test_rejects_missing_case(self): shutil.rmtree(sorted(v.INPUTS.iterdir())[0]); self.assertEqual(v.validate(),1)
 def test_rejects_noncanonical_base64(self):
  p=v.INPUTS/'01-exact-intersects'/'BINDING.json'; x=json.loads(p.read_bytes()); x['binding']['sources'][0]['bytes']='YQ'; p.write_text(json.dumps(x,separators=(',',':'))+'\n'); self.assertEqual(v.validate(),1)
 def test_rejects_stale_census(self):
  p=v.INPUTS/'01-exact-intersects'/'CASE.json'; p.write_bytes(p.read_bytes()+b' '); self.assertEqual(v.validate(),1)
if __name__=='__main__': unittest.main()
