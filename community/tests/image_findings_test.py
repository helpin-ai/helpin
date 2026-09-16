import datetime
import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location('findings', Path(__file__).resolve().parents[1] / 'verify-image-findings.py')
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)

class FindingsTest(unittest.TestCase):
    def test_exact_pin_version_and_expiry_are_required(self):
        image = 'upstream:1@sha256:' + 'a'*64
        report = {'Results': [{'Vulnerabilities': [{'VulnerabilityID': 'CVE-fixture', 'PkgName': 'fixture', 'InstalledVersion': '1', 'Severity': 'HIGH', 'FixedVersion': '2'}]}]}
        policy = {image: {'expires': '2026-10-16', 'findings': [{'id': 'CVE-fixture', 'package': 'fixture', 'installed': '1'}]}}
        today = datetime.date(2026, 9, 16)
        self.assertEqual(module.unreviewed(image, report, policy, today), ([], 1))
        for ref, date in [('upstream:1', today), (image+'f', today), (image, datetime.date(2026,10,17))]:
            self.assertEqual(len(module.unreviewed(ref, report, policy, date)[0]), 1)
        report['Results'][0]['Vulnerabilities'][0]['InstalledVersion'] = '1.1'
        self.assertEqual(len(module.unreviewed(image, report, policy, today)[0]), 1)

if __name__ == '__main__': unittest.main()
