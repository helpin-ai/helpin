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
    "A refined, minimal product illustration for a B2B software brand, in the restrained style of Linear's marketing site. "
    "Vertical composition on a completely flat, uniform near-black background (#0A0B0B) that reaches every edge "
    "with no vignette, no gradients, no glow, no texture, no grain, no noise. "
    "{subject} The UI is large and fills about 80 percent of the frame width, centered vertically. "
    "UI elements are crisp, precise charcoal panels (#1A1D1C) with 1px hairline borders (#343937), small rounded corners, "
    "and clear mid-gray placeholder bars (#6B736F) instead of any writing, with enough contrast to read at small sizes. Strictly monochrome grays, "
    "with one restrained emerald accent (#0F7A50) used on a single small element only. "
    "Perfect alignment, generous empty space, calm, confident and corporate. "
    "Absolutely no text, no letters, no numbers, no words, no logos, no watermarks, no people, no faces, "
    "no flowing lines, no light rays, no particles, no sparkles."
)

SUBJECTS = {
    'home': "A single clean panel in the center shows one customer record: a small avatar circle at the top and a short vertical timeline of four rows beneath it, with thin connector lines to three smaller cards around it labeled only with placeholder bars.",
    'product': "Six small, evenly spaced square tiles arranged in a neat two-by-three grid, each with one simple line icon and a placeholder bar; one tile has the emerald accent.",
    'customer-support': "A conversation panel with three message rows, alternating left and right, and a small resolved status pill at the bottom with an emerald checkmark.",
    'projects': "A clean board with three columns of task cards; one card in the middle column is highlighted with an emerald left edge.",
    'crm': "A horizontal pipeline of four stage columns with a few deal cards in each; the rightmost column's top card carries the emerald accent.",
    'meetings': "A meeting panel with a thin monochrome audio waveform at the top and, beneath it, a short list of three action-item rows with checkboxes, one checked in emerald.",
    'knowledge': "A document panel with a title bar and several paragraph lines, a small search field above it, and one highlighted paragraph marked with a thin emerald bar.",
    'ai-agents': "A small central panel connected by thin straight gray lines to six smaller agent panels arranged evenly around it; one connection is emerald.",
    'developers': "A terminal-style panel with a title bar and several lines of placeholder code, beside a small card showing a pair of curly braces as a simple line icon; the cursor is emerald.",
    'self-hosting': "A single server icon inside a dashed rounded rectangle, with three thin gray lines leading out to small optional service tiles outside the boundary; the server's status dot is emerald.",
    'branding': "A tidy brand board: a row of four color swatch tiles (near-black, charcoal, light gray and one emerald), and beneath it a larger type-specimen panel showing only placeholder bars of different weights.",
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
