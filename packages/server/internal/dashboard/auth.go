package dashboard

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"github.com/cerebellum-labs/cerebellum/packages/server/internal/httperr"
)

const (
	// __Host- makes the browser require Secure and Path=/ and refuse a
	// Domain, so no other subdomain can set or overwrite this cookie.
	sessionCookie = "__Host-cerebellum-admin"
	sessionTTL    = 7 * 24 * time.Hour
)

// sessions issues and checks the admin cookie: its expiry, signed with
// HMAC-SHA256. Nothing is stored on the server, so a session can't be
// revoked one by one; changing SESSION_SECRET ends them all.
type sessions struct {
	secret []byte
}

func (s sessions) issue(now time.Time) string {
	payload := strconv.FormatInt(now.Add(sessionTTL).Unix(), 10)
	return payload + "." + base64.RawURLEncoding.EncodeToString(s.sign(payload))
}

func (s sessions) valid(value string, now time.Time) bool {
	payload, sig, ok := strings.Cut(value, ".")
	if !ok {
		return false
	}
	got, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil || !hmac.Equal(got, s.sign(payload)) {
		return false
	}
	expires, err := strconv.ParseInt(payload, 10, 64)
	return err == nil && now.Unix() < expires
}

func (s sessions) sign(payload string) []byte {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(payload))
	return mac.Sum(nil)
}

// passwordMatches compares in constant time: hashing first gives both sides
// the same length, so not even the password's length leaks.
func passwordMatches(given, want string) bool {
	g := sha256.Sum256([]byte(given))
	w := sha256.Sum256([]byte(want))
	return subtle.ConstantTimeCompare(g[:], w[:]) == 1
}

// newLoginLimiter allows 5 attempts at once and then one every 6 seconds,
// for everyone together: enough for a person, far too slow to guess.
func newLoginLimiter() *rate.Limiter {
	return rate.NewLimiter(rate.Every(6*time.Second), 5)
}

type loginRequest struct {
	Password string `json:"password"`
}

type okResponse struct {
	OK bool `json:"ok"`
}

func (h *Handler) login(c *gin.Context) {
	if !h.loginLimiter.Allow() {
		httperr.TooManyRequests(c, "too many sign-in attempts, try again in a minute")
		return
	}
	var req loginRequest
	if !httperr.BindJSON(c, &req) {
		return
	}
	if !passwordMatches(req.Password, h.adminPassword) {
		httperr.Unauthorized(c, "wrong password")
		return
	}
	h.setSessionCookie(c, h.sessions.issue(h.now()), int(sessionTTL/time.Second))
	c.JSON(http.StatusOK, okResponse{OK: true})
}

func (h *Handler) logout(c *gin.Context) {
	h.setSessionCookie(c, "", -1)
	c.JSON(http.StatusOK, okResponse{OK: true})
}

func (h *Handler) setSessionCookie(c *gin.Context, value string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     sessionCookie,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		Secure:   true,
		HttpOnly: true,
		// Sent with requests from the dashboard's subdomain, which is the
		// same site, and never with requests started by another site.
		SameSite: http.SameSiteLaxMode,
	})
}

// requireAdmin lets a request through only with a valid admin cookie.
func (h *Handler) requireAdmin(c *gin.Context) {
	cookie, err := c.Cookie(sessionCookie)
	if err != nil || !h.sessions.valid(cookie, h.now()) {
		httperr.Unauthorized(c, "sign in to change devices")
		return
	}
	c.Next()
}

// checkOrigin refuses a request that changes something when a browser sent
// it from a page outside the dashboard. SameSite=Lax already keeps other
// sites' requests from carrying the cookie, but other subdomains of the same
// site still could. Clients other than browsers send no Origin and pass.
func (h *Handler) checkOrigin(c *gin.Context) {
	switch c.Request.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		c.Next()
		return
	}
	origin := c.GetHeader("Origin")
	if origin == "" || h.originAllowed(origin, c.Request.Host) {
		c.Next()
		return
	}
	httperr.Forbidden(c, "requests from this origin are not allowed")
}

// originAllowed accepts the dashboard's own origin and the API's own host.
func (h *Handler) originAllowed(origin, host string) bool {
	if h.allowedOrigin != "" && origin == h.allowedOrigin {
		return true
	}
	u, err := url.Parse(origin)
	return err == nil && u.Host != "" && u.Host == host
}
