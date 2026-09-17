import tempfile
from pathlib import Path
import unittest

from check_links import anchors, check


class LinkTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        (self.root / 'guide (one).md').write_text('# Hello, `world`!\n## Same\n## Same\n<a id="custom"></a>\n')

    def verify(self, content):
        (self.root / 'README.md').write_text(content)
        return check(self.root, ['README.md'])

    def test_inline_images_references_encoded_paths_and_titles(self):
        (self.root / 'image.png').write_bytes(b'fixture')
        count, errors = self.verify('''[guide](<guide (one).md#hello-world>)
[again](guide%20%28one%29.md#same-1 "title")
![image](image.png)
[custom][ref]
[ref]: <guide (one).md#custom>
[shortcut]
[shortcut]: guide%20%28one%29.md
''')
        self.assertEqual(errors, [])
        self.assertEqual(count, 5)

    def test_reports_missing_files_anchors_and_references(self):
        _, errors = self.verify('[bad](missing.md)\n[bad](guide%20%28one%29.md#absent)\n[bad][absent]\n')
        self.assertEqual(len(errors), 3)
        self.assertTrue(any('README.md:1: missing target' in e for e in errors))
        self.assertTrue(any('missing heading' in e for e in errors))
        self.assertTrue(any('undefined link reference' in e for e in errors))

    def test_ignores_examples_comments_and_external_urls(self):
        count, errors = self.verify('''```md
[example](missing.md)
```
~~~md
[example](missing.md)
~~~
`[example](missing.md)`
<!-- [example](missing.md) -->
[web](https://helpin.ai/docs)
[email](mailto:support@example.test)
[normal bracketed text]
''')
        self.assertEqual((count, errors), (0, []))

    def test_renamed_target_is_detected_without_editing_source(self):
        self.assertEqual(self.verify('[guide](<guide (one).md>)')[1], [])
        (self.root / 'guide (one).md').rename(self.root / 'renamed.md')
        self.assertIn('missing target', check(self.root, ['README.md'])[1][0])

    def test_rejects_escape_and_missing_document(self):
        self.assertIn('leaves repository', self.verify('[outside](../outside.md)')[1][0])
        self.assertIn('does not exist', check(self.root, ['missing.md'])[1][0])

    def test_heading_forms_and_duplicate_collisions(self):
        self.assertEqual(anchors('# A\n# A\n# A-1\nTitle\n=====\n## Über café\n'),
                         {'a', 'a-1', 'a-1-1', 'title', 'über-café'})


if __name__ == '__main__':
    unittest.main()
