package metadatafs

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/jszwedko/ec2-metadatafs/logger"
)

// IMDSv2Client wraps an HTTP client to access v2 of the Instance Metadata Service API
type IMDSv2Client struct {
	Client   *http.Client
	Endpoint string
	TokenTTL time.Duration
	Logger   logger.LeveledLogger

	tokenMu sync.RWMutex
	token   metadataToken
}

type metadataToken struct {
	Expires time.Time
	Token   string
}

// NewIMDSv2Client returns a new IMDSv2Client
func NewIMDSv2Client(endpoint string, tokenTTL time.Duration, l logger.LeveledLogger) *IMDSv2Client {
	return &IMDSv2Client{
		Client:   &http.Client{},
		Endpoint: endpoint,
		TokenTTL: tokenTTL,
		Logger:   l,
	}
}

// Get issues a GET request to the given path, refreshing the access token if needed
func (c *IMDSv2Client) Get(path string) (*http.Response, error) {
	token, err := c.getToken()
	if err != nil {
		return nil, fmt.Errorf("could not refresh metadata token: %w", err)
	}

	url := joinURL(c.Endpoint, path)
	c.Logger.Debugf("issuing HTTP GET to AWS metadata API for path: %s", url)
	r, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("could not build GET request: %w", err)
	}

	r.Header.Add("X-aws-ec2-metadata-token", token)

	resp, err := c.Client.Do(r)
	if err != nil {
		return nil, err
	}
	c.Logger.Debugf("got %d from AWS metadata API for path %s", resp.StatusCode, url)
	return resp, nil
}

// Head issues a HEAD request to the given path, refreshing the access token if needed
func (c *IMDSv2Client) Head(path string) (*http.Response, error) {
	token, err := c.getToken()
	if err != nil {
		return nil, fmt.Errorf("could not refresh metadata token: %w", err)
	}
	url := joinURL(c.Endpoint, path)
	c.Logger.Debugf("issuing HTTP HEAD to AWS metadata API for path: %s", url)
	r, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return nil, fmt.Errorf("could not build HEAD request: %w", err)
	}
	r.Header.Add("X-aws-ec2-metadata-token", token)

	resp, err := c.Client.Do(r)
	if err != nil {
		return nil, err
	}
	c.Logger.Debugf("got %d from AWS metadata API for path %s", resp.StatusCode, url)
	return resp, nil
}

// return the token, refreshing if needed
func (c *IMDSv2Client) getToken() (string, error) {
	const prefetchWindow = 10 * time.Second

	c.tokenMu.RLock()
	if c.token.Token != "" && c.token.Expires.After(time.Now()) {
		c.tokenMu.RUnlock()
		return c.token.Token, nil
	}
	c.tokenMu.RUnlock()

	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()

	url := joinURL(c.Endpoint, "/api/token")
	r, err := http.NewRequest(http.MethodPut, url, nil)
	if err != nil {
		return "", fmt.Errorf("error building refresh metadata token request: %w", err)
	}

	r.Header.Add("X-aws-ec2-metadata-token-ttl-seconds", strconv.FormatInt(int64(c.TokenTTL/time.Second), 10))

	expires := time.Now().Add(c.TokenTTL - prefetchWindow)
	c.Logger.Debugf("issuing HTTP PUT to AWS metadata API for path: %s", url)
	resp, err := c.Client.Do(r)
	if err != nil {
		return "", fmt.Errorf("error refreshing metadata token: %w", err)
	}
	defer resp.Body.Close()

	token, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading metadata token response: %w", err)
	}

	c.token.Token = string(token)
	c.token.Expires = expires

	return c.token.Token, nil
}
