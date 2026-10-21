#!/usr/bin/env python3
import json, subprocess, tempfile, unittest
from pathlib import Path
ROOT=Path(__file__).resolve().parents[1]; SCRIPT=ROOT/'scripts/map-ci-affected-packages.py'
META={"packages":[{"ImportPath":"m/a","Dir":"/repo/a","Deps":[]},{"ImportPath":"m/b","Dir":"/repo/b","Deps":["m/a"]}],"manifests":{"linux":["m/a","m/b"],"darwin":["m/a","m/b"]}}
class TestMapper(unittest.TestCase):
 def map_changes(self, changed, event='pull_request'):
  with tempfile.TemporaryDirectory() as d:
   p=Path(d)/'m.json'; p.write_text(json.dumps(META))
   r=subprocess.run(['python3',str(SCRIPT),'--metadata',str(p),'--repo-root','/repo','--event',event,*sum((['--changed',x] for x in changed),[])],text=True,stdout=subprocess.PIPE,check=True)
   return json.loads(r.stdout)
 def test_reverse_dependencies(self):
  self.assertEqual(self.map_changes(['a/x.go'])['platforms']['linux'],['m/a','m/b'])
 def test_forced_full_and_uncertain(self):
  self.assertEqual(self.map_changes(['go.mod'])['mode'],'full')
  self.assertEqual(self.map_changes(['unknown/file.txt'])['mode'],'full')
  self.assertEqual(self.map_changes(['a/x.go'],event='merge_group')['mode'],'full')
if __name__=='__main__': unittest.main()
