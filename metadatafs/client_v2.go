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

type IMDSv2Client struct {
	Client   *http.Client
	Endpoint string
	TokenTTL time.Duration
	Logger   logger.LeveledLogger

	tokenMu sync.RWMutex
	token   MetadataToken
}

type MetadataToken struct {
	Expires time.Time
	Token   string
}

func NewIMDSv2Client(endpoint string, tokenTTL time.Duration, l logger.LeveledLogger) *IMDSv2Client {
	return &IMDSv2Client{
		Client:   &http.Client{},
		Endpoint: endpoint,
		TokenTTL: tokenTTL,
		Logger:   l,
	}
}

func (c *IMDSv2Client) Get(path string) (*http.Response, error) {
	err := c.ensureFreshToken()
	if err != nil {
		return nil, fmt.Errorf("could not refresh metadata token: %w", err)
	}

	url := joinURL(c.Endpoint, path)
	c.Logger.Debugf("issuing HTTP GET to AWS metadata API for path: %s", url)
	r, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("could not build GET request: %w", err)
	}

	r.Header.Add("X-aws-ec2-metadata-token", func() string {
		c.tokenMu.RLock()
		defer c.tokenMu.RUnlock()
		return c.token.Token
	}())

	resp, err := c.Client.Do(r)
	if err != nil {
		return nil, err
	}
	c.Logger.Debugf("got %d from AWS metadata API for path %s", resp.StatusCode, url)
	return resp, nil
}

func (c *IMDSv2Client) Head(path string) (*http.Response, error) {
	err := c.ensureFreshToken()
	if err != nil {
		return nil, fmt.Errorf("could not refresh metadata token: %w", err)
	}
	url := joinURL(c.Endpoint, path)
	c.Logger.Debugf("issuing HTTP HEAD to AWS metadata API for path: %s", url)
	r, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return nil, fmt.Errorf("could not build HEAD request: %w", err)
	}
	r.Header.Add("X-aws-ec2-metadata-token", func() string {
		c.tokenMu.RLock()
		defer c.tokenMu.RUnlock()
		return c.token.Token
	}())

	resp, err := c.Client.Do(r)
	if err != nil {
		return nil, err
	}
	c.Logger.Debugf("got %d from AWS metadata API for path %s", resp.StatusCode, url)
	return resp, nil
}

// set -x TOKEN (curl -X PUT "http://localhost:8080/latest/api/token" -H "X-aws-ec2-metadata-token-ttl-seconds: 21600")
func (c *IMDSv2Client) ensureFreshToken() (err error) {
	needsRefresh := func() bool {
		c.tokenMu.RLock()
		defer c.tokenMu.RUnlock()
		// if the token would expire in the next second
		return c.token.Token == "" || c.token.Expires.Truncate(time.Second).Before(time.Now().Truncate(time.Second).Add(time.Second))
	}

	if !needsRefresh() {
		return nil
	}

	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()

	url := joinURL(c.Endpoint, "/api/token")
	r, err := http.NewRequest(http.MethodPut, url, nil)
	if err != nil {
		return fmt.Errorf("error building refresh metadata token request: %w", err)
	}

	r.Header.Add("X-aws-ec2-metadata-token-ttl-seconds", strconv.FormatInt(int64(c.TokenTTL/time.Second), 10))

	// set before request to avoid unexpected expiration
	expires := time.Now().Add(c.TokenTTL)
	c.Logger.Debugf("issuing HTTP PUT to AWS metadata API for path: %s", url)
	resp, err := c.Client.Do(r)
	if err != nil {
		return fmt.Errorf("error refreshing metadata token: %w", err)
	}
	defer resp.Body.Close()

	token, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading metadata token response: %w", err)
	}

	c.token.Token = string(token)
	c.token.Expires = expires

	return nil
}
