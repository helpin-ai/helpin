"""Generate text-free artwork options for the website's social preview (OG) cards.

The artwork is generated once, reviewed by a person, then committed as
scripts/assets/og-art/<slug>.jpg. generate-og-images.mjs composites it into the
cards at build time, so builds never call the API. See scripts/assets/og-art/README.md.

Usage:
    OPENAI_API_KEY=... python3 scripts/generate-og-art.py [--model M] [--n N] <slug> [<slug> ...]

Options are written to og-art-options/<slug>-<i>.png (git-ignored scratch output).
"""
import argparse, base64, json, os, sys, urllib.error, urllib.request

STYLE = (
    "A premium, restrained editorial illustration for a software brand, vertical composition. "
    "Deep forest-green background (#0B2119) with a subtle lighter green radial glow. "
    "Fine flowing line bundles in mint green (#9CDBB3) and emerald (#0F7A50), like smooth streamlines in a vortex, "
    "curving gently through the frame with soft luminous highlights. "
    "{subject} "
    "UI elements are minimal frosted-glass cards with rounded corners, thin mint outlines, and blank horizontal bars instead of any writing. "
    "Generous negative space, calm and precise, high-end SaaS brand aesthetic, subtle film grain. "
    "Absolutely no text, no letters, no numbers, no words, no logos, no watermarks, no people, no faces."
)

SUBJECTS = {
    'home': "Many thin flowing lines from all edges converge into one softly glowing core in the middle of the frame, suggesting a single shared customer history.",
    'pricing': "Only flowing line bundles, no cards or objects: a calm wave of lines sweeps across the lower third and the top edge, leaving a large, empty, softly lit dark area in the middle of the frame.",
    'product': "Six small frosted-glass tiles float in a loose ring, each connected by a fine glowing line to a single bright core at the center.",
    'customer-support': "A vertical stack of three blank chat bubbles follows a glowing path that ends at a small circular checkmark badge, suggesting a request that was resolved.",
    'projects': "Three slim kanban columns of blank task cards; one glowing line runs from a small chat bubble at the top into one highlighted card, linking a customer request to the work.",
    'crm': "A left-to-right sequence of four rounded pipeline stages with small deal cards moving through them, and a soft pulse ring radiating from the most advanced card.",
    'meetings': "A smooth audio waveform on the upper part of the frame flows downward and transforms into a neat stack of three blank task cards.",
    'knowledge': "A layered stack of blank document pages, with a soft vertical beam of light passing through them like a search highlighting one page.",
    'ai-agents': "Eight small glowing orbs travel on elegant elliptical orbits around one brighter central core, like coordinated specialists.",
    'developers': "A frosted-glass terminal window with blank lines of code, a pair of curly-brace shapes floating nearby, and a small plug connector linked by a glowing line.",
    'self-hosting': "A compact server block inside a dashed rounded boundary, with a few fine lines reaching out past the boundary to small optional nodes outside it.",
    'branding': "A single elegant vortex of flowing mint lines spiraling around a calm, softly glowing center.",
}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--model', default='gpt-image-2.5-sunburst')
    parser.add_argument('--n', type=int, default=2)
    parser.add_argument('slugs', nargs='+', choices=sorted(SUBJECTS))
    args = parser.parse_args()
    key = os.environ.get('OPENAI_API_KEY')
    if not key:
        sys.exit('Set OPENAI_API_KEY')
    os.makedirs('og-art-options', exist_ok=True)
    for slug in args.slugs:
        payload = {'model': args.model, 'prompt': STYLE.format(subject=SUBJECTS[slug]), 'n': args.n,
                   'size': '1024x1536', 'quality': 'medium', 'output_format': 'png'}
        req = urllib.request.Request('https://api.openai.com/v1/images/generations', data=json.dumps(payload).encode(),
                                     headers={'Content-Type': 'application/json', 'Authorization': f'Bearer {key}'})
        try:
            result = json.load(urllib.request.urlopen(req, timeout=300))
        except urllib.error.HTTPError as err:
            print(f'{slug}: HTTP {err.code}: {err.read().decode()[:300]}')
            continue
        for i, item in enumerate(result['data'], 1):
            path = f'og-art-options/{slug}-{i}.png'
            with open(path, 'wb') as f:
                f.write(base64.b64decode(item['b64_json']))
            print(f'saved {path}')


if __name__ == '__main__':
    main()
