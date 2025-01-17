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

After getting `CODE`, `CLIENT_ID` and `CLIENT_SECRET`, you'd be able to get `refresh_token` via request to this url

```bash
curl "https://accounts.google.com/o/oauth2/token" -d \
"client_id=${CLIENT_ID}&client_secret=${CLIENT_SECRET}&code=${CODE}&grant_type=authorization_code&redirect_uri=urn:ietf:wg:oauth:2.0:oob"
```

**Note**: you have to use `refresh_token` instead of `access_token`.

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
./go-webext status chrome --app bjefoaoblohljkadffjcpkgfamdadogp
```

###### Addons Mozilla Org

```sh
./go-webext status firefox --app sample@example.org
```

##### Insert:
Examples of commands to upload new extension (not uploaded before) to the stores:

###### Addons Mozilla Org

```sh
./go-webext insert firefox -f ./firefox.zip -s ./source.zip
```

###### Chrome Web Store

```sh
./go-webext insert chrome -f ./chrome.zip
```

###### Microsoft Edge Addons Store

There is no API for creating a new product. You must complete these tasks manually in Microsoft Partner Center.

##### Update:
Examples of commands to upload new version of extension to the stores:

###### Addons Mozilla Org

```sh
./go-webext update firefox -f ./firefox.zip -s ./source.zip
```

###### Chrome Web Store

```sh
./go-webext update chrome -f ./chrome.zip -a bjefoaoblohljkaaaajcpkgfamdadogp
```

###### Microsoft Edge Addons Store

```sh
./go-webext update edge -f ./edge.zip -a 7a933b66-f8ff-4292-bc88-db593afg4bf8
```

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
