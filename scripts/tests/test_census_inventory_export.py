"""Export-only PRIVATE_CENSUS_AST_V2 bindings; never a v4 launch config."""
import os
from pathlib import Path
import tempfile
import unittest
import test_private_variable_v2_handoff as fixtures

h = fixtures.h

class CensusExportTest(unittest.TestCase):
    def fixture(self, base):
        owner = fixtures.HandoffTest()
        f = owner.export_fixture(base)
        old = h.decode(f['config'].read_bytes())
        prep_path = f['mailbox'] / 'preparation.json'
        prep = h.decode(prep_path.read_bytes())
        prep['ScopeSHA256'] = h.digest(b'owned-census-denominator')
        prep_path.write_bytes(h.canonical(prep))
        binding = dict(Schema='private-census-inventory-host-export.v1', Profile='PRIVATE_CENSUS_AST_V2',
                       Protocol=h.PROTOCOL, Authority=0, Accepted=False, Qualified=False,
                       FeatureIdentity='UNRESOLVED', SourceCompleteness='UNKNOWN', AbsenceInference='PROHIBITED',
                       ScopeSHA256=prep['ScopeSHA256'], PreparationSHA256=prep['PreparationSHA256'],
                       Automation=dict(**old['automation'], ScopeSHA256=prep['ScopeSHA256']),
                       Roots=[old['scope']['root'], old['operation_directory']])
        self.save(f, binding)
        return owner, f, binding

    def save(self, f, binding):
        f['config'].write_bytes(h.canonical(binding))
        f['pin'] = h.digest(f['config'].read_bytes())

    def test_census_export_exact_and_launch_refused(self):
        with tempfile.TemporaryDirectory(prefix='.owned-census-export-', dir=Path.home()) as base:
            owner, f, _ = self.fixture(base)
            owner.assert_export_exact(f, owner.export(f))
            with self.assertRaisesRegex(ValueError, 'only local handoff v4'):
                h.config(str(f['config']), f['pin'])
            print('CENSUS_EXPORT_ONLY: PASS exact parts; not a v4 launch configuration')

    def test_census_export_family_and_scope_refusals(self):
        for mutation in ('qualified-profile', 'qualified', 'accepted', 'authority', 'source', 'scope', 'preparation'):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory(prefix='.owned-census-export-', dir=Path.home()) as base:
                owner, f, binding = self.fixture(base)
                if mutation == 'qualified-profile': binding['Profile'] = 'CALLS_ONLY_FROZEN_CENSUS_V1'
                elif mutation == 'qualified': binding['Qualified'] = True
                elif mutation == 'accepted': binding['Accepted'] = True
                elif mutation == 'authority': binding['Authority'] = 1
                elif mutation == 'source': binding['SourceCompleteness'] = 'COMPLETE'
                elif mutation == 'scope': binding['ScopeSHA256'] = h.digest(b'foreign scope')
                else: binding['PreparationSHA256'] = h.digest(b'foreign preparation')
                self.save(f, binding)
                with self.assertRaises(ValueError): owner.export(f)
                self.assertFalse(f['dest'].exists())
                print('CENSUS_EXPORT_REFUSAL: PASS ' + mutation)

if __name__ == '__main__':
    unittest.main()
