"""Print the Helpin Slack app manifest to import in Slack's app settings."""
import argparse
from pathlib import Path
from urllib.parse import urlsplit

parser = argparse.ArgumentParser(description=__doc__)
group = parser.add_mutually_exclusive_group(required=True)
group.add_argument("--bootstrap", action="store_true", help="create the bot before activating callbacks")
group.add_argument("--app-origin", help="public Helpin origin for OAuth")
parser.add_argument("--api-base", help="public API base including its mount path (defaults to APP_ORIGIN/api)")
args = parser.parse_args()
template = Path(__file__).with_name("manifest.yaml").read_text()


def https_url(value, allow_path):
    value = value.rstrip("/")
    try:
        parts = urlsplit(value)
        port = parts.port
    except ValueError as error:
        parser.error(str(error))
    if (parts.scheme != "https" or not parts.hostname or parts.username is not None
            or parts.password is not None or parts.query or parts.fragment
            or (parts.path and not allow_path) or port == 0
            or any(c.isspace() or c in "\\\"'<>#" for c in value)):
        parser.error("use a public HTTPS URL without credentials, query, or fragment")
    return value


if args.bootstrap:
    if args.api_base:
        parser.error("--api-base belongs with --app-origin")
    # A new app must exist before Slack issues its signing secret.
    bot = template.split("  shortcuts:", 1)[0]
    print(bot + "oauth_config:\n  scopes:\n    bot:\n      - chat:write")
else:
    app = https_url(args.app_origin, False)
    api = https_url(args.api_base or app + "/api", True)
    print(template.replace("https://helpin.example.com/api/", api + "/")
          .replace("https://helpin.example.com/oauth/", app + "/oauth/"), end="")
