package monitoring

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	probing "github.com/prometheus-community/pro-bing"
)

type Checker interface {
	Check(ctx context.Context, target *Target) *CheckResult
}

// ICMPChecker untuk ping check
type ICMPChecker struct{}

func (c *ICMPChecker) Check(ctx context.Context, target *Target) *CheckResult {
	result := &CheckResult{
		TargetID:  target.ID,
		CheckedAt: time.Now(),
	}

	timeout := time.Duration(target.Timeout) * time.Second
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	pinger, err := probing.NewPinger(target.Host)
	if err != nil {
		result.Status = StatusDown
		result.Error = fmt.Sprintf("failed to create pinger: %v", err)
		return result
	}

	pinger.Count = 3
	pinger.Timeout = timeout
	pinger.SetPrivileged(false)

	err = pinger.Run()
	if err != nil {
		result.Status = StatusDown
		result.Error = err.Error()
		return result
	}

	stats := pinger.Statistics()
	if stats.PacketsRecv == 0 {
		result.Status = StatusDown
		result.Error = "no packets received"
		return result
	}

	result.Status = StatusUp
	result.ResponseTime = stats.AvgRtt
	return result
}

// HTTPChecker untuk HTTP/HTTPS check
type HTTPChecker struct {
	client *http.Client
}

func NewHTTPChecker() *HTTPChecker {
	return &HTTPChecker{
		client: &http.Client{
			Timeout: 10 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (c *HTTPChecker) Check(ctx context.Context, target *Target) *CheckResult {
	result := &CheckResult{
		TargetID:  target.ID,
		CheckedAt: time.Now(),
	}

	url := target.Host
	if target.Path != "" {
		url = url + target.Path
	}

	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		result.Status = StatusDown
		result.Error = err.Error()
		return result
	}

	resp, err := c.client.Do(req)
	if err != nil {
		result.Status = StatusDown
		result.Error = err.Error()
		return result
	}
	defer resp.Body.Close()

	result.ResponseTime = time.Since(start)

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		result.Status = StatusUp
	} else {
		result.Status = StatusDown
		result.Error = fmt.Sprintf("HTTP status: %d", resp.StatusCode)
	}

	return result
}

// TCPChecker untuk TCP port check
type TCPChecker struct{}

func (c *TCPChecker) Check(ctx context.Context, target *Target) *CheckResult {
	result := &CheckResult{
		TargetID:  target.ID,
		CheckedAt: time.Now(),
	}

	timeout := time.Duration(target.Timeout) * time.Second
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	address := fmt.Sprintf("%s:%d", target.Host, target.Port)
	start := time.Now()

	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		result.Status = StatusDown
		result.Error = err.Error()
		return result
	}
	defer conn.Close()

	result.Status = StatusUp
	result.ResponseTime = time.Since(start)
	return result
}

// GetChecker return appropriate checker based on type
func GetChecker(checkType CheckType) Checker {
	switch checkType {
	case CheckTypeICMP:
		return &ICMPChecker{}
	case CheckTypeHTTP:
		return NewHTTPChecker()
	case CheckTypeTCP:
		return &TCPChecker{}
	default:
		return &ICMPChecker{}
	}
}
