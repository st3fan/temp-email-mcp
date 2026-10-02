package main

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/emersion/go-imap"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// registerTools adds every tool of this server.
func registerTools(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("list_accounts",
		mcp.WithDescription(
			"List the configured email accounts. The account names returned here "+
				"are the values for the account argument of the other tools."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
	), handleListAccounts)

	s.AddTool(mcp.NewTool("list_folders",
		mcp.WithDescription(
			"List all folders of an email account with their total and unread counts."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithString("account",
			mcp.Required(),
			mcp.Description("Account name as returned by list_accounts"),
		),
	), handleListFolders)

	s.AddTool(mcp.NewTool("list_messages",
		mcp.WithDescription(
			"List recent messages in a folder of an email account, newest first. "+
				"Listing never marks messages as read, so it is safe to poll. "+
				"Every message has an id of the form \"<folder>:<uid>\"; pass it to "+
				"read_message or archive_message."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithString("account",
			mcp.Required(),
			mcp.Description("Account name as returned by list_accounts"),
		),
		mcp.WithString("folder",
			mcp.Description("Folder to read (default INBOX)"),
		),
		mcp.WithNumber("limit",
			mcp.Description("How many messages to show (default 20)"),
		),
		mcp.WithBoolean("unread",
			mcp.Description("Only show unread messages (default false)"),
		),
		mcp.WithBoolean("preview",
			mcp.Description("Include a one-line body preview (default true)"),
		),
	), handleListMessages)

	s.AddTool(mcp.NewTool("read_message",
		mcp.WithDescription(
			"Read one message in full. This marks the message as read. By default "+
				"quoted replies and the signature are stripped and HTML is converted "+
				"to text; set raw to keep them. Attachments are listed with name, "+
				"type and size but never decoded."),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithString("account",
			mcp.Required(),
			mcp.Description("Account name as returned by list_accounts"),
		),
		mcp.WithString("id",
			mcp.Required(),
			mcp.Description("Message id of the form \"<folder>:<uid>\", as printed by list_messages"),
		),
		mcp.WithBoolean("raw",
			mcp.Description("Keep quoted replies and the signature (default false)"),
		),
	), handleReadMessage)

	s.AddTool(mcp.NewTool("archive_message",
		mcp.WithDescription(
			"Move one message to the archive folder of an email account. This moves "+
				"the message out of its current folder; it cannot delete mail."),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithString("account",
			mcp.Required(),
			mcp.Description("Account name as returned by list_accounts"),
		),
		mcp.WithString("id",
			mcp.Required(),
			mcp.Description("Message id of the form \"<folder>:<uid>\", as printed by list_messages"),
		),
	), handleArchiveMessage)
}

// toolError turns an error into a tool result that reports the error to the
// calling agent.
func toolError(err error) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError(err.Error()), nil
}

// connect loads the named account and opens an IMAP/TLS session.
func connect(name string) (*Conn, error) {
	accounts, _, err := loadAccounts()
	if err != nil {
		return nil, err
	}
	acct, err := accounts.lookup(name)
	if err != nil {
		return nil, err
	}
	return dialAccount(name, acct)
}

// handleListAccounts implements the list_accounts tool.
func handleListAccounts(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	accounts, path, err := loadAccounts()
	if err != nil {
		return toolError(err)
	}

	var b bytes.Buffer
	kv(&b, "CONFIG", path)
	kv(&b, "ACCOUNTS", fmt.Sprintf("%d configured", len(accounts)))
	for _, name := range accounts.names() {
		acct := accounts[name]
		host, port, _ := splitHostPort(acct.Server, acct.Port)
		fmt.Fprintf(&b, "  - %s  (user %q on %s:%d)\n", name, acct.Username, host, port)
	}
	return mcp.NewToolResultText(b.String()), nil
}

// handleListFolders implements the list_folders tool.
func handleListFolders(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	account, err := req.RequireString("account")
	if err != nil {
		return toolError(err)
	}

	conn, err := connect(account)
	if err != nil {
		return toolError(err)
	}
	defer conn.Close()

	boxes, err := conn.mailboxes()
	if err != nil {
		return toolError(err)
	}

	folders := make([]folder, 0, len(boxes))
	for _, box := range boxes {
		f := folder{Name: box.Name, Notes: folderNotes(box.Attributes)}
		if st, err := conn.status(box.Name); err == nil {
			f.Total = st.Messages
			f.Unread = st.Unseen
		} else {
			f.Notes = append(f.Notes, "count unavailable")
		}
		folders = append(folders, f)
	}

	var b bytes.Buffer
	printFolders(&b, account, folders)
	return mcp.NewToolResultText(b.String()), nil
}

// handleListMessages implements the list_messages tool.
func handleListMessages(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	account, err := req.RequireString("account")
	if err != nil {
		return toolError(err)
	}
	mailbox := canonicalFolder(req.GetString("folder", "INBOX"))
	limit := req.GetInt("limit", 20)
	unread := req.GetBool("unread", false)
	preview := req.GetBool("preview", true)
	if limit < 1 {
		return toolError(fmt.Errorf("limit must be at least 1"))
	}

	conn, err := connect(account)
	if err != nil {
		return toolError(err)
	}
	defer conn.Close()

	stats, err := conn.status(mailbox)
	if err != nil {
		return toolError(err)
	}
	summary := &folderSummary{Total: stats.Messages, Unread: stats.Unseen}

	_, msgs, err := conn.listMessages(mailbox, limit, unread, preview)
	if err != nil {
		return toolError(err)
	}

	var b bytes.Buffer
	printHeader(&b, account, mailbox, summary)
	if len(msgs) == 0 {
		what := "messages"
		if unread {
			what = "unread messages"
		}
		fmt.Fprintf(&b, "\nNo %s in %q.\n", what, mailbox)
		return mcp.NewToolResultText(b.String()), nil
	}

	printMessages(&b, msgs, len(msgs), int(summary.Total))
	return mcp.NewToolResultText(b.String()), nil
}

// handleReadMessage implements the read_message tool.
func handleReadMessage(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	account, err := req.RequireString("account")
	if err != nil {
		return toolError(err)
	}
	raw := req.GetBool("raw", false)

	ref, err := parseMsgRef(req.GetString("id", ""))
	if err != nil {
		return toolError(err)
	}

	conn, err := connect(account)
	if err != nil {
		return toolError(err)
	}
	defer conn.Close()

	msg, err := conn.fetchMessage(ref)
	if err != nil {
		return toolError(err)
	}

	parsed, err := parseMessage(msg.Raw)
	if err != nil {
		parsed = &parsedMessage{Text: string(msg.Raw)}
	}

	var b bytes.Buffer
	printMessage(&b, account, msg, parsed, !raw)
	return mcp.NewToolResultText(b.String()), nil
}

// handleArchiveMessage implements the archive_message tool.
func handleArchiveMessage(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	account, err := req.RequireString("account")
	if err != nil {
		return toolError(err)
	}

	ref, err := parseMsgRef(req.GetString("id", ""))
	if err != nil {
		return toolError(err)
	}

	conn, err := connect(account)
	if err != nil {
		return toolError(err)
	}
	defer conn.Close()

	dest, err := conn.moveMessage(ref)
	if err != nil {
		return toolError(err)
	}

	var b bytes.Buffer
	printArchived(&b, account, ref, dest)
	return mcp.NewToolResultText(b.String()), nil
}

// folderNotes turns special-use attributes into readable notes.
func folderNotes(attrs []string) []string {
	special := []string{
		imap.ArchiveAttr, imap.AllAttr, imap.DraftsAttr, imap.FlaggedAttr,
		imap.JunkAttr, imap.SentAttr, imap.TrashAttr, imap.ImportantAttr,
	}
	notes := make([]string, 0, len(attrs))
	for _, attr := range attrs {
		for _, s := range special {
			if attr == s {
				notes = append(notes, strings.TrimPrefix(attr, `\`))
			}
		}
	}
	return notes
}

// canonicalFolder maps an empty or blank folder name onto INBOX.
func canonicalFolder(name string) string {
	if strings.TrimSpace(name) == "" {
		return "INBOX"
	}
	return strings.TrimSpace(name)
}
