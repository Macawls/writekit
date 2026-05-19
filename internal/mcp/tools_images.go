package mcp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"writekit/internal/auth"
	"writekit/internal/image"
	"writekit/internal/tenant"
)

const (
	maxBase64InputBytes = 6 << 20
	maxURLFetchBytes    = 20 << 20
	urlFetchTimeout     = 10 * time.Second
)

func (s *Server) registerImageTools(mcpServer *mcp.Server) {
	mcpServer.AddTool(&mcp.Tool{
		Name: "upload_image",
		Description: `Upload an image and host it on the user's site. The server compresses to WebP, downscales large images (longest side capped at 2048px), strips EXIF, and stores the blob in the tenant's SQLite. Returns a stable /img/{id}.webp URL ready to drop into markdown as ![alt](url).

Provide exactly one of:
- data: base64-encoded image bytes (PNG, JPEG, GIF, or WebP; cap 6 MiB raw).
- url: HTTPS URL the server fetches (cap 20 MiB response).
- path: local file path. Desktop mode only; rejected on hosted.

Animated GIFs are converted to animated WebP automatically (per-frame, preserving timing).

The id is a sha256 content hash — uploading the same image twice returns the same URL (deduped=true), and the URL is immutable for the lifetime of the blob.`,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"data":      map[string]any{"type": "string", "description": "Base64-encoded image bytes"},
				"url":       map[string]any{"type": "string", "description": "HTTPS URL the server will fetch"},
				"path":      map[string]any{"type": "string", "description": "Local file path (desktop mode only)"},
				"tenant_id": map[string]any{"type": "string", "description": "Site ID (only needed if you have multiple sites)"},
			},
		},
	}, s.uploadImage)

	mcpServer.AddTool(&mcp.Tool{
		Name:        "list_images",
		Description: "List uploaded images for this site, newest first. Returns metadata only (no blob data).",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"limit":     map[string]any{"type": "integer", "description": "Max results (default 50)"},
				"offset":    map[string]any{"type": "integer", "description": "Pagination offset"},
				"tenant_id": map[string]any{"type": "string", "description": "Site ID (only needed if you have multiple sites)"},
			},
		},
	}, s.listImages)

	mcpServer.AddTool(&mcp.Tool{
		Name: "delete_image",
		Description: `Delete an uploaded image by id.

Note: image URLs are served with long-lived immutable cache headers, so existing CDN/browser caches may retain the blob until expiry. Deletion only stops new origin fetches.`,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":        map[string]any{"type": "string", "description": "Image id (sha256 hex)"},
				"tenant_id": map[string]any{"type": "string", "description": "Site ID (only needed if you have multiple sites)"},
			},
			"required": []string{"id"},
		},
	}, s.deleteImage)
}

func (s *Server) uploadImage(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	user := auth.UserFromContext(ctx)
	if user == nil {
		return toolError("not authenticated — please sign in at the WriteKit website first"), nil
	}

	var args struct {
		Data     string `json:"data"`
		URL      string `json:"url"`
		Path     string `json:"path"`
		TenantID string `json:"tenant_id"`
	}
	raw, _ := json.Marshal(req.Params.Arguments)
	json.Unmarshal(raw, &args)

	modes := 0
	if args.Data != "" {
		modes++
	}
	if args.URL != "" {
		modes++
	}
	if args.Path != "" {
		modes++
	}
	if modes != 1 {
		return toolError("provide exactly one of: data, url, path"), nil
	}

	db, tenantID, err := s.resolveTenantWithRole(ctx, user.ID, args.TenantID, "editor")
	if err != nil {
		return toolError(err.Error()), nil
	}

	src, srcErr := s.openImageSource(ctx, args.Data, args.URL, args.Path)
	if srcErr != nil {
		return toolError(srcErr.Error()), nil
	}
	defer src.Close()

	webpBytes, w, h, frames, err := image.Process(src)
	if err != nil {
		return toolErrorf(err, "image processing failed: %v", err), nil
	}

	sum := sha256.Sum256(webpBytes)
	id := hex.EncodeToString(sum[:])

	created, err := db.CreateImage(ctx, &tenant.Image{
		ID:         id,
		Bytes:      webpBytes,
		Width:      w,
		Height:     h,
		SizeBytes:  int64(len(webpBytes)),
		FrameCount: frames,
	})
	if err != nil {
		return toolErrorf(err, "failed to store image: %v", err), nil
	}

	result := map[string]any{
		"id":          id,
		"url":         s.tenantBaseURL(tenantID) + "/img/" + id + ".webp",
		"bytes":       len(webpBytes),
		"width":       w,
		"height":      h,
		"frame_count": frames,
		"deduped":     !created,
	}
	out, _ := json.MarshalIndent(result, "", "  ")
	return toolResult(string(out)), nil
}

