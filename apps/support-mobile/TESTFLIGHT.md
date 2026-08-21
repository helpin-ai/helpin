# TestFlight release setup

The support mobile app is uploaded through the manually triggered
`Deploy Support Mobile to TestFlight` GitHub Actions workflow. The workflow
runs on GitHub's `macos-15` runner, builds a signed IPA against staging, checks
it with App Store Connect, and uploads it to TestFlight.

## One-time Apple setup

These steps require an active Apple Developer Program membership. Use the
explicit bundle identifier already configured in `src-tauri/tauri.conf.json`:

```text
ai.helpin.mobile
```

1. In [Certificates, Identifiers & Profiles](https://developer.apple.com/account/resources/identifiers/list),
   register an explicit App ID with bundle ID `ai.helpin.mobile`. Enable Push
   Notifications now if the first beta should exercise native push. Apple's
   detailed instructions are in [Register an App ID](https://developer.apple.com/help/account/identifiers/register-an-app-id).
2. In [App Store Connect](https://appstoreconnect.apple.com/apps), create an
   iOS app record. Select the registered bundle ID, use `Helpin` as the name,
   and choose a stable internal SKU such as `helpin-mobile-ios`. See Apple's
   [Add a new app](https://developer.apple.com/help/app-store-connect/create-an-app-record/add-a-new-app)
   instructions. The Account Holder must accept any pending agreements first.
3. On a Mac, use Keychain Access to create a certificate signing request as
   described in [Create a certificate signing request](https://developer.apple.com/help/account/certificates/create-a-certificate-signing-request).
4. In Certificates, Identifiers & Profiles, create an **Apple Distribution**
   certificate from that request. Download it and double-click it so the
   certificate is installed beside its private key in Keychain Access. Apple
   explains the certificate types in its [Certificates overview](https://developer.apple.com/help/account/create-certificates/certificates-overview).
5. In Keychain Access, open **My Certificates**, expand the Apple Distribution
   certificate, select the certificate and private key together, and choose
   **File > Export Items**. Export a password-protected `.p12` file. Apple's
   [Keychain export guide](https://support.apple.com/guide/keychain-access/import-and-export-keychain-items-kyca35961/mac)
   covers the export flow.
6. Create an **App Store Connect** provisioning profile for
   `ai.helpin.mobile`, selecting the Apple Distribution certificate created
   above. Download the resulting `.mobileprovision` file. Follow Apple's
   [Create an App Store Connect provisioning profile](https://developer.apple.com/help/account/provisioning-profiles/create-an-app-store-provisioning-profile/)
   instructions.
7. In App Store Connect, open **Users and Access > Integrations > Team Keys**.
   Generate an API key with at least the Developer role, record its Issuer ID
   and Key ID, and download its `.p8` private key. Apple allows the private key
   to be downloaded only once. See [App Store Connect API](https://developer.apple.com/help/app-store-connect/get-started/app-store-connect-api).

## Encode the signing files

On the Mac containing the exported files, copy each value to the clipboard:

```bash
# IOS_CERTIFICATE
base64 -i HelpinDistribution.p12 | pbcopy

# IOS_MOBILE_PROVISION
base64 -i Helpin_AppStore.mobileprovision | pbcopy

# APPLE_API_PRIVATE_KEY (store the original PEM text, not base64)
cat AuthKey_XXXXXXXXXX.p8 | pbcopy
```

Keep the `.p12`, its password, provisioning profile, and `.p8` key outside the
repository. Revoke and replace them immediately if they are exposed.

## Configure the GitHub environment

In the repository, open **Settings > Environments**, create an environment
named `testflight`, and add a required reviewer. Restrict deployment branches
to the branch that is allowed to publish mobile builds. GitHub's
[environment setup guide](https://docs.github.com/en/actions/how-tos/deploy/configure-and-manage-deployments/manage-environments)
explains secrets, variables, reviewers, and branch restrictions.

Add these environment **secrets**:

| Secret | Value |
| --- | --- |
| `APPLE_API_ISSUER` | Issuer ID shown above the App Store Connect team-key table |
| `APPLE_API_KEY_ID` | Key ID for the downloaded `AuthKey_*.p8` |
| `APPLE_API_PRIVATE_KEY` | Complete PEM content of the `.p8` file, including the BEGIN/END lines |
| `IOS_CERTIFICATE` | Base64-encoded Apple Distribution `.p12` file |
| `IOS_CERTIFICATE_PASSWORD` | Password chosen when exporting the `.p12` file |
| `IOS_MOBILE_PROVISION` | Base64-encoded App Store Connect `.mobileprovision` file |

Add these environment **variables** if the defaults are not correct:

| Variable | Default |
| --- | --- |
| `SUPPORT_MOBILE_API_URL` | `https://stage.helpin.ai/api` |
| `SUPPORT_MOBILE_WEB_APP_URL` | `https://stage.helpin.ai` |

The `VITE_*` values are compiled into the app and are therefore public
configuration, not secrets.

## Run a release

The workflow must exist on the repository's default branch before GitHub shows
its **Run workflow** button. After this file is merged:

1. Open **Actions > Deploy Support Mobile to TestFlight**.
2. Select **Run workflow** and choose the approved release branch. Leave the
   build-number override empty unless App Store Connect already contains a
   build number greater than the workflow's run number.
3. Approve the `testflight` environment deployment when prompted.
4. Follow the build until `Upload IPA to TestFlight` succeeds.
5. Wait for Apple to process the build, then open the app's **TestFlight** tab
   in App Store Connect and complete any export-compliance questions.
6. Add the build to an internal testing group. Apple documents this in
   [TestFlight overview](https://developer.apple.com/help/app-store-connect/test-a-beta-version/testflight-overview)
   and [Add testers to builds](https://developer.apple.com/help/app-store-connect/test-a-beta-version/add-testers-to-builds).

The workflow normally uses the monotonically increasing GitHub run number as
the iOS build number, so repeated uploads of app version `0.1.0` remain
unique. The optional override accepts a positive integer for an existing App
Store Connect record whose latest build is higher. A signed IPA is retained
as a workflow artifact for 14 days.

## Native push limitation

The first TestFlight build can be used to review the app without Firebase push
configuration; the native plugin disables itself when
`GoogleService-Info.plist` is absent. Before testing push notifications, add
the Firebase plist securely during the build, enable Push Notifications and
Background Modes in the generated Xcode project, wire the APNs token callback,
and verify delivery on a physical iPhone. Those steps are tracked separately
in `src-tauri/tauri-plugin-helpin-push/README.md`.
