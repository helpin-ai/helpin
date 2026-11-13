import json
import struct
import unittest

from check_link_previews import META_KEYS, Metadata, PreviewError, check


class LinkPreviewCheckTest(unittest.TestCase):
    def fixture(self, generic_html=False, html_image=False, cacheable=False):
        head = ''.join(f'<meta property="{key}" content="current-{key}">' for key in META_KEYS)
        headers = {'Content-Type': 'text/html', 'Cache-Control': 'public' if cacheable else 'private, no-store'}

        def fetch(url):
            if '/link-previews/metadata?' in url:
                return {}, json.dumps({'head': head}).encode()
            if '/link-previews/image?' in url:
                if html_image:
                    return headers, b'<html>app shell</html>'
                return {'Content-Type': 'image/png', 'Cache-Control': 'private, no-store'}, b'\x89PNG\r\n\x1a\n' + b'\0' * 8 + struct.pack('>II', 1200, 630)
            return headers, (head.replace('current-og:title', 'generic') if generic_html else head).encode()

        return fetch

    def test_accepts_matching_metadata_and_image(self):
        check('https://app.example.test', 'https://api.example.test/api', self.fixture())

    def test_rejects_successful_html_response_with_generic_metadata(self):
        with self.assertRaisesRegex(PreviewError, 'current og:title'):
            check('https://app.example.test', 'https://api.example.test/api', self.fixture(generic_html=True))

    def test_rejects_spa_html_from_image_proxy(self):
        with self.assertRaisesRegex(PreviewError, 'image/png'):
            check('https://app.example.test', 'https://api.example.test/api', self.fixture(html_image=True))

    def test_rejects_cacheable_metadata(self):
        with self.assertRaisesRegex(PreviewError, 'cached after workspace opt-out'):
            check('https://app.example.test', 'https://api.example.test/api', self.fixture(cacheable=True))

    def test_parser_preserves_duplicate_tags_and_decodes_entities(self):
        parsed = Metadata('<meta property="og:title" content="A &amp; B"><meta property="og:title" content="old">')
        self.assertEqual(parsed.values['og:title'], ['A & B', 'old'])


if __name__ == '__main__':
    unittest.main()
