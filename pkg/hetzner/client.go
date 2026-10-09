package hetzner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/cockroachdb/errors"
)

const defaultBaseUrl = "https://api.hetzner.cloud/v1"

// Client is a minimal Hetzner Cloud API client covering what P-Manager needs
// to create and delete P-Node servers.
type Client struct {
	token   string
	baseUrl string
	hc      *http.Client
}

// New creates a client for the given project API token.
func New(token string) *Client {
	return &Client{token: token, baseUrl: defaultBaseUrl, hc: &http.Client{Timeout: 30 * time.Second}}
}

// WithBaseUrl points the client at another API base URL (for tests).
func (c *Client) WithBaseUrl(baseUrl string) *Client {
	c.baseUrl = baseUrl
	return c
}

// ApiError is an error reported by the Hetzner API.
type ApiError struct {
	Status  int
	Code    string
	Message string
}

func (e *ApiError) Error() string {
	return fmt.Sprintf("hetzner: %s (%s)", e.Message, e.Code)
}

// IsNotFound reports whether err is a Hetzner "not found" error.
func IsNotFound(err error) bool {
	var apiErr *ApiError
	return errors.As(err, &apiErr) && (apiErr.Code == "not_found" || apiErr.Status == http.StatusNotFound)
}

// IsUniqueness reports whether err is a Hetzner uniqueness (name taken) error.
func IsUniqueness(err error) bool {
	var apiErr *ApiError
	return errors.As(err, &apiErr) && apiErr.Code == "uniqueness_error"
}

type SshKey struct {
	Id          int64  `json:"id"`
	Name        string `json:"name"`
	PublicKey   string `json:"public_key"`
	Fingerprint string `json:"fingerprint"`
}

type Location struct {
	Name    string `json:"name"`
	Country string `json:"country"`
	City    string `json:"city"`
}

type Price struct {
	Location     string `json:"location"`
	PriceMonthly struct {
		Net   string `json:"net"`
		Gross string `json:"gross"`
	} `json:"price_monthly"`
	PriceHourly struct {
		Net   string `json:"net"`
		Gross string `json:"gross"`
	} `json:"price_hourly"`
}

type ServerTypeLocation struct {
	Name      string `json:"name"`
	Available bool   `json:"available"`
}

type ServerType struct {
	Id           int64                `json:"id"`
	Name         string               `json:"name"`
	Architecture string               `json:"architecture"`
	Deprecated   bool                 `json:"deprecated"`
	Cores        int                  `json:"cores"`
	Memory       float64              `json:"memory"`
	Disk         int                  `json:"disk"`
	Prices       []Price              `json:"prices"`
	Locations    []ServerTypeLocation `json:"locations"`
}

// MonthlyPrice returns the gross monthly price of the type at the location,
// or 0 when the type is not priced there.
func (t ServerType) MonthlyPrice(location string) float64 {
	for _, p := range t.Prices {
		if p.Location == location {
			price, err := strconv.ParseFloat(p.PriceMonthly.Gross, 64)
			if err != nil {
				return 0
			}
			return price
		}
	}
	return 0
}

// HourlyPriceNet returns the net hourly price string of the type at the
// location (trimmed to 4 decimals), or "" when not priced there.
func (t ServerType) HourlyPriceNet(location string) string {
	for _, p := range t.Prices {
		if p.Location == location {
			price, err := strconv.ParseFloat(p.PriceHourly.Net, 64)
			if err != nil {
				return ""
			}
			return strconv.FormatFloat(price, 'f', 4, 64)
		}
	}
	return ""
}

type Image struct {
	Id         int64   `json:"id"`
	Name       string  `json:"name"`
	OsFlavor   string  `json:"os_flavor"`
	OsVersion  string  `json:"os_version"`
	Status     string  `json:"status"`
	Deprecated *string `json:"deprecated"`
}

