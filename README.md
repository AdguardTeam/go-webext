# go-webext

Utils for managing web extensions written in Go.

## CLI

### Installation

```bash
go install github.com/adguardteam/go-webext
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

Description about getting credentials for Microsoft Edge Addons Store can be found
here: https://docs.microsoft.com/en-us/microsoft-edge/extensions-chromium/publish/api/using-addons-api#before-you-begin

#### Environment variables

Received credentials can be stored in the `.env` file or provided via environment variables.

```dotenv
CHROME_CLIENT_ID=<client_id>
CHROME_CLIENT_SECRET=<client_secret>
CHROME_REFRESH_TOKEN=<refresh_token>

FIREFOX_CLIENT_ID=<client_id>
FIREFOX_CLIENT_SECRET=<client_secret>

EDGE_CLIENT_ID=<client_id>
EDGE_CLIENT_SECRET=<client_secret>
EDGE_ACCESS_TOKEN_URL=<access_token_url>
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

To get status of the extension in the Chrome store:

```sh
webext status chrome --app bjefoaoblohljkbmkfjcpkgfamdadogp
```

To get status of the extension in the Firefox store:

```sh
webext status firefox --app sample@example.org
```

To upload new extension to the Mozilla store:

```sh
webext insert firefox -f /path/to/file -s /path/to/source
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
