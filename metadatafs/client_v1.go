package metadatafs

import (
	"net/http"

	"github.com/jszwedko/ec2-metadatafs/logger"
)

// IMDSv1Client wraps an HTTP client to access v1 of the Instance Metadata Service API
type IMDSv1Client struct {
	Client   *http.Client
	Endpoint string
	Logger   logger.LeveledLogger
}

// NewIMDSv1Client returns a new IMDSv1Client
func NewIMDSv1Client(endpoint string, l logger.LeveledLogger) *IMDSv1Client {
	return &IMDSv1Client{
		Client:   &http.Client{},
		Endpoint: endpoint,
		Logger:   l,
	}
}

// Get issues a GET request to the given path
func (c *IMDSv1Client) Get(path string) (*http.Response, error) {
	url := joinURL(c.Endpoint, path)
	c.Logger.Debugf("issuing HTTP GET to AWS metadata API for path: %s", url)
	resp, err := c.Client.Get(url)
	if err != nil {
		return nil, err
	}
	c.Logger.Debugf("got %d from AWS metadata API for path %s", resp.StatusCode, url)
	return resp, nil
}

// Head issues a HEAD request to the given path
func (c *IMDSv1Client) Head(path string) (*http.Response, error) {
	url := joinURL(c.Endpoint, path)
	c.Logger.Debugf("issuing HTTP HEAD to AWS metadata API for path: %s", url)
	resp, err := c.Client.Head(url)
	if err != nil {
		return nil, err
	}
	c.Logger.Debugf("got %d from AWS metadata API for path: %s", resp.StatusCode, url)
	return resp, nil
}