type Server struct {
	Id        int64  `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Created   string `json:"created"`
	PublicNet struct {
		Ipv4 *struct {
			Ip string `json:"ip"`
		} `json:"ipv4"`
	} `json:"public_net"`
	ServerType ServerType `json:"server_type"`
	Location   Location   `json:"location"`
	Image      *Image     `json:"image"`
}

// Ipv4 returns the server's public IPv4 address, or "" when it has none yet.
func (s *Server) Ipv4() string {
	if s == nil || s.PublicNet.Ipv4 == nil {
		return ""
	}
	return s.PublicNet.Ipv4.Ip
}

type PublicNet struct {
	EnableIpv4 bool `json:"enable_ipv4"`
	EnableIpv6 bool `json:"enable_ipv6"`
}

type CreateServerRequest struct {
	Name             string            `json:"name"`
	ServerType       string            `json:"server_type"`
	Image            string            `json:"image"`
	Location         string            `json:"location"`
	SshKeys          []int64           `json:"ssh_keys"`
	Labels           map[string]string `json:"labels,omitempty"`
	PublicNet        PublicNet         `json:"public_net"`
	StartAfterCreate bool              `json:"start_after_create"`
}

// ListSshKeys returns all SSH keys of the project.
func (c *Client) ListSshKeys(ctx context.Context) ([]SshKey, error) {
	return listAll[SshKey](ctx, c, "/ssh_keys", "ssh_keys", nil)
}

// CreateSshKey uploads a public key to the project.
func (c *Client) CreateSshKey(ctx context.Context, name, publicKey string) (*SshKey, error) {
	var response struct {
		SshKey *SshKey `json:"ssh_key"`
	}
	body := map[string]string{"name": name, "public_key": publicKey}
	if err := c.do(ctx, http.MethodPost, "/ssh_keys", body, &response); err != nil {
		return nil, err
	}
	return response.SshKey, nil
}

// ListLocations returns all locations.
func (c *Client) ListLocations(ctx context.Context) ([]Location, error) {
	return listAll[Location](ctx, c, "/locations", "locations", nil)
}

// ListServerTypes returns all server types with their per-location prices
// and availability.
func (c *Client) ListServerTypes(ctx context.Context) ([]ServerType, error) {
	return listAll[ServerType](ctx, c, "/server_types", "server_types", nil)
}

// ListSystemImages returns the available x86 system (OS) images.
func (c *Client) ListSystemImages(ctx context.Context) ([]Image, error) {
	query := url.Values{}
	query.Set("type", "system")
	query.Set("architecture", "x86")
	query.Set("status", "available")
	return listAll[Image](ctx, c, "/images", "images", query)
}

// CreateServer creates a server.
func (c *Client) CreateServer(ctx context.Context, request CreateServerRequest) (*Server, error) {
	var response struct {
		Server *Server `json:"server"`
	}
	if err := c.do(ctx, http.MethodPost, "/servers", request, &response); err != nil {
		return nil, err
	}
	if response.Server == nil {
		return nil, errors.New("hetzner: create server returned no server")
	}
	return response.Server, nil
}

// GetServer returns a server by id.
func (c *Client) GetServer(ctx context.Context, id int64) (*Server, error) {
	var response struct {
		Server *Server `json:"server"`
	}
	if err := c.do(ctx, http.MethodGet, "/servers/"+strconv.FormatInt(id, 10), nil, &response); err != nil {
		return nil, err
	}
	if response.Server == nil {
		return nil, errors.New("hetzner: get server returned no server")
	}
	return response.Server, nil
}

// DeleteServer deletes a server by id.
func (c *Client) DeleteServer(ctx context.Context, id int64) error {
	return c.do(ctx, http.MethodDelete, "/servers/"+strconv.FormatInt(id, 10), nil, nil)
}

// listAll fetches every page of a list endpoint and returns the items under key.
func listAll[T any](ctx context.Context, c *Client, path, key string, query url.Values) ([]T, error) {
	var all []T
	for page := 1; ; page++ {
		q := url.Values{}
		for k, v := range query {
			q[k] = v
		}
		q.Set("page", strconv.Itoa(page))
		q.Set("per_page", "50")

		var envelope map[string]json.RawMessage
		if err := c.do(ctx, http.MethodGet, path+"?"+q.Encode(), nil, &envelope); err != nil {
			return nil, err
		}

		var items []T
		if raw, ok := envelope[key]; ok {
			if err := json.Unmarshal(raw, &items); err != nil {
				return nil, errors.Wrapf(err, "hetzner: cannot parse %s", key)
			}
		}
		all = append(all, items...)

		var meta struct {
			Pagination struct {
				NextPage *int `json:"next_page"`
			} `json:"pagination"`
		}
		if raw, ok := envelope["meta"]; ok {
			_ = json.Unmarshal(raw, &meta)
		}
		if meta.Pagination.NextPage == nil || *meta.Pagination.NextPage <= page {
			return all, nil
		}
	}
}

// do performs an API request and decodes the JSON response into out (if not nil).
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		content, err := json.Marshal(body)
		if err != nil {
			return errors.WithStack(err)
		}
		reader = bytes.NewReader(content)
	}

	request, err := http.NewRequestWithContext(ctx, method, c.baseUrl+path, reader)
	if err != nil {
		return errors.WithStack(err)
	}
	request.Header.Set("Authorization", "Bearer "+c.token)
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := c.hc.Do(request)
	if err != nil {
		return errors.Wrap(err, "hetzner: request failed")
	}
	defer func() { _ = response.Body.Close() }()

	content, err := io.ReadAll(response.Body)
	if err != nil {
		return errors.Wrap(err, "hetzner: cannot read response")
	}

	if response.StatusCode >= 400 {
		apiErr := &ApiError{Status: response.StatusCode, Code: "unknown", Message: http.StatusText(response.StatusCode)}
		var envelope struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(content, &envelope) == nil && envelope.Error.Message != "" {
			apiErr.Code = envelope.Error.Code
			apiErr.Message = envelope.Error.Message
		}
		return errors.WithStack(apiErr)
	}

	if out == nil || len(content) == 0 {
		return nil
	}
	if err = json.Unmarshal(content, out); err != nil {
		return errors.Wrap(err, "hetzner: cannot parse response")
	}
	return nil
}
