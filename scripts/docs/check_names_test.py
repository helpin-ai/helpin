import unittest

from check_names import check_names


class NamingTests(unittest.TestCase):
    def test_current_guides_and_historical_names(self):
        self.assertEqual(check_names([
            'docs/README.md', 'docs/community/README.md', 'docs/agent-runtime-local-setup.md',
            'docs/plans/2026-09-17-support-routing.md', 'docs/prds/support-live-chat.md',
        ]), [])

    def test_non_documentation_discovery_files_are_outside_scope(self):
        self.assertEqual(check_names(['AGENTS.md', 'ARCHITECTURE.md',
                                     '.agents/skills/helpin-documentation/SKILL.md',
                                     'docs/mockups/layout.html']), [])

    def test_rejects_uppercase_spaces_and_underscores(self):
        for name in ['docs/CRM_MODULE.md', 'docs/new guide.md', 'docs/Guide.MD']:
            with self.subTest(name=name):
                self.assertEqual(len(check_names([name])), 1)

    def test_rejects_redundant_type_and_status_prefixes(self):
        for name in ['docs/prds/prd-support.md', 'docs/plans/todo-support.md']:
            with self.subTest(name=name):
                self.assertEqual(len(check_names([name])), 1)

    def test_checks_date_validity_without_inventing_dates(self):
        self.assertEqual(check_names(['docs/plans/undated-plan.md',
                                     'docs/plans/2024-02-29-upgrade.md']), [])
        self.assertEqual(len(check_names(['docs/plans/2026-02-30-upgrade.md'])), 1)


if __name__ == '__main__':
    unittest.main()
