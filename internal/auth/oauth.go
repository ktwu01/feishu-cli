package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// tokenResponse 飞书 token 端点响应
type tokenResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	TokenType        string `json:"token_type"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshExpiresIn int    `json:"refresh_token_expires_in"`
	Scope            string `json:"scope"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// RefreshAccessToken 用 refresh_token 刷新 access_token
func RefreshAccessToken(refreshToken, appID, appSecret, baseURL string) (*TokenStore, error) {
	tokenURL := baseURL + "/open-apis/authen/v2/oauth/token"

	body := map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": refreshToken,
		"client_id":     appID,
		"client_secret": appSecret,
	}

	return doTokenRequest(tokenURL, body)
}

// doTokenRequest 执行 token 请求
func doTokenRequest(tokenURL string, body map[string]string) (*TokenStore, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	httpClient := &http.Client{Timeout: 10 * time.Second}
	resp, err := httpClient.Post(tokenURL, "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("请求 token 端点失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token 端点返回 HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var tokenResp tokenResponse
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return nil, fmt.Errorf("解析 token 响应失败: %w", err)
	}

	if tokenResp.Error != "" {
		return nil, fmt.Errorf("获取 token 失败: %s - %s", tokenResp.Error, tokenResp.ErrorDescription)
	}

	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("token 响应中缺少 access_token")
	}

	now := time.Now()
	store := &TokenStore{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		TokenType:    tokenResp.TokenType,
		ExpiresAt:    now.Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
		Scope:        tokenResp.Scope,
	}

	if tokenResp.RefreshExpiresIn > 0 {
		store.RefreshExpiresAt = now.Add(time.Duration(tokenResp.RefreshExpiresIn) * time.Second)
	}

	return store, nil
}
