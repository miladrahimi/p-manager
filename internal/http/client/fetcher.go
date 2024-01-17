package client

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/xray-manager/internal/config"
	"io"
	"net/http"
	"time"
)

const DebugURL = "https://rg.miladrahimi.com/"

var ErrUnauthorized = errors.New("fetcher: unauthorized")

type Fetcher struct {
	Engine *http.Client
}

func (f *Fetcher) Do(method, url, token string, requestBody interface{}) ([]byte, error) {
	var requestReader io.Reader
	if requestBody != nil {
		jsonBody, err := json.Marshal(requestBody)
		if err != nil {
			return nil, f.ErrWrap(err, "cannot marshal body", method, url)
		}
		requestReader = bytes.NewBuffer(jsonBody)
	}

	request, err := http.NewRequest(method, url, requestReader)
	if err != nil {
		return nil, f.ErrWrap(err, "cannot create request", method, url)
	}

	request.Header.Add(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request.Header.Add(echo.HeaderAuthorization, "Bearer "+token)

	response, err := f.Engine.Do(request)
	if err != nil {
		return nil, f.ErrWrap(err, "cannot do request", method, url)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode == http.StatusUnauthorized {
		return nil, ErrUnauthorized
	}

	if response.StatusCode != http.StatusOK {
		return nil, f.ErrMake(fmt.Sprintf("bad response status: %d", response.StatusCode), method, url)
	}

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, f.ErrWrap(err, "cannot read response body", method, url)
	}

	return responseBody, nil
}

func (f *Fetcher) ErrWrap(err error, message string, method, url string) error {
	return fmt.Errorf("fetcher: %s, method: %s, url: %s err: %s", message, method, url, err.Error())
}

func (f *Fetcher) ErrMake(message string, method, url string) error {
	return fmt.Errorf("fetcher: %s, method: %s, url: %s", message, method, url)
}

// New creates an instance of Fetcher.
func New(c *config.Config) *Fetcher {
	customTransport := http.DefaultTransport.(*http.Transport).Clone()
	customTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	return &Fetcher{
		Engine: &http.Client{
			Transport: customTransport,
			Timeout:   time.Duration(c.HttpClient.Timeout) * time.Second,
		},
	}
}
