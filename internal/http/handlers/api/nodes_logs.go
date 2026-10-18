package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/data"
	"github.com/miladrahimi/p-node/pkg/http/logs"
)

// nodeLogsClient talks to the nodes' log endpoints. It has no timeout because
// the stream is long-lived; requests are bounded by their context instead.
var nodeLogsClient = &http.Client{Transport: http.DefaultTransport}

// nodeLogsIndexTimeout bounds the node log file listing request.
const nodeLogsIndexTimeout = 10 * time.Second

// nodeLogsRequest builds an authenticated request to a node's log API. When
// the node is unknown or has no HTTP API (push disabled), it sends the error
// response itself and returns ok=false with its result.
func nodeLogsRequest(ctx context.Context, c echo.Context, db *data.Store, path string) (req *http.Request, sent error, ok bool) {
	var node data.Node
	found := false
	db.Read(func(d *data.Data) {
		if n := d.FindNodeById(c.Param("id")); n != nil {
			node = *n
			found = true
		}
	})
	if !found {
		return nil, c.JSON(http.StatusNotFound, map[string]string{"message": "Node not found."}), false
	}
	if node.HttpPort <= 0 || node.HttpToken == "" {
		return nil, c.JSON(http.StatusBadRequest, map[string]string{"message": "The node has no HTTP API (push disabled)."}), false
	}

	target := fmt.Sprintf("http://%s:%d%s", node.Host, node.HttpPort, path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, c.JSON(http.StatusBadRequest, map[string]string{"message": "Cannot build the node request."}), false
	}
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+node.HttpToken)
	return req, nil, true
}

// forwardNodeError relays a non-2xx node response to the client with the
// node's status code: its JSON message when it sent one, a generic one else.
func forwardNodeError(c echo.Context, resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
	if len(body) > 0 && strings.HasPrefix(resp.Header.Get(echo.HeaderContentType), echo.MIMEApplicationJSON) {
		return c.JSONBlob(resp.StatusCode, body)
	}
	return c.JSON(resp.StatusCode, map[string]string{
		"message": fmt.Sprintf("The node answered %d (is it up to date?).", resp.StatusCode),
	})
}

// NodesLogsIndex lists a node's log files through its GET /logs.
func NodesLogsIndex(db *data.Store) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx, cancel := context.WithTimeout(c.Request().Context(), nodeLogsIndexTimeout)
		defer cancel()
		req, sent, ok := nodeLogsRequest(ctx, c, db, "/logs")
		if !ok {
			return sent
		}
		resp, err := nodeLogsClient.Do(req)
		if err != nil {
			return c.JSON(http.StatusBadGateway, map[string]string{"message": "Cannot reach the node."})
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			return forwardNodeError(c, resp)
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return c.JSON(http.StatusBadGateway, map[string]string{"message": "Cannot read the node response."})
		}
		return c.JSONBlob(http.StatusOK, body)
	}
}

// NodesLogsStream proxies a node's GET /logs/:name chunked stream to the client
// until the client disconnects, the node ends it, or done is closed.
func NodesLogsStream(db *data.Store, done <-chan struct{}) echo.HandlerFunc {
	return func(c echo.Context) error {
		name := c.Param("name")
		if !logs.ValidName(name) {
			return c.JSON(http.StatusNotFound, map[string]string{"message": "Log file not found."})
		}

		ctx, cancel := context.WithCancel(c.Request().Context())
		defer cancel()
		go func() {
			select {
			case <-done:
				cancel()
			case <-ctx.Done():
			}
		}()

		path := "/logs/" + url.PathEscape(name)
		if lines := c.QueryParam("lines"); lines != "" {
			path += "?lines=" + url.QueryEscape(lines)
		}
		req, sent, ok := nodeLogsRequest(ctx, c, db, path)
		if !ok {
			return sent
		}
		resp, err := nodeLogsClient.Do(req)
		if err != nil {
			return c.JSON(http.StatusBadGateway, map[string]string{"message": "Cannot reach the node."})
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			return forwardNodeError(c, resp)
		}

		res := c.Response()
		res.Header().Set(echo.HeaderContentType, "text/plain; charset=utf-8")
		res.Header().Set("X-Accel-Buffering", "no")
		res.WriteHeader(http.StatusOK)

		// Relay each chunk as it arrives so the stream stays live.
		buf := make([]byte, 32<<10)
		for {
			n, err := resp.Body.Read(buf)
			if n > 0 {
				if _, werr := res.Write(buf[:n]); werr != nil {
					return nil
				}
				res.Flush()
			}
			if err != nil {
				return nil
			}
		}
	}
}
