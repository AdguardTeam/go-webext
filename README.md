# go-webext

A CLI tool for managing browser extensions across multiple web stores:
Chrome Web Store, Firefox Add-ons (AMO), and Microsoft Edge Add-ons.
It supports uploading, updating, publishing, signing, and checking the status
of extensions via each store's API.

## Contents

- [Installation](#installation)
- [Credentials](#credentials)
- [Environment Variables](#environment-variables)
- [Usage](#usage)
  - [Global Options](#global-options)
  - [Commands](#commands)
  - [Examples](#examples)
- [Documentation](#documentation)

## Installation

```bash
go install github.com/adguardteam/go-webext@latest
```

To install a specific version:

```bash
go install github.com/adguardteam/go-webext@v0.4.1
```

## Credentials

Before using the CLI, obtain API credentials for each store you plan to use.

### Chrome Web Store

Follow the guide at <https://developer.chrome.com/docs/webstore/using_webstore_api/>
to obtain `CODE`, `CLIENT_ID`, and `CLIENT_SECRET`.

Then get a `refresh_token`:

```bash
curl "https://accounts.google.com/o/oauth2/token" -d \
"client_id=${CLIENT_ID}&client_secret=${CLIENT_SECRET}&code=${CODE}&grant_type=authorization_code&redirect_uri=urn:ietf:wg:oauth:2.0:oob"
```

**Note**: Use the `refresh_token`, not the `access_token`.

Set up the publisher ID in the Chrome Developer Dashboard if you use the v2 API.

### Firefox Add-ons (AMO)

Log in to your Firefox account and generate API credentials at
<https://addons.mozilla.org/en-US/developers/addon/api/key/>.

### Microsoft Edge Add-ons

Two API versions are supported:

- **v1.1 (recommended)** — uses API Key authentication.
- **v1.0 (deprecated)** — uses Client Secret and Access Token URL. Microsoft is
  deprecating secrets in favor of API keys, and the key expiration will reduce
  from 2 years to 72 days.

For v1.1 credentials setup, visit:
<https://learn.microsoft.com/en-us/microsoft-edge/extensions-chromium/publish/api/using-addons-api?tabs=v1-1>.

## Environment Variables

Credentials are loaded from a `.env` file in the working directory or from
environment variables directly.

### Chrome

```dotenv
CHROME_CLIENT_ID=<client_id>
CHROME_CLIENT_SECRET=<client_secret>
CHROME_REFRESH_TOKEN=<refresh_token>
CHROME_PUBLISHER_ID=<publisher_id>
CHROME_API_VERSION=v1
```

`CHROME_PUBLISHER_ID` is required only for the v2 API. `CHROME_API_VERSION`
defaults to `v1`; set to `v2` to use the Chrome Web Store v2 API. The `insert`
command always uses the v1 API.

### Firefox

```dotenv
FIREFOX_CLIENT_ID=<client_id>
FIREFOX_CLIENT_SECRET=<client_secret>
```

### Edge (v1.1 — recommended)

```dotenv
EDGE_CLIENT_ID=<client_id>
EDGE_API_KEY=<api_key>
EDGE_API_VERSION=v1.1
```

**Note**: The Edge API key may contain special characters that need escaping.
Refer to the [godotenv documentation](https://github.com/joho/godotenv?tab=readme-ov-file#writing-env-files)
for quoting rules.

### Edge (v1.0 — deprecated)

```dotenv
EDGE_CLIENT_ID=<client_id>
EDGE_CLIENT_SECRET=<client_secret>
EDGE_ACCESS_TOKEN_URL=<access_token_url>
EDGE_API_VERSION=v1
```

## Usage

```
webext [global options] command [command options] [arguments...]
```

### Global Options

- `-v, --verbose` — enable debug-level logging.

### Commands

| Command   | Description                                      |
|-----------|--------------------------------------------------|
| `status`  | Returns extension info                           |
| `insert`  | Uploads a new extension to the store             |
| `update`  | Uploads a new version of an existing extension   |
| `publish` | Publishes an extension to the store              |
| `sign`    | Signs an extension in the store (Firefox only)   |
| `help`    | Shows a list of commands or help for one command |

### Examples

#### Status

Get extension status info from the stores:

```sh
# Chrome (v1 API, default)
./go-webext status chrome -a <item_id>

# Chrome (v2 API)
CHROME_API_VERSION=v2 ./go-webext status chrome -a <item_id>

# Firefox
./go-webext status firefox --app sample@example.org
```

#### Insert

Upload a new extension (not uploaded before):

```sh
# Chrome (v1 API)
./go-webext insert chrome -f ./chrome.zip

# Firefox
./go-webext insert firefox -f ./firefox.zip -s ./source.zip
```

**Edge**: There is no API for creating a new product. You must create it
manually in Microsoft Partner Center.

#### Update

Upload a new version of an existing extension:

```sh
# Chrome (v1 API, default)
./go-webext update chrome -a <item_id> -f ./chrome.zip

# Chrome (v2 API)
CHROME_API_VERSION=v2 ./go-webext update chrome -a <item_id> -f ./chrome.zip

# Firefox (listed channel)
./go-webext update firefox -f ./firefox.zip -s ./source.zip -c listed

# Firefox with approval notes for Mozilla reviewers
./go-webext update firefox -f ./firefox.zip -s ./source.zip -c listed \
  -n "Build with: docker run --rm -v \$(pwd):/src example/build"

# Edge
./go-webext update edge -f ./edge.zip -a <product_id>
```

Firefox update options:

- `-c, --channel` (required): `listed` or `unlisted`
- `-s, --source`: path to source archive
- `-n, --approval-notes`: information for Mozilla reviewers, visible only to
  Mozilla (e.g. build reproduction instructions)

Edge update options:

- `-t, --timeout`: upload timeout in seconds

#### Publish

Publish extensions to the stores:

**Chrome v1 API** (default):

```sh
# Basic publish
./go-webext publish chrome -a <item_id>

# Publish to trusted testers
./go-webext publish chrome -a <item_id> -t trustedTesters

# With gradual rollout and skip review
./go-webext publish chrome -a <item_id> -p 50 -e
```

**Chrome v2 API**:

```sh
# Basic publish (immediate after review completes)
CHROME_API_VERSION=v2 ./go-webext publish chrome -a <item_id>

# Staged publish (requires manual publish in dashboard)
CHROME_API_VERSION=v2 ./go-webext publish chrome -a <item_id> -s

# Gradual rollout to 50% of users
CHROME_API_VERSION=v2 ./go-webext publish chrome -a <item_id> -p 50

# Skip review if qualified
CHROME_API_VERSION=v2 ./go-webext publish chrome -a <item_id> -e

# Combine options (staged + 50% rollout + skip review)
CHROME_API_VERSION=v2 ./go-webext publish chrome -a <item_id> -s -p 50 -e
```

Chrome publish options:

- `-t, --target`: (v1 only) publish target (`trustedTesters` or `default`)
- `-e, --expedited`: request skip review if qualified
- `-s, --staged`: (v2 only) stage for future publishing instead of immediate
- `-p, --percentage`: deployment percentage (0–100) for gradual rollout

**Edge**:

```sh
./go-webext publish edge -a <product_id>
```

#### Sign

Sign an extension (Firefox only). Uses the unlisted channel.

```sh
# Basic sign (output defaults to firefox.xpi)
./go-webext sign firefox -f ./firefox.zip -s ./source.zip

# Specify output path
./go-webext sign firefox -f ./firefox.zip -s ./source.zip -o ./signed.xpi

# With approval notes
./go-webext sign firefox -f ./firefox.zip -s ./source.zip \
  -n "Build with: docker run --rm -v \$(pwd):/src example/build"
```

Sign options:

- `-s, --source`: path to source archive
- `-o, --output`: output file path (default: `firefox.xpi`)
- `-n, --approval-notes`: information for Mozilla reviewers

## Documentation

- [Development](DEVELOPMENT.md) — setup, build, test, and contribute
- [Deployment and configuration](DEPLOYMENT.md) — release process
- [Changelog](CHANGELOG.md) — release history
- [LLM agent rules](AGENTS.md) — project context for AI agents
