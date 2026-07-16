package claudeweb

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
	"time"

	http "github.com/bogdanfinn/fhttp"
	"github.com/google/uuid"
)

type AuthMode string

const (
	AuthModeSessionKey AuthMode = "session_key"
	AuthModeFullCookie AuthMode = "full_cookie"
)

type Credentials struct {
	SessionKey        string
	SessionKeyLC      string
	RoutingHint       string
	CFBM              string
	CFUVID            string
	OrgUUID           string
	AuthMode          AuthMode
	BrowserCookie     string
	DeviceID          string
	ActivitySessionID string
	AnonymousID       string
	SSID              string
}

type browserSession struct {
	fullCookie      string
	sessionKey      string
	sessionKeyLC    string
	orgUUID         string
	deviceID        string
	activitySession string
	anonymousID     string
	ssid            string
	ddSessionID     string
	ddAppID         string
	fbp             string
	gclAU           string
	ionVK           string
	createdAtMS     int64
}

func newBrowserSession(credentials Credentials) *browserSession {
	fullCookie := ""
	if credentials.AuthMode == AuthModeFullCookie {
		fullCookie = strings.TrimSpace(credentials.BrowserCookie)
	}
	seed := strings.TrimSpace(credentials.SessionKey)
	if seed == "" {
		seed = cookieValue(fullCookie, "sessionKey")
	}
	createdAtMS := time.Now().UnixMilli()
	sessionKeyLC := firstNonEmpty(
		cookieValue(fullCookie, "sessionKeyLC"),
		strings.TrimSpace(credentials.SessionKeyLC),
	)
	if sessionKeyLC == "" {
		sessionKeyLC = strconv.FormatInt(createdAtMS, 10)
	}

	deviceID := firstNonEmpty(cookieValue(fullCookie, "anthropic-device-id"), credentials.DeviceID)
	if deviceID == "" {
		deviceID = stableUUID(seed, "device")
	}
	activitySessionID := firstNonEmpty(cookieValue(fullCookie, "activitySessionId"), credentials.ActivitySessionID)
	if activitySessionID == "" {
		activitySessionID = stableUUID(seed, "activity")
	}
	anonymousID := firstNonEmpty(cookieValue(fullCookie, "ajs_anonymous_id"), credentials.AnonymousID)
	if anonymousID == "" {
		anonymousID = "claudeai.v1." + stableUUID(seed, "anonymous")
	}
	ssid := firstNonEmpty(cookieValue(fullCookie, "__ssid"), credentials.SSID)
	if ssid == "" {
		ssid = stableUUID(seed, "ssid")
	}
	ddSessionID := ddSessionIDFromCookie(fullCookie)
	if ddSessionID == "" {
		ddSessionID = stableUUID(seed, "dd-session")
	}

	return &browserSession{
		fullCookie:      fullCookie,
		sessionKey:      seed,
		sessionKeyLC:    sessionKeyLC,
		orgUUID:         firstNonEmpty(cookieValue(fullCookie, "lastActiveOrg"), credentials.OrgUUID),
		deviceID:        deviceID,
		activitySession: activitySessionID,
		anonymousID:     anonymousID,
		ssid:            ssid,
		ddSessionID:     ddSessionID,
		ddAppID:         stableUUID(seed, "dd-app"),
		fbp:             firstNonEmpty(cookieValue(fullCookie, "_fbp"), stableFBP(seed, createdAtMS)),
		gclAU:           firstNonEmpty(cookieValue(fullCookie, "_gcl_au"), stableGCLAU(seed, createdAtMS)),
		ionVK:           firstNonEmpty(cookieValue(fullCookie, "ion-vk"), stableUUID(seed, "ion-vk")),
		createdAtMS:     createdAtMS,
	}
}

func (s *browserSession) apply(request *http.Request) {
	request.Header.Set("anthropic-client-platform", "web_claude_ai")
	request.Header.Set("anthropic-device-id", s.deviceID)
	s.addDatadogHeaders(request)
	if s.fullCookie != "" {
		request.Header.Set("Cookie", s.fullCookie)
		return
	}

	add := func(name, value string) {
		if value = strings.TrimSpace(value); value != "" {
			request.AddCookie(&http.Cookie{Name: name, Value: value})
		}
	}
	add("sessionKey", s.sessionKey)
	add("sessionKeyLC", s.sessionKeyLC)
	add("anthropic-device-id", s.deviceID)
	add("activitySessionId", s.activitySession)
	add("ajs_anonymous_id", s.anonymousID)
	add("__ssid", s.ssid)
	add("CH-prefers-color-scheme", "light")
	add("user-sidebar-visible-on-load", "true")
	add("user-sidebar-pinned", "true")
	add("_fbp", s.fbp)
	add("_gcl_au", s.gclAU)
	add("ion-vk", s.ionVK)
	add("_dd_s", s.generatedDDSCookie())
	add("lastActiveOrg", s.orgUUID)
}

func (s *browserSession) generatedDDSCookie() string {
	expiresAtMS := time.Now().Add(15 * time.Minute).UnixMilli()
	return fmt.Sprintf("aid=%s&rum=2&id=%s&created=%d&expire=%d", s.ddAppID, s.ddSessionID, s.createdAtMS, expiresAtMS)
}

func (s *browserSession) addDatadogHeaders(request *http.Request) {
	traceID := randomUint63()
	parentID := randomUint63()
	traceHigh := randomUint63()
	request.Header.Set("traceparent", fmt.Sprintf("00-%016x%016x-%016x-01", traceHigh, traceID, parentID))
	request.Header.Set("tracestate", "dd=s:1;o:rum")
	request.Header.Set("x-datadog-origin", "rum")
	request.Header.Set("x-datadog-parent-id", strconv.FormatUint(parentID, 10))
	request.Header.Set("x-datadog-sampling-priority", "1")
	request.Header.Set("x-datadog-trace-id", strconv.FormatUint(traceID, 10))
}

func cookieValue(cookieHeader, name string) string {
	for _, part := range strings.Split(cookieHeader, ";") {
		keyValue := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(keyValue) == 2 && keyValue[0] == name {
			return keyValue[1]
		}
	}
	return ""
}

func ddSessionIDFromCookie(cookieHeader string) string {
	for _, part := range strings.Split(cookieValue(cookieHeader, "_dd_s"), "&") {
		keyValue := strings.SplitN(part, "=", 2)
		if len(keyValue) == 2 && keyValue[0] == "id" {
			return keyValue[1]
		}
	}
	return ""
}

func stableUUID(seed, label string) string {
	sum := sha256.Sum256([]byte(label + "\x00" + seed))
	var value uuid.UUID
	copy(value[:], sum[:16])
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return value.String()
}

func stableDigits(seed, label string, length int) string {
	sum := sha256.Sum256([]byte(label + "\x00" + seed))
	result := make([]byte, length)
	for index := range result {
		result[index] = '0' + sum[index%len(sum)]%10
	}
	return string(result)
}

func stableFBP(seed string, createdAtMS int64) string {
	return fmt.Sprintf("fb.1.%d.%s", createdAtMS, stableDigits(seed, "fbp", 17))
}

func stableGCLAU(seed string, createdAtMS int64) string {
	return fmt.Sprintf("1.1.%s.%d", stableDigits(seed, "gcl-au", 10), createdAtMS/1000)
}

func randomUint63() uint64 {
	var buffer [8]byte
	if _, err := rand.Read(buffer[:]); err != nil {
		return uint64(time.Now().UnixNano()) & ((1 << 63) - 1)
	}
	return binary.BigEndian.Uint64(buffer[:]) & ((1 << 63) - 1)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
