package email

import "fmt"

func layout(content string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body style="margin:0;padding:0;background-color:#f8f8f8;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;">
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="background-color:#f8f8f8;padding:40px 20px;">
<tr><td align="center">
<table role="presentation" width="520" cellpadding="0" cellspacing="0" style="background-color:#ffffff;border-radius:8px;border:1px solid #e5e5e5;">

<!-- Header -->
<tr><td style="padding:32px 40px 0;">
<span style="font-size:18px;font-weight:600;color:#111;letter-spacing:-0.02em;">WriteKit</span>
</td></tr>

<!-- Content -->
<tr><td style="padding:24px 40px 32px;">
%s
</td></tr>

<!-- Footer -->
<tr><td style="padding:20px 40px;border-top:1px solid #e5e5e5;">
<p style="margin:0;font-size:12px;color:#999;line-height:1.5;">
WriteKit &mdash; Your blog, managed by conversation<br>
<a href="https://writekit.dev" style="color:#999;">writekit.dev</a>
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
<h1 style="margin:0 0 16px;font-size:22px;font-weight:500;color:#111;line-height:1.3;">%s, welcome to WriteKit</h1>

<p style="margin:0 0 20px;font-size:15px;color:#555;line-height:1.6;">
Your account is ready. WriteKit is an MCP-first blogging platform &mdash; you manage your blog entirely through conversation with your AI assistant.
</p>

<p style="margin:0 0 8px;font-size:13px;font-weight:600;color:#111;text-transform:uppercase;letter-spacing:0.04em;">Get started</p>

<div style="background-color:#f5f5f5;border:1px solid #e5e5e5;border-radius:6px;padding:16px 20px;margin:0 0 20px;">
<pre style="margin:0;font-family:'SF Mono','Fira Code',monospace;font-size:13px;line-height:1.6;color:#111;white-space:pre-wrap;">{
  "mcpServers": {
    "writekit": {
      "url": "https://writekit.dev/mcp"
    }
  }
}</pre>
</div>

<p style="margin:0 0 20px;font-size:14px;color:#555;line-height:1.6;">
Add this to your Claude Desktop configuration, then say <strong>&ldquo;write a post about...&rdquo;</strong> to get started.
</p>

<table role="presentation" cellpadding="0" cellspacing="0">
<tr><td style="background-color:#2563eb;border-radius:6px;">
<a href="https://writekit.dev/dashboard" style="display:inline-block;padding:10px 24px;font-size:14px;font-weight:500;color:#ffffff;text-decoration:none;">Go to Dashboard</a>
</td></tr>
</table>
`, greeting))
}

func commentNotificationHTML(blogName, postTitle, commentAuthor, commentContent, postURL string) string {
	return layout(fmt.Sprintf(`
<h1 style="margin:0 0 16px;font-size:20px;font-weight:500;color:#111;line-height:1.3;">New comment on your post</h1>

<p style="margin:0 0 20px;font-size:15px;color:#555;line-height:1.6;">
<strong>%s</strong> left a comment on <strong>&ldquo;%s&rdquo;</strong> on %s.
</p>

<div style="background-color:#f5f5f5;border-left:3px solid #e5e5e5;padding:16px 20px;margin:0 0 20px;border-radius:0 6px 6px 0;">
<p style="margin:0;font-size:14px;color:#333;line-height:1.6;">%s</p>
</div>

<table role="presentation" cellpadding="0" cellspacing="0">
<tr><td style="background-color:#2563eb;border-radius:6px;">
<a href="%s" style="display:inline-block;padding:10px 24px;font-size:14px;font-weight:500;color:#ffffff;text-decoration:none;">View Post</a>
</td></tr>
</table>

<p style="margin:16px 0 0;font-size:13px;color:#999;line-height:1.5;">
To delete this comment, ask your AI assistant: &ldquo;delete the spam comment from %s&rdquo;
</p>
`, commentAuthor, postTitle, blogName, commentContent, postURL, commentAuthor))
}
