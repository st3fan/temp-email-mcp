# temp-email-mcp

An MCP server that gives an AI agent read access to email accounts over
IMAP. It is the same functionality as the
[checkemail](https://github.com/st3fan/checkemail) command-line tool — list
messages, read a message, archive a message — exposed as MCP tools instead
of shell commands.

It is built for flows like signup verification, password reset and magic
links: the agent polls an inbox, reads the mail the application sent, acts
on it, and files the message away.

## Install

Install the binary:

```sh
go install github.com/st3fan/temp-email-mcp@latest
```

This puts `temp-email-mcp` in `$(go env GOPATH)/bin`.

Or run it without installing:

```sh
go run github.com/st3fan/temp-email-mcp@latest
```

By default the server speaks MCP over stdio, which is what local agents
want. Two other transports are available:

```sh
temp-email-mcp --transport stdio                # default; stdout/stdin
temp-email-mcp --transport sse --addr localhost:8080
temp-email-mcp --transport http --addr localhost:8080   # streamable HTTP at /mcp
```

## Configure

Accounts live in `~/.config/checkemail/accounts.json` — the same file the
`checkemail` CLI uses, so both tools can share one configuration. The key
is the account name you pass to the tools. The file holds credentials, so
keep it private:

```sh
chmod 600 ~/.config/checkemail/accounts.json
```

```json
{
  "ada@example.com": {
    "server": "imap.example.com",
    "username": "ada@example.com",
    "password": "app-specific-password"
  },
  "bob@example.com": {
    "server": "imap.example.com:993",
    "username": "bob@example.com",
    "password": "app-specific-password",
    "archive": "Saved Items"
  }
}
```

| field | required | meaning |
| --- | --- | --- |
| `server` | yes | IMAP host. May include `:port`; defaults to 993. |
| `username` | yes | Login name. |
| `password` | yes | Use an app-specific password, not the real one. |
| `port` | no | Only used when `server` has no port. |
| `archive` | no | Destination folder for `archive_message`. Autodetected when omitted. |

`$XDG_CONFIG_HOME` and `$CHECKEMAIL_ACCOUNTS` are honoured, in that order
of override.

## Use it in OpenCode

Add the server to OpenCode from the project (or add `--global` to make it
available everywhere):

```sh
opencode mcp add temp-email -- temp-email-mcp
```

Without installing the binary first, let OpenCode build and run it:

```sh
opencode mcp add temp-email -- go run github.com/st3fan/temp-email-mcp@latest
```

Or edit `opencode.jsonc` by hand:

```jsonc
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "servers": {
      "temp-email": {
        "type": "local",
        "command": ["temp-email-mcp"],
        "environment": {
          "CHECKEMAIL_ACCOUNTS": "{env:CHECKEMAIL_ACCOUNTS}"
        }
      }
    }
  }
}
```

Check that it connected:

```sh
opencode mcp list
```

To use the HTTP transport instead of stdio, run the server yourself and
add it as a remote server:

```sh
temp-email-mcp --transport http --addr localhost:8080
```

```jsonc
{
  "mcp": {
    "servers": {
      "temp-email": {
        "type": "remote",
        "url": "http://localhost:8080/mcp"
      }
    }
  }
}
```

The tools are then available as `temp-email_list_accounts`,
`temp-email_list_folders`, `temp-email_list_messages`,
`temp-email_read_message` and `temp-email_archive_message`.

## Tools

| Tool | Behaviour |
| --- | --- |
| `list_accounts` | Show configured account names and where the config was loaded from. |
| `list_folders` | Show all folders with total and unread counts and special-use notes. |
| `list_messages` | Show recent messages of a folder, newest first. Never marks mail read, so it is safe to poll. |
| `read_message` | Show one message in full and mark it as read. Quoted replies and signatures are stripped unless `raw` is set. |
| `archive_message` | Move one message to the account's archive folder. |

Message ids are `"<folder>:<uid>"` — exactly the form `list_messages`
prints. A bare number is shorthand for a message in `INBOX`.

Output is plain, labelled text (`LABEL: value` lines), the same format
`checkemail` prints, so it reads well in a transcript and greps well in a
script.

## Sample prompts

With the server connected, prompts like these exercise the whole loop:

```text
What email accounts do I have configured?
```

```text
List the unread messages in ada@example.com's inbox and summarize each one.
```

```text
Read the most recent message in ada@example.com's inbox, extract the
verification link, then archive the message.
```

```text
I just submitted a signup form for app.example.com with the address
ada@example.com. Watch the inbox for the confirmation email, follow up
to twice over the next minute, extract the confirmation link, and tell
me what it is. Archive the message once you have it.
```

```text
Which folders in bob@example.com have unread mail, and how much?
```

The typical loop the agent will run: poll with `list_messages -unread`,
read the interesting id with `read_message`, act on the link or code in
the body, then clean up with `archive_message`.

## Behaviour worth knowing

- **Listing never marks mail read.** `list_messages` only reads envelopes
  plus a peeked body preview. Only `read_message` sets `\Seen`.
- **Archive moves, it does not copy.** The message leaves the source
  folder. The destination is the account's `archive` setting, else the
  folder flagged `\Archive`, else an existing `Archive`/`Archives` folder,
  else a newly created `Archive`.
- **TLS only.** Implicit TLS on port 993. There is no plaintext and no
  STARTTLS path.
- **No sending.** There is no code path that can send, reply, edit or
  delete mail. `archive_message` is the only tool that changes anything,
  and it only moves.
- **Read the id, not the list index.** Indices shift as new mail arrives;
  uids do not.
