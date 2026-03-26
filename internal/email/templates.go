package email

import "fmt"

func layout(content string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body style="margin:0;padding:0;background-color:#fafafa;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;">
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="background-color:#fafafa;padding:40px 20px;">
<tr><td align="center">
<table role="presentation" width="480" cellpadding="0" cellspacing="0" style="background-color:#ffffff;border-radius:10px;border:1px solid #e4e4e7;">

<!-- Header -->
<tr><td style="padding:28px 32px 0;">
<span style="font-family:'SF Mono','Fira Code','JetBrains Mono',monospace;font-size:14px;font-weight:500;color:#0f0f0f;letter-spacing:-0.04em;">writekit</span>
</td></tr>

<!-- Content -->
<tr><td style="padding:20px 32px 28px;">
%s
</td></tr>

<!-- Footer -->
<tr><td style="padding:16px 32px;border-top:1px solid #e4e4e7;">
<p style="margin:0;font-size:12px;color:#a1a1aa;line-height:1.5;">
<a href="https://writekit.dev" style="color:#a1a1aa;text-decoration:none;">writekit.dev</a>
</p>
</td></tr>

</table>
</td></tr>
</table>
</body>
</html>`, content)
}

func welcomeHTML(name string) string {
	greeting := "Hi"
	if name != "" {
		greeting = fmt.Sprintf("Hi %s", name)
	}

	return layout(fmt.Sprintf(`
<p style="margin:0 0 16px;font-size:15px;color:#0f0f0f;line-height:1.5;">%s, welcome to WriteKit.</p>

<p style="margin:0 0 20px;font-size:14px;color:#71717a;line-height:1.6;">
Add the MCP server to your AI assistant to get started:
</p>

<div style="background-color:#fafafa;border:1px solid #e4e4e7;border-radius:8px;padding:14px 18px;margin:0 0 20px;">
<code style="margin:0;font-family:'SF Mono','Fira Code',monospace;font-size:13px;line-height:1.6;color:#0f0f0f;">claude mcp add --transport http writekit https://writekit.dev/mcp</code>
</div>

<p style="margin:0;font-size:13px;color:#a1a1aa;line-height:1.5;">
Or add <code style="font-family:'SF Mono','Fira Code',monospace;font-size:12px;background:#fafafa;border:1px solid #e4e4e7;padding:1px 5px;border-radius:3px;">{"mcpServers":{"writekit":{"url":"https://writekit.dev/mcp"}}}</code> to your MCP config.
</p>
`, greeting))
}

func magicLinkHTML(link string) string {
	return layout(fmt.Sprintf(`
<p style="margin:0 0 16px;font-size:15px;color:#0f0f0f;line-height:1.5;">Sign in to WriteKit</p>

<table role="presentation" cellpadding="0" cellspacing="0" style="margin:0 0 20px;">
<tr><td style="background-color:#18181b;border-radius:8px;">
<a href="%s" style="display:inline-block;padding:10px 20px;font-size:13px;font-weight:500;color:#ffffff;text-decoration:none;">Sign in</a>
</td></tr>
</table>

<p style="margin:0;font-size:13px;color:#a1a1aa;line-height:1.5;">
This link expires in 10 minutes. If you didn&rsquo;t request this, ignore this email.
</p>
`, link))
}

func commentNotificationHTML(ownerName, blogName, postTitle, commentAuthor, commentContent, postURL string) string {
	greeting := "Hey"
	if ownerName != "" {
		greeting = fmt.Sprintf("Hey %s", ownerName)
	}

	return layout(fmt.Sprintf(`
<p style="margin:0 0 16px;font-size:15px;color:#0f0f0f;line-height:1.5;">%s, new comment on &ldquo;%s&rdquo;</p>

<p style="margin:0 0 12px;font-size:13px;color:#71717a;">%s on %s:</p>

<div style="border-left:2px solid #e4e4e7;padding:8px 16px;margin:0 0 20px;">
<p style="margin:0;font-size:14px;color:#3f3f46;line-height:1.6;">%s</p>
</div>

<table role="presentation" cellpadding="0" cellspacing="0">
<tr><td style="background-color:#18181b;border-radius:8px;">
<a href="%s" style="display:inline-block;padding:10px 20px;font-size:13px;font-weight:500;color:#ffffff;text-decoration:none;">View post</a>
</td></tr>
</table>
`, greeting, postTitle, commentAuthor, blogName, commentContent, postURL))
}