type imageSource struct {
	io.Reader
	closers []io.Closer
}

func (s *imageSource) Close() error {
	var firstErr error
	for _, c := range s.closers {
		if err := c.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (s *Server) openImageSource(ctx context.Context, data, fetchURL, path string) (*imageSource, error) {
	switch {
	case data != "":
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(data))
		if err != nil {
			return nil, fmt.Errorf("invalid base64 data: %w", err)
		}
		if len(decoded) > maxBase64InputBytes {
			return nil, fmt.Errorf("base64 input exceeds %d bytes", maxBase64InputBytes)
		}
		return &imageSource{Reader: bytes.NewReader(decoded)}, nil

	case fetchURL != "":
		u, err := url.Parse(fetchURL)
		if err != nil {
			return nil, fmt.Errorf("invalid url: %w", err)
		}
		if u.Scheme != "https" {
			return nil, fmt.Errorf("url must use https scheme")
		}
		fetchCtx, cancel := context.WithTimeout(ctx, urlFetchTimeout)
		req, err := http.NewRequestWithContext(fetchCtx, http.MethodGet, fetchURL, nil)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("build request: %w", err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("fetch url: %w", err)
		}
		if resp.StatusCode/100 != 2 {
			resp.Body.Close()
			cancel()
			return nil, fmt.Errorf("fetch url: status %d", resp.StatusCode)
		}
		closer := closerFunc(func() error {
			err := resp.Body.Close()
			cancel()
			return err
		})
		return &imageSource{
			Reader:  io.LimitReader(resp.Body, maxURLFetchBytes+1),
			closers: []io.Closer{closer},
		}, nil

	case path != "":
		if !s.Config.Local {
			return nil, fmt.Errorf("path mode is only available in desktop mode")
		}
		cleaned := filepath.Clean(path)
		f, err := os.Open(cleaned)
		if err != nil {
			return nil, fmt.Errorf("open path: %w", err)
		}
		return &imageSource{Reader: f, closers: []io.Closer{f}}, nil
	}
	return nil, fmt.Errorf("no source provided")
}

type closerFunc func() error

func (f closerFunc) Close() error { return f() }

func (s *Server) listImages(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	user := auth.UserFromContext(ctx)
	if user == nil {
		return toolError("not authenticated"), nil
	}

	var args struct {
		Limit    int    `json:"limit"`
		Offset   int    `json:"offset"`
		TenantID string `json:"tenant_id"`
	}
	raw, _ := json.Marshal(req.Params.Arguments)
	json.Unmarshal(raw, &args)

	db, tenantID, err := s.resolveTenantWithRole(ctx, user.ID, args.TenantID, "viewer")
	if err != nil {
		return toolError(err.Error()), nil
	}

	metas, err := db.ListImages(ctx, args.Limit, args.Offset)
	if err != nil {
		return toolErrorf(err, "list images: %v", err), nil
	}

	base := s.tenantBaseURL(tenantID)
	items := make([]map[string]any, 0, len(metas))
	for _, m := range metas {
		items = append(items, map[string]any{
			"id":          m.ID,
			"url":         base + "/img/" + m.ID + ".webp",
			"width":       m.Width,
			"height":      m.Height,
			"bytes":       m.SizeBytes,
			"frame_count": m.FrameCount,
			"animated":    m.IsAnimated(),
			"created_at":  m.CreatedAt,
		})
	}
	out, _ := json.MarshalIndent(map[string]any{"images": items, "count": len(items)}, "", "  ")
	return toolResult(string(out)), nil
}

func (s *Server) deleteImage(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	user := auth.UserFromContext(ctx)
	if user == nil {
		return toolError("not authenticated"), nil
	}

	var args struct {
		ID       string `json:"id"`
		TenantID string `json:"tenant_id"`
	}
	raw, _ := json.Marshal(req.Params.Arguments)
	json.Unmarshal(raw, &args)

	if args.ID == "" {
		return toolError("id is required"), nil
	}

	db, _, err := s.resolveTenantWithRole(ctx, user.ID, args.TenantID, "editor")
	if err != nil {
		return toolError(err.Error()), nil
	}

	if err := db.DeleteImage(ctx, args.ID); err != nil {
		return toolErrorf(err, "delete image: %v", err), nil
	}
	return toolResult(fmt.Sprintf("deleted image %s", args.ID)), nil
}
