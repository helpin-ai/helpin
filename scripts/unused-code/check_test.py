import unittest
from unittest.mock import patch
import subprocess
import check


class AuditTests(unittest.TestCase):
    def test_new_symbol_fails_even_when_total_count_drops(self):
        self.assertEqual(check.compare({'go-ee': ['new']}, {'go-ee': ['old', 'older']}), {'go-ee': ['new']})

    def test_removed_findings_do_not_fail(self):
        self.assertEqual(check.compare({'go-ee': []}, {'go-ee': ['old']}), {})

    def test_locations_do_not_churn_baseline(self):
        fn = {'Name': 'Service.Old', 'Position': {'File': 'service.go', 'Line': 12}}
        first = check.go_findings([{'Funcs': [fn]}])
        fn['Position']['Line'] = 99
        self.assertEqual(first, check.go_findings([{'Funcs': [fn]}]))

    def test_knip_symbol_identity(self):
        self.assertEqual(check.knip_findings({'issues': [{'file': 'a.ts', 'exports': [{'name': 'unused', 'line': 4}]}]}), {'exports:a.ts:unused'})

    @patch('check.subprocess.run')
    def test_tool_failure_cannot_be_accepted_as_empty_baseline(self, run):
        for status, stderr in [(2, 'failed'), (0, 'ERROR: config failed')]:
            run.return_value = subprocess.CompletedProcess([], status, '{"issues": []}', stderr)
            with self.assertRaises(RuntimeError):
                check.run_json(['knip'], check.ROOT, {})


if __name__ == '__main__':
    unittest.main()
