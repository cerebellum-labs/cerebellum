package dashboard

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/cerebellum-labs/cerebellum/packages/server/internal/db"
	"github.com/cerebellum-labs/cerebellum/packages/server/internal/httperr"
	"github.com/cerebellum-labs/cerebellum/packages/server/internal/view"
)

const (
	maxKeyLen  = 64
	maxNameLen = 100
	// uniqueViolation is PostgreSQL's SQLSTATE for a duplicate key.
	uniqueViolation = "23505"
)

// keyPattern matches the database's check on devices.key.
var keyPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// newToken returns a device token, 32 random bytes as URL-safe base64, and
// the hash the database keeps of it.
func newToken() (token string, hash []byte) {
	b := make([]byte, 32)
	_, _ = rand.Read(b) // crypto/rand.Read never fails; it crashes the program instead.
	token = base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(token))
	return token, sum[:]
}

type devicesResponse struct {
	Devices []view.Device `json:"devices"`
}

func (h *Handler) listDevices(c *gin.Context) {
	devices, err := h.q.ListDevices(c.Request.Context())
	if err != nil {
		h.internalError(c, "list devices", err)
		return
	}
	now := h.now()
	views := make([]view.Device, len(devices))
	for i, d := range devices {
		views[i] = view.NewDevice(d, now)
	}
	c.JSON(http.StatusOK, devicesResponse{Devices: views})
}

type createDeviceRequest struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

type createDeviceResponse struct {
	Device view.Device `json:"device"`
	// Token is shown once; the server keeps only its hash.
	Token string `json:"token"`
}

func (h *Handler) createDevice(c *gin.Context) {
	var req createDeviceRequest
	if !httperr.BindJSON(c, &req) {
		return
	}
	key := strings.TrimSpace(req.Key)
	if len(key) > maxKeyLen || !keyPattern.MatchString(key) {
		httperr.BadRequest(c, "use lowercase letters, digits and single dashes for the key, like fruit-sorter-2")
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = key
	}
	if utf8.RuneCountInString(name) > maxNameLen {
		httperr.BadRequest(c, "keep the name under 100 characters")
		return
	}

	token, hash := newToken()
	device, err := h.q.CreateDevice(c.Request.Context(), db.CreateDeviceParams{Key: key, Name: name, TokenHash: hash})
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == uniqueViolation {
		httperr.Conflict(c, "a device with the key "+key+" already exists")
		return
	}
	if err != nil {
		h.internalError(c, "create device", err)
		return
	}

	view := view.NewDevice(device, h.now())
	h.publishDevice(c.Request.Context(), view)
	c.JSON(http.StatusCreated, createDeviceResponse{Device: view, Token: token})
}

type rotateTokenResponse struct {
	Token string `json:"token"`
}

func (h *Handler) rotateToken(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	token, hash := newToken()
	device, err := h.q.RotateDeviceToken(c.Request.Context(), db.RotateDeviceTokenParams{ID: id, TokenHash: hash})
	if errors.Is(err, pgx.ErrNoRows) {
		httperr.Missing(c, "this device no longer exists")
		return
	}
	if err != nil {
		h.internalError(c, "rotate device token", err)
		return
	}
	h.publishDevice(c.Request.Context(), view.NewDevice(device, h.now()))
	c.JSON(http.StatusOK, rotateTokenResponse{Token: token})
}

func (h *Handler) deleteDevice(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	device, err := h.q.DeleteDevice(c.Request.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		httperr.Missing(c, "this device no longer exists")
		return
	}
	if err != nil {
		h.internalError(c, "delete device", err)
		return
	}
	h.publishDevice(c.Request.Context(), view.NewDevice(device, h.now()))
	c.JSON(http.StatusOK, okResponse{OK: true})
}

// pathID parses the :id parameter. On failure it answers 404, since no
// resource has a malformed id, and returns false.
func pathID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httperr.Missing(c, "there is nothing with this id")
		return uuid.UUID{}, false
	}
	return id, true
}
