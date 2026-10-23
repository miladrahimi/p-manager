package cloudflare

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/cockroachdb/errors"
)

const DefaultBaseUrl = "https://api.cloudflare.com/client/v4"

// Client is a minimal Cloudflare API client covering what P-Manager needs to
// put P-Nodes behind the CDN: zone lookup, Origin CA certificates and DNS records.
type Client struct {
	token   string
	baseUrl string
	hc      *http.Client
}

// New creates a client for the given API token.
func New(token string) *Client {
	return &Client{token: token, baseUrl: DefaultBaseUrl, hc: &http.Client{Timeout: 30 * time.Second}}
}

// WithBaseUrl points the client at another API base URL (for tests).
func (c *Client) WithBaseUrl(baseUrl string) *Client {
	c.baseUrl = baseUrl
	return c
}

// ApiError is an error reported by the Cloudflare API.
type ApiError struct {
	Status  int
	Code    int
	Message string
}

func (e *ApiError) Error() string {
	return fmt.Sprintf("cloudflare: %s (%d)", e.Message, e.Code)
}

// IsNotFound reports whether err is a Cloudflare "not found" error.
func IsNotFound(err error) bool {
	var apiErr *ApiError
	return errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound
}

type Zone struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

type OriginCertificate struct {
	Id          string `json:"id"`
	Certificate string `json:"certificate"`
	ExpiresOn   string `json:"expires_on"`
}

type DnsRecord struct {
	Id      string `json:"id,omitempty"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	Proxied bool   `json:"proxied"`
	Ttl     int    `json:"ttl"`
	Comment string `json:"comment,omitempty"`
}

// FindZone returns the zone with the given name.
func (c *Client) FindZone(ctx context.Context, name string) (*Zone, error) {
	var zones []Zone
	if err := c.do(ctx, http.MethodGet, "/zones?"+url.Values{"name": {name}}.Encode(), nil, &zones); err != nil {
		return nil, err
	}
	if len(zones) == 0 {
		return nil, errors.Newf("cloudflare: zone %s not found (is it on this account and the token allowed to read it?)", name)
	}
	return &zones[0], nil
}

// CreateOriginCertificate signs csr (PEM) with Cloudflare's Origin CA for the
// given hostnames. validityDays is one of 7, 30, 90, 365, 730, 1095 or 5475.
func (c *Client) CreateOriginCertificate(
	ctx context.Context,
	csr string,
	hostnames []string,
	validityDays int,
) (*OriginCertificate, error) {
	body := map[string]any{
		"csr":                csr,
		"hostnames":          hostnames,
		"request_type":       "origin-ecc",
		"requested_validity": validityDays,
	}
	var certificate OriginCertificate
	if err := c.do(ctx, http.MethodPost, "/certificates", body, &certificate); err != nil {
		return nil, err
	}
	return &certificate, nil
}

// RevokeOriginCertificate revokes an Origin CA certificate.
func (c *Client) RevokeOriginCertificate(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/certificates/"+id, nil, nil)
}

// CreateDnsRecord creates a DNS record in the zone.
func (c *Client) CreateDnsRecord(ctx context.Context, zoneId string, record DnsRecord) (*DnsRecord, error) {
	var created DnsRecord
	if err := c.do(ctx, http.MethodPost, "/zones/"+zoneId+"/dns_records", record, &created); err != nil {
		return nil, err
	}
	return &created, nil
}

// DeleteDnsRecord deletes a DNS record from the zone.
func (c *Client) DeleteDnsRecord(ctx context.Context, zoneId, id string) error {
	return c.do(ctx, http.MethodDelete, "/zones/"+zoneId+"/dns_records/"+id, nil, nil)
}

// do sends a request and decodes the "result" of the response envelope into out.
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
		return errors.Wrap(err, "cloudflare: request failed")
	}
	defer func() { _ = response.Body.Close() }()

	content, err := io.ReadAll(response.Body)
	if err != nil {
		return errors.Wrap(err, "cloudflare: cannot read response")
	}

	var envelope struct {
		Success bool `json:"success"`
		Errors  []struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"errors"`
		Result json.RawMessage `json:"result"`
	}
	parsed := json.Unmarshal(content, &envelope) == nil

	if response.StatusCode >= 400 || (parsed && !envelope.Success) {
		apiErr := &ApiError{Status: response.StatusCode, Message: http.StatusText(response.StatusCode)}
		if parsed && len(envelope.Errors) > 0 {
			apiErr.Code = envelope.Errors[0].Code
			apiErr.Message = envelope.Errors[0].Message
		}
		return errors.WithStack(apiErr)
	}
	if !parsed {
		return errors.New("cloudflare: cannot parse response")
	}

	if out == nil || len(envelope.Result) == 0 {
		return nil
	}
	if err = json.Unmarshal(envelope.Result, out); err != nil {
		return errors.Wrap(err, "cloudflare: cannot parse result")
	}
	return nil
}
