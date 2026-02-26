package settings

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/asifthewebguy/webhostmanager-api/internal/setup"
	"github.com/asifthewebguy/webhostmanager-api/pkg/crypto"
	"github.com/asifthewebguy/webhostmanager-api/pkg/response"
)

// exposedKeys defines the complete list of settings keys this handler allows
// reading and writing. Order controls the order in the List response.
var exposedKeys = []string{
	"cloudflare.api_token",
	"cloudflare.zone_id",
	"cloudflare.account_id",
	"smtp.host",
	"smtp.port",
	"smtp.user",
	"smtp.password",
	"smtp.from",
	"notifications.slack_webhook",
	"notifications.discord_webhook",
	"proxy.type",
}

// encryptedKeys lists which keys must be encrypted at rest and must never be
// returned in plaintext via the API.
var encryptedKeys = map[string]bool{
	"cloudflare.api_token":         true,
	"smtp.password":                true,
	"notifications.slack_webhook":  true,
	"notifications.discord_webhook": true,
}

// exposedSet is used for O(1) key validation on writes.
var exposedSet = func() map[string]bool {
	m := make(map[string]bool, len(exposedKeys))
	for _, k := range exposedKeys {
		m[k] = true
	}
	return m
}()

// SettingItem is the JSON shape returned by List.
type SettingItem struct {
	Key         string `json:"key"`
	Value       string `json:"value"`      // always "" for encrypted keys
	IsEncrypted bool   `json:"is_encrypted"`
	IsSet       bool   `json:"is_set"`     // true if a non-empty value exists in DB
}

// UpdateSettingRequest is the body for PUT /settings.
type UpdateSettingRequest struct {
	Key   string `json:"key"   binding:"required"`
	Value string `json:"value"` // empty string = no-op for encrypted keys
}

// Handler exposes settings read/write endpoints.
type Handler struct {
	setupSvc *setup.Service
	encKey   string
}

func NewHandler(setupSvc *setup.Service, encKey string) *Handler {
	return &Handler{setupSvc: setupSvc, encKey: encKey}
}

// List godoc — GET /api/v1/settings
// Returns all exposed settings. Encrypted settings always return value="".
func (h *Handler) List(c *gin.Context) {
	items := make([]SettingItem, 0, len(exposedKeys))
	for _, key := range exposedKeys {
		item := SettingItem{Key: key, IsEncrypted: encryptedKeys[key]}
		val, _, err := h.setupSvc.GetSetting(key)
		if err == nil {
			item.IsSet = val != ""
			if !encryptedKeys[key] {
				item.Value = val
			}
		}
		items = append(items, item)
	}
	c.JSON(http.StatusOK, response.OK(items))
}

// Update godoc — PUT /api/v1/settings
// Updates a single setting. For encrypted keys, a blank value is a no-op.
func (h *Handler) Update(c *gin.Context) {
	var req UpdateSettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(err.Error()))
		return
	}
	if !exposedSet[req.Key] {
		c.JSON(http.StatusBadRequest, response.Error("unknown setting key: "+req.Key))
		return
	}
	if encryptedKeys[req.Key] {
		if req.Value == "" {
			// No-op: don't overwrite an existing encrypted value with blank.
			c.JSON(http.StatusOK, response.OK(gin.H{"message": "no change"}))
			return
		}
		enc, err := crypto.Encrypt(req.Value, h.encKey)
		if err != nil {
			c.JSON(http.StatusInternalServerError, response.Error("encryption failed"))
			return
		}
		if err := h.setupSvc.SaveSetting(req.Key, enc, true); err != nil {
			c.JSON(http.StatusInternalServerError, response.Error("failed to save setting"))
			return
		}
	} else {
		if err := h.setupSvc.SaveSetting(req.Key, req.Value, false); err != nil {
			c.JSON(http.StatusInternalServerError, response.Error("failed to save setting"))
			return
		}
	}
	c.JSON(http.StatusOK, response.OK(gin.H{"message": "setting updated"}))
}
