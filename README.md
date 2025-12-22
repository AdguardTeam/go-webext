# go-webext

Utils for managing web extensions written in Go.

## CLI

### Installation

```bash
go install github.com/adguardteam/go-webext
```

### Deployment

To release a new version:

1. Create a new release on GitHub
2. Add a new tag following semantic versioning (e.g., v0.1.3)

Users can install a specific version using:
```bash
go install github.com/adguardteam/go-webext@v0.1.3
```

### Usage:

Before start, you have to get credentials to the stores API.

#### Chrome Credentials

How to get Chrome Store credentials API described here https://developer.chrome.com/docs/webstore/using_webstore_api/

After getting `CODE`, `CLIENT_ID`, `CLIENT_SECRET`, you'd be able to get `refresh_token` via request to this url

```bash
curl "https://accounts.google.com/o/oauth2/token" -d \
"client_id=${CLIENT_ID}&client_secret=${CLIENT_SECRET}&code=${CODE}&grant_type=authorization_code&redirect_uri=urn:ietf:wg:oauth:2.0:oob"
```

**Note**: you have to use `refresh_token` instead of `access_token`.

Also you need to set up the publisher ID in the Chrome Developer Dashboard and use it in your environment variables.

#### Firefox Credentials

Getting credentials for Addons Mozilla Org is not difficult. You should have a Firefox account and be logged in. After
that go to the following link: https://addons.mozilla.org/en-US/developers/addon/api/key/

#### Edge Credentials

Microsoft Edge Addons Store now supports two API versions:
- v1.1 (Recommended) - Uses API Key authentication
- v1.0 (Deprecated) - Uses Client Secret and Access Token URL

**Important Note**: Microsoft is deprecating v1.0 API. You should migrate to v1.1 as soon as possible because:
- Secrets are being deprecated and replaced with API keys
- Access Token URL is being deprecated
- API key expiration will reduce from 2 years to 72 days

For v1.1 API credentials setup, visit: https://learn.microsoft.com/en-us/microsoft-edge/extensions-chromium/publish/api/using-addons-api?tabs=v1-1

#### Environment variables

Received credentials can be stored in the `.env` file or provided via environment variables.

For Edge Store v1.1 (Recommended):
```dotenv
EDGE_CLIENT_ID=<client_id>
EDGE_API_KEY=<api_key>
EDGE_API_VERSION="v1.1"
```

**Note**: The Edge API key might contain special characters that need to be escaped correctly. You can use the `godotenv` library to handle this. For more details on writing env files, refer to the [godotenv documentation](https://github.com/joho/godotenv?tab=readme-ov-file#writing-env-files).

For Edge Store v1.0 (Deprecated):
```dotenv
EDGE_CLIENT_ID=<client_id>
EDGE_CLIENT_SECRET=<client_secret>
EDGE_ACCESS_TOKEN_URL=<access_token_url>
EDGE_API_VERSION="v1"
```

For Chrome Store (both v1.1 and v2):
```dotenv
CHROME_CLIENT_ID=<client_id>
CHROME_CLIENT_SECRET=<client_secret>
CHROME_REFRESH_TOKEN=<refresh_token>
CHROME_PUBLISHER_ID=<publisher_id>
```

**Note**: `CHROME_PUBLISHER_ID` is required only for v2 API. Chrome Web Store now supports both v1.1 and v2 APIs. You can configure which API version to use via the `CHROME_API_VERSION` environment variable (defaults to v1). The `insert` command uses v1.1 API only. The `update`, `status`, and `publish` commands automatically use the configured API version.

After that, you can use the CLI.

```sh
webext [global options] command [command options] [arguments...]
```

#### Commands:

```
- status   returns extension info
- insert   uploads extension to the store
- update   uploads new version of extension to the store
- publish  publishes extension to the store
- sign     signs extension in the store
- help, h  shows a list of commands or help for one command
```

#### Examples:

##### Status:

Examples to get extension status info from the stores:

###### Chrome Web Store

```sh
# Using v1 API (default)
./go-webext status chrome -a <item_id>

# Using v2 API
CHROME_API_VERSION=v2 ./go-webext status chrome -a <item_id>
```

###### Addons Mozilla Org

```sh
./go-webext status firefox --app sample@example.org
```

##### Insert:
Upload new extension (not uploaded before) to the stores:

###### Chrome Web Store (v1.1 API)

```sh
./go-webext insert chrome -f ./chrome.zip
```

###### Addons Mozilla Org

```sh
./go-webext insert firefox -f ./firefox.zip -s ./source.zip
```

###### Microsoft Edge Addons Store

There is no API for creating a new product. You must complete these tasks manually in Microsoft Partner Center.

##### Update:
Upload new version of existing extension:

###### Chrome Web Store

```sh
# Using v1 API (default)
./go-webext update chrome -a <item_id> -f ./chrome.zip

# Using v2 API (set CHROME_API_VERSION=v2)
CHROME_API_VERSION=v2 ./go-webext update chrome -a <item_id> -f ./chrome.zip
```

###### Addons Mozilla Org

```sh
./go-webext update firefox -f ./firefox.zip -s ./source.zip
```

###### Microsoft Edge Addons Store

```sh
./go-webext update edge -f ./edge.zip -a 7a933b66-f8ff-4292-bc88-db593afg4bf8
```

##### Publish:
Publish extensions to the stores:

###### Chrome Web Store

Using v1 API (default):
```sh
# Basic publish
./go-webext publish chrome -a <item_id>

# Publish to trusted testers
./go-webext publish chrome -a <item_id> --target trustedTesters
# or using short alias
./go-webext publish chrome -a <item_id> -t trustedTesters

# With gradual rollout and skip review
./go-webext publish chrome -a <item_id> -p 50 --expedited
# or using short alias
./go-webext publish chrome -a <item_id> -p 50 -e
```

Using v2 API:
```sh
# Basic publish (immediate after review completes)
CHROME_API_VERSION=v2 ./go-webext publish chrome -a <item_id>

# Staged publish (requires manual publish in dashboard)
CHROME_API_VERSION=v2 ./go-webext publish chrome -a <item_id> -s

# Gradual rollout to 50% of users
CHROME_API_VERSION=v2 ./go-webext publish chrome -a <item_id> -p 50

# Skip review if qualified
CHROME_API_VERSION=v2 ./go-webext publish chrome -a <item_id> -e

# Combine options (staged publish with 50% rollout and skip review)
CHROME_API_VERSION=v2 ./go-webext publish chrome -a <item_id> -s -p 50 -e
```

Available options:
- `-t, --target`: (v1 only) Publish target (trustedTesters or default)
- `-e, --expedited`: Request skip review if qualified
- `-s, --staged`: (v2 only) Stage for publishing in the future instead of publishing immediately
- `-p, --percentage`: Deployment percentage (0-100) for gradual rollout

## Planned features

- [ ] create CLI to deploy to the stores
    - [x] chrome
    - [x] firefox
    - [x] edge
    - [ ] opera
    - [ ] static
    - [ ] TODO add description for every possible command
- [ ] get publish status for extensions from storage (published, draft, on review)
    - [ ] subscribe on status change via email or slack
- [ ] collect stats from storage

## Planned improvements

- [ ] setup sentry to collect errors
