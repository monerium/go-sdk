package monerium

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
	"nhooyr.io/websocket"
)

const (
	SandboxBaseURL      = "https://api.monerium.dev"
	SandboxWebsocketURL = "wss://api.monerium.dev"
	SandboxTokenURL     = "https://api.monerium.dev/auth/token"

	ProductionBaseURL      = "https://api.monerium.app"
	ProductionWebsocketURL = "wss://api.monerium.app"
	ProductionTokenURL     = "https://api.monerium.app/auth/token"
)

// NewClient initializes a new API client.
// baseURL and wsURL should point to corresponding urls for Sandbox or Production environments.
// AuthConfig is used for passing data related to OAuth2 ClientCredentials flow.
// Client behavior can be tweaked via ClientOption.
func NewClient(ctx context.Context, baseURL, wsURL string, auth *AuthConfig, opts ...ClientOption) *Client {
	conf := &clientcredentials.Config{
		ClientID:     auth.ClientID,
		ClientSecret: auth.ClientSecret,
		TokenURL:     auth.TokenURL,
	}

	cli := &Client{
		baseURL:     baseURL,
		wsURL:       wsURL,
		httpClient:  conf.Client(ctx),
		tokenSource: conf.TokenSource(ctx),
		notifyTick:  500 * time.Millisecond,
	}
	for _, o := range opts {
		o(cli)
	}

	return cli
}

// ClientOption represents an configurable option to Client.
type ClientOption func(*Client)

// WithNotifyTick sets tick duration for polling websocket connection.
func WithNotifyTick(d time.Duration) ClientOption {
	return func(c *Client) {
		c.notifyTick = d
	}
}

// Client represents a new Monerium API client.
type Client struct {
	baseURL     string
	wsURL       string
	httpClient  *http.Client
	tokenSource oauth2.TokenSource
	notifyTick  time.Duration
}

// AuthConfig is used for passing data related to OAuth2 Client Credentials flow.
type AuthConfig struct {
	// ClientID is the application's ID.
	ClientID string
	// ClientSecret is the application's secret.
	ClientSecret string
	// TokenURL is the resource server's token endpoint URL.
	TokenURL string
}

// dialWebsocket creates authorization header and dials websocket under path.
func dialWebsocket(ctx context.Context, path string, tok *oauth2.Token) (*websocket.Conn, error) {
	wc, _, err := websocket.Dial(ctx, path, &websocket.DialOptions{
		HTTPHeader: newAuthorizationHeaderFrom(tok),
	})
	return wc, err
}

// newAuthorizationHeaderFrom creates a new http.Header with Bearer token from oauth2.Token.
func newAuthorizationHeaderFrom(tok *oauth2.Token) http.Header {
	bearer := "Bearer " + tok.AccessToken

	return http.Header{
		"Authorization": []string{bearer},
	}
}

// get makes a HTTP GET request against path (base URL is taken from Client)
// and returns response body (as bytes) and headers on success.
func (c *Client) get(ctx context.Context, path string) ([]byte, error) {
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, http.NoBody)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(r)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	bs, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, newErrorFrom(path, bs, resp.Header)
	}

	return bs, nil
}

// post makes a HTTP POST request with req against path (base URL is taken from Client)
// and returns response body (as bytes) and headers on success.
// req is expected to be 'marshallable' to JSON.
func (c *Client) post(ctx context.Context, path string, req any) ([]byte, error) {
	rs, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(rs))
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(r)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	bs, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return nil, newErrorFrom(path, bs, resp.Header)
	}

	return bs, nil
}

// upload makes a HTTP POST request with form against path (base URL is taken from Client)
// and returns response body (as bytes) and headers on success.
// content is a content of a file to be uploaded, represented by the filename.
func (c *Client) upload(ctx context.Context, path string, filename string, content io.Reader) ([]byte, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(fw, content); err != nil {
		return nil, err
	}
	w.Close()

	r, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, &buf)
	if err != nil {
		return nil, err
	}
	r.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := c.httpClient.Do(r)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	bs, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, newErrorFrom(path, bs, resp.Header)
	}

	return bs, nil
}

// newErrorFrom creates a new client-facing error from call name, response body and headers.
func newErrorFrom(callName string, body []byte, header http.Header) error {
	var errResp errorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		return err
	}

	msg := fmt.Sprintf("%s call failed due to: %s", callName, errResp.Message)
	if corrID, ok := header["X-Correlation-Id"]; ok {
		errResp.CorrelationID = corrID[0]
		msg = fmt.Sprintf("%s. CorrelationID: %s", msg, errResp.CorrelationID)
	}
	if errResp.Errors != nil {
		msg = fmt.Sprintf("%s. Details: %s", msg, errResp.Errors)
	}

	return fmt.Errorf(msg)
}

// errorResponse represents error response and CorrelationID taken from 'X-Correlation-Id' header.
// Details represents details about resource failure.
// Errors represents a nested map of fields that failed validation.
type errorResponse struct {
	Code    int    `json:"code"`
	Status  string `json:"status"`
	Message string `json:"message"`
	Details struct {
		ID       string `json:"id"`
		Method   string `json:"method"`
		Resource string `json:"resource"`
	} `json:"details"`
	Errors        json.RawMessage `json:"errors"`
	CorrelationID string
}
