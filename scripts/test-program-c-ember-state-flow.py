#!/usr/bin/env python3
import importlib.util, json, subprocess, tempfile, unittest
from pathlib import Path
ROOT=Path(__file__).resolve().parents[1]
SCRIPT=ROOT/'scripts/qualify-program-c-ember-state-flow.py'
spec=importlib.util.spec_from_file_location('qualifier',SCRIPT); q=importlib.util.module_from_spec(spec); spec.loader.exec_module(q)
REV='d955fe0d403356450c5372b23a61ba8e2de4cd5d'
def prepublication_matrix():
 text=(ROOT/'qualification/program-c/profile-qualification.tsv').read_text()
 for line in text.splitlines():
  if line.startswith('state-flow-v1\ttypescript\tember\tember-glint\t1.0.3\t'):
   fields=line.split('\t'); fields[6:]=['MISSING','-','-']; return text.replace(line,'\t'.join(fields)).encode()
 raise AssertionError('target matrix row missing')
class QualificationTest(unittest.TestCase):
 def test_positive_deterministic_strict_receipt(self):
  matrix=prepublication_matrix()
  a=q.build(ROOT,REV,matrix); b=q.build(ROOT,REV,matrix)
  self.assertEqual(a,b,'ASSERT_PROGRAM_C_EMBER_STATE_FLOW_DETERMINISTIC')
  self.assertEqual([e['relation'] for e in a['relation_evidence']],list(q.RELATIONS),'ASSERT_PROGRAM_C_EMBER_STATE_FLOW_EXACT_RELATIONS')
  self.assertTrue(all(e['count']>0 and e['custody']=='PROVIDER_PROVED' and e['replay']=='EXACT_BYTES' for e in a['relation_evidence']),'ASSERT_PROGRAM_C_EMBER_STATE_FLOW_PROVIDER_PROVED_REPLAY')
  print('ASSERT_PROGRAM_C_EMBER_STATE_FLOW_RECEIPT result=PASS')
 def test_fail_closed_tampered_prepublication_matrix(self):
  matrix=(ROOT/'qualification/program-c/profile-qualification.tsv').read_text().replace('\tyes\tMISSING\t-\t-','\tyes\tPASS\tx\tsha256:'+'0'*64,1).encode()
  with self.assertRaisesRegex(ValueError,'pre-publication MISSING'):
   q.build(ROOT,REV,matrix)
  print('ASSERT_PROGRAM_C_EMBER_STATE_FLOW_PREPUBLICATION result=FAIL expected=true')
 def test_fail_closed_wrong_revision(self):
  with tempfile.TemporaryDirectory() as d:
   result=subprocess.run(['python3',str(SCRIPT),'--repository-revision','wrong','--output',str(Path(d)/'r.json')],text=True,capture_output=True)
  self.assertNotEqual(result.returncode,0,'ASSERT_PROGRAM_C_EMBER_STATE_FLOW_REVISION_FAIL_CLOSED')
  print('ASSERT_PROGRAM_C_EMBER_STATE_FLOW_REVISION result=FAIL expected=true')
if __name__=='__main__': unittest.main()
