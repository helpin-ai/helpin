#!/usr/bin/env python3
"""Verify initial crawler metadata and the app-origin image proxy after deploy."""
import argparse
from html.parser import HTMLParser
import json
import struct
import sys
import time
from urllib.parse import urlencode, urlsplit
from urllib.request import Request, urlopen


CRAWLER = 'Slackbot-LinkExpanding 1.0 (+https://api.slack.com/robots)'
PATHS = ('/login', '/w/link-preview-check/pm/tasks/00000000-0000-0000-0000-000000000000')
META_KEYS = ('og:title', 'og:description', 'og:url', 'og:image', 'twitter:card', 'twitter:image')


class PreviewError(RuntimeError):
    pass


class Metadata(HTMLParser):
    def __init__(self, html):
        super().__init__(convert_charrefs=True)
        self.values = {}
        self.feed(html)

    def handle_starttag(self, tag, attrs):
        attrs = dict(attrs)
        key = attrs.get('property', attrs.get('name'))
        if tag == 'meta' and key:
            self.values.setdefault(key, []).append(attrs.get('content', ''))


def fetch(url):
    request = Request(url, headers={'User-Agent': CRAWLER, 'Cache-Control': 'no-cache'})
    with urlopen(request, timeout=10) as response:
        body = response.read(2 * 1024 * 1024 + 1)
        if len(body) > 2 * 1024 * 1024:
            raise PreviewError('Preview response exceeded 2 MiB')
        return response.headers, body


def check(app_origin, api_base, fetcher=fetch):
    for path in PATHS:
        query = urlencode({'path': path})
        _, api_body = fetcher(f'{api_base}/link-previews/metadata?{query}')
        payload = json.loads(api_body)
        expected = Metadata(payload['head']).values
        headers, html = fetcher(app_origin + path)
        actual = Metadata(html.decode('utf-8')).values
        for key in META_KEYS:
            values = expected.get(key, [])
            if len(values) != 1 or not values[0] or actual.get(key) != values:
                raise PreviewError(f'{path}: initial HTML does not contain the current {key}')
        if 'no-store' not in headers.get('Cache-Control', '').lower():
            raise PreviewError(f'{path}: initial HTML can be cached after workspace opt-out')

    # This endpoint exercises the edge function even when the API uses its own
    # public origin for og:image. A missing function otherwise returns SPA HTML.
    headers, png = fetcher(f'{app_origin}/api/link-previews/image?{urlencode({"path": "/login"})}')
    if headers.get('Content-Type', '').split(';')[0] != 'image/png':
        raise PreviewError('App image proxy did not return image/png')
    if len(png) < 24 or png[:8] != b'\x89PNG\r\n\x1a\n' or struct.unpack('>II', png[16:24]) != (1200, 630):
        raise PreviewError('App image proxy did not return a 1200 x 630 PNG')
    if 'no-store' not in headers.get('Cache-Control', '').lower():
        raise PreviewError('App image proxy can be cached after workspace opt-out')


def http_url(value):
    parsed = urlsplit(value)
    if parsed.scheme not in ('http', 'https') or not parsed.netloc or parsed.username or parsed.password or parsed.query or parsed.fragment:
        raise argparse.ArgumentTypeError('Expected an HTTP(S) URL without credentials, query, or fragment')
    return value.rstrip('/')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--app-url', required=True, type=http_url)
    parser.add_argument('--api-url', required=True, type=http_url)
    parser.add_argument('--attempts', type=int, default=1)
    parser.add_argument('--retry-delay', type=float, default=5)
    args = parser.parse_args()
    if args.attempts < 1 or args.retry_delay < 0:
        parser.error('attempts must be positive and retry-delay must be nonnegative')
    for attempt in range(1, args.attempts + 1):
        try:
            check(args.app_url, args.api_url)
            print('Link preview checks passed: initial metadata and app image proxy.')
            return 0
        except Exception as error:
            print(f'Link preview check {attempt}/{args.attempts} failed: {error}', file=sys.stderr)
            if attempt < args.attempts:
                time.sleep(args.retry_delay)
    return 1


if __name__ == '__main__':
    sys.exit(main())
