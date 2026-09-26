# Browser test references

[Documentation index](../README.md)

For maintained widget browser tests, use the
[SDK widget test guide](../../packages/sdk-js/test/e2e/widget/README.md).
That suite provides deterministic mocked API and storage responses.

The [historical support-widget test page](support-widget-test.html) is a manual
fixture with a hard-coded development host, widget key, and hosted CDN loader.
It is not a ready-to-run local test or evidence that the old host is available.
Before loading it, replace both the script attributes and the configuration-fetch
URL with your own test installation. Opening it unchanged makes external requests.
