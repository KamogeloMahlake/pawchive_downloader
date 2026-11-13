# Pawchive Downloader

<p align="center">
  <strong>A fast, resumable command-line archiver for Pawchive (and Kemono-style) creators.</strong><br>
  Downloads every post, file and attachment into a folder you own, keeps it up to date, and lets you read it offline in your terminal or browser.<br>
  <em>Built in Go. One small binary, no runtime to install, no accounts, no database server.</em>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.22%2B-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go 1.22+">
  <img src="https://img.shields.io/badge/Platform-Windows%20%7C%20Linux%20%7C%20macOS-555?style=for-the-badge" alt="Platforms: Windows, Linux, macOS">
  <img src="https://img.shields.io/badge/Tests-race%20detector%20clean-success?style=for-the-badge" alt="Tests pass with the race detector">
</p>

---

## Highlights

- **Server + path file addresses.** Every file and attachment is fetched from the server the post names, joined with the data folder and the path: `<server><data-prefix><path>`. If a file has no server, or its server fails, the `-data-url` host is used as a fallback.
- **Safe by default.** A server named by the API that points at your own computer or network (localhost, `10.x`, `192.168.x`, `.local`, ...) is refused unless you pass `-allow-private-servers`.
- **Resumable downloads.** Files are written as `.part` files and resumed with HTTP `Range`; the result is renamed only when the size checks out, so a crash never leaves a half-written file under its real name.
- **Stall protection and retries.** A connection that stops sending data is abandoned and retried (60 s by default); `Retry-After` is honoured; a 404 is not retried.
- **Incremental updates.** `update` fetches only new or edited posts. A rotated file server is not mistaken for an edit.
- **History is never lost.** When a post is edited, the old version is kept in a `history/` folder.
- **Offline reading.** Read a post in the terminal, or run a local web viewer with images, video, audio and downloads.
- **Parallel.** Several files download at once (`-workers`, default 4).
- **Tested.** Unit and end-to-end tests run against fake servers (no network) and pass with `go test -race`.

## How file addresses are built

A post lists its files like this:

```json
{ "name": "cover.jpg", "path": "/ab/cd/cover.jpg", "server": "https://n1.example" }
```

The downloader joins the pieces:

```text
<server>             + <data-prefix> + <path>
https://n1.example   + /data         + /ab/cd/cover.jpg   =>   https://n1.example/data/ab/cd/cover.jpg
```

| Situation | What happens |
|---|---|
| File has a server | Tried first: `<server><data-prefix><path>` |
| Server missing or fails | Falls back to `<data-url><path>` |
| Server is `n2.example` (no scheme) | `https://` is added |
| Prefix is already in the path or server | It is not added twice |
| Path has `%20` or a `?query` | Kept valid and encoded correctly |
| Path is a full `https://...` address | Used as it is |

If every address fails, the error lists each one that was tried.

## Getting started

You need [Go](https://go.dev/dl/) 1.26.3 or newer.

```bash
git clone https://github.com/KamogeloMahlake/pawchive_downloader.git
cd pawchive_downloader
go build -o pawchive_downloader .
```

Run it directly without building:

```bash
go run . help
```

### Your first archive

```bash
# 1. Check you have the right creator (nothing is downloaded)
./pawchive_downloader creator fanbox 3773642

# 2. Download all posts, files and attachments
./pawchive_downloader download fanbox 3773642

# 3. Later: fetch only what is new or edited (every creator you have)
./pawchive_downloader update

# 4. Browse it
./pawchive_downloader serve        # then open http://127.0.0.1:8080
```

`<service>` and `<id>` are the two parts of a creator's address on the site, for example `fanbox` and `3773642`.

## Commands

| Command | What it does |
|---|---|
| `creator <service> <id>` | Show a creator's profile without downloading |
| `download <service> <id>` | Download all posts and files of a creator |
| `update [<service> <id>]` | Fetch new and edited posts; with no arguments, update every creator already downloaded |
| `list [<service> <id>]` | List downloaded creators, or one creator's posts |
| `show <service> <id> <post-id>` | Read one downloaded post in the terminal |
| `serve` | Browse everything in your web browser |
| `help` | Show usage |

`update <service> <id>` for a creator you never downloaded stops with a hint to use `download` first (usually a typo).

## Flags

Flags work on any command and can go before or after the arguments. Most can also be set with an environment variable.

| Flag | Default | Environment variable | Meaning |
|---|---|---|---|
| `-dir` | `pawchive_data` | `PAWCHIVE_DIR` | Data folder |
| `-base` | `https://pawchive.pw/api/v1` | `PAWCHIVE_BASE_URL` | API base URL |
| `-data-url` | `https://n4.pawchive.pw/data/` | `PAWCHIVE_DATA_URL` | Fallback file host, used when a file has no server or its server fails |
| `-workers` | `4` | | Parallel downloads |
| `-no-files` | off | | Save post text only, skip files (`download` / `update`) |
| `-addr` | `127.0.0.1:8080` | | Address for `serve` |

Examples:

```bash
./pawchive_downloader download fanbox 3773642 -workers 8
./pawchive_downloader download fanbox 3773642 -no-files          # text only
./pawchive_downloader show fanbox 3773642 1234567
./pawchive_downloader serve -addr 127.0.0.1:9090
```

## What gets saved

Everything lives under the data folder (`pawchive_data` by default), as plain files you can back up, sync or open with any tool:

```text
pawchive_data/
  <service>/
    <user-id>/
      creator.json          the creator's profile
      index.json            table of contents: every post, its files, sizes and status
      posts/
        <post-id>/
          post.json         the post as the API returned it
          history/          earlier versions, if the post was edited
          files/
            ab12cd34_cover.jpg
            ef56ab78_book.zip
```

File names get a short prefix made from the file's path, so two files with the same name in one post never overwrite each other.

`pawchive_data/` is listed in `.gitignore`.

## Updating and resuming

- Run `update` as often as you like. Unchanged posts and files already on disk are skipped; a file whose size on disk no longer matches the record is downloaded again.
- Stopped halfway (Ctrl+C, power cut, lost connection)? Run the same command again. Finished files are kept and partial files are resumed.
- Posts that disappear from the site are marked as no longer listed and your local copy is kept.
- A file that failed is recorded with its error in `index.json` and tried again on the next `update`.

## The web viewer

`serve` starts a read-only viewer on `127.0.0.1` (only your computer can reach it, unless you change `-addr`):

- a home page listing your creators
- a creator page with a post list
- a post page with the text, images, video and audio shown inline, and other files as downloads

Files are served with `X-Content-Type-Options: nosniff` and a strict Content-Security-Policy; only `GET` requests are accepted and path tricks like `..` are rejected.

## Development

Project layout:

```text
main.go                    command line: flags, commands, help
pawchive/                  the API's data types (Post, File, Attachment) and the edit fingerprint
internal/archive/          the sync engine: file addresses, worker pool, per-post logic
internal/clients/          HTTP client: retries, resume, stall timeout
internal/storage/          the folder layout, index and atomic JSON writes
internal/viewer/           terminal output and the web viewer
```

`STEP_BY_STEP_CHANGES.md` (if you have it) explains each change made in the latest improvement pass, with commented code.

## Disclaimer

This tool is intended for personal archiving and backup. Please respect creators' rights and the terms of service of the sites you use.
