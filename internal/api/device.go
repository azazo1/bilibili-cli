package api

import (
	"context"
	"net/http"
	"time"
)

func (c *Client) credentialWithDevice(ctx context.Context, credential *Credential) *Credential {
	device, err := c.deviceCredential(ctx)
	if err != nil {
		if c.Logger != nil {
			c.Logger.Debug("获取匿名设备标识失败", "error", err)
		}
		return credential
	}
	if credential == nil {
		return device
	}
	copy := *credential
	if copy.Buvid3 == "" {
		copy.Buvid3 = device.Buvid3
	}
	if copy.Buvid4 == "" {
		copy.Buvid4 = device.Buvid4
	}
	return &copy
}

func (c *Client) deviceCredential(ctx context.Context) (*Credential, error) {
	c.deviceMu.Lock()
	if c.device != nil && time.Now().Before(c.deviceExpires) {
		copy := *c.device
		c.deviceMu.Unlock()
		return &copy, nil
	}
	c.deviceMu.Unlock()

	var data map[string]any
	if err := c.request(ctx, http.MethodGet, "/x/frontend/finger/spi", nil, nil, nil, &data); err != nil {
		return nil, withAction("获取匿名设备标识", err)
	}
	device := &Credential{Buvid3: stringValue(data["b_3"]), Buvid4: stringValue(data["b_4"])}
	if device.Buvid3 == "" {
		return nil, NewError(CodeUpstream, "获取匿名设备标识", "响应缺少 b_3")
	}
	if err := c.activateDevice(ctx, device); err != nil && c.Logger != nil {
		c.Logger.Debug("激活匿名设备失败", "error", err)
	}
	c.deviceMu.Lock()
	c.device = device
	c.deviceExpires = time.Now().Add(24 * time.Hour)
	c.deviceMu.Unlock()
	copy := *device
	return &copy, nil
}
