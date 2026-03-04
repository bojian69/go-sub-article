// Package wechat provides WeChat API client and data models.
package wechat

// AccessTokenResponse represents the response of access_token API (simple mode).
type AccessTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	ErrCode     int    `json:"errcode,omitempty"`
	ErrMsg      string `json:"errmsg,omitempty"`
}

// ComponentTokenRequest represents the request to get component_access_token.
type ComponentTokenRequest struct {
	ComponentAppID        string `json:"component_appid"`
	ComponentAppSecret    string `json:"component_appsecret"`
	ComponentVerifyTicket string `json:"component_verify_ticket"`
}

// ComponentTokenResponse represents the response of component_access_token API.
type ComponentTokenResponse struct {
	ComponentAccessToken string `json:"component_access_token"`
	ExpiresIn            int    `json:"expires_in"`
}

// RefreshAuthorizerTokenRequest represents the request to refresh authorizer_access_token.
type RefreshAuthorizerTokenRequest struct {
	ComponentAppID         string `json:"component_appid"`
	AuthorizerAppID        string `json:"authorizer_appid"`
	AuthorizerRefreshToken string `json:"authorizer_refresh_token"`
}

// RefreshAuthorizerTokenResponse represents the response of refresh authorizer token API.
type RefreshAuthorizerTokenResponse struct {
	AuthorizerAccessToken  string `json:"authorizer_access_token"`
	ExpiresIn              int    `json:"expires_in"`
	AuthorizerRefreshToken string `json:"authorizer_refresh_token"`
}

// BatchGetRequest represents the request to get published articles list.
type BatchGetRequest struct {
	Offset    int `json:"offset"`
	Count     int `json:"count"`
	NoContent int `json:"no_content,omitempty"`
}

// BatchGetResponse represents the response of freepublish_batchget API.
type BatchGetResponse struct {
	TotalCount int                `json:"total_count"`
	ItemCount  int                `json:"item_count"`
	Item       []PublishedArticle `json:"item"`
	ErrCode    int                `json:"errcode,omitempty"`
	ErrMsg     string             `json:"errmsg,omitempty"`
}

// PublishedArticle represents a published article item.
type PublishedArticle struct {
	ArticleID  string          `json:"article_id"`
	Content    *ArticleContent `json:"content,omitempty"`
	UpdateTime int64           `json:"update_time"`
}

// ArticleContent represents the content of an article.
type ArticleContent struct {
	NewsItem []NewsItem `json:"news_item"`
}

// NewsItem represents a single news item in an article.
type NewsItem struct {
	Title              string `json:"title"`
	Author             string `json:"author"`
	Digest             string `json:"digest"`
	Content            string `json:"content"`
	ContentSourceURL   string `json:"content_source_url"`
	ThumbMediaID       string `json:"thumb_media_id"`
	ThumbURL           string `json:"thumb_url"`
	NeedOpenComment    int    `json:"need_open_comment"`
	OnlyFansCanComment int    `json:"only_fans_can_comment"`
	URL                string `json:"url"`
	IsDeleted          bool   `json:"is_deleted"`
}

// GetArticleRequest represents the request to get article details.
type GetArticleRequest struct {
	ArticleID string `json:"article_id"`
}

// GetArticleResponse represents the response of freepublishGetarticle API.
type GetArticleResponse struct {
	NewsItem []NewsItem `json:"news_item"`
	ErrCode  int        `json:"errcode,omitempty"`
	ErrMsg   string     `json:"errmsg,omitempty"`
}

// ErrorResponse represents a WeChat API error response.
type ErrorResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

// IsSuccess checks if the error response indicates success.
func (e *ErrorResponse) IsSuccess() bool {
	return e.ErrCode == 0
}

// Common WeChat API error codes
const (
	ErrCodeSuccess                = 0
	ErrCodeInvalidCredential      = 40001
	ErrCodeAccessTokenExpired     = 42001
	ErrCodeAPIUnauthorized        = 48001
	ErrCodeRateLimited            = 45009
	ErrCodeInvalidArticleID       = 53600
	// Subscription message error codes
	ErrCodeInvalidOpenID          = 40003 // OpenID invalid
	ErrCodeTemplateNotFound       = 40037 // Template ID not found
	ErrCodeSubscriptionExpired    = 43101 // User rejected or subscription expired
	ErrCodeDataFieldTooLong       = 47003 // Data field value too long
	ErrCodeSubscriptionRateLimit  = 45009 // Rate limit exceeded
)

// IsTokenExpiredError checks if the error code indicates token expiration.
func IsTokenExpiredError(errCode int) bool {
	return errCode == ErrCodeInvalidCredential || errCode == ErrCodeAccessTokenExpired
}

// IsRetryableError checks if the error is retryable.
func IsRetryableError(errCode int) bool {
	// Network errors and rate limiting are retryable
	return errCode == ErrCodeRateLimited
}

// SendSubscriptionMessageRequest represents the request to send subscription message.
type SendSubscriptionMessageRequest struct {
	ToUser           string                 `json:"touser"`
	TemplateID       string                 `json:"template_id"`
	Page             string                 `json:"page,omitempty"`
	Data             map[string]interface{} `json:"data"`
	MiniprogramState string                 `json:"miniprogram_state,omitempty"`
	Lang             string                 `json:"lang,omitempty"`
}

// SendSubscriptionMessageResponse represents the response of subscription message send API.
type SendSubscriptionMessageResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
	MsgID   int64  `json:"msgid,omitempty"`
}

// GetTemplateListResponse represents the response of get template list API.
type GetTemplateListResponse struct {
	ErrCode int                      `json:"errcode"`
	ErrMsg  string                   `json:"errmsg"`
	Data    []SubscriptionTemplate   `json:"data"`
}

// SubscriptionTemplate represents a subscription message template.
type SubscriptionTemplate struct {
	PriTmplID string `json:"priTmplId"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Example   string `json:"example"`
	Type      int    `json:"type"`
}
