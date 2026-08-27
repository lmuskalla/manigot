// Package notify publishes push notifications to an ntfy server
// (https://docs.ntfy.sh/) for unattended host-side events — today, when an
// `mg jdi` run stops or a later run finds a previous one crashed. It is the
// phone-reaching complement to the TUI's terminal bell: a notification that
// gets attention when nobody is watching the terminal (a VPS running `mg jdi`
// unattended).
//
// Configuration is opt-in and lives in manigot's .env (read via
// config.EnvValue): NTFY_TOPIC activates the feature (unset = strict no-op,
// byte-for-byte identical behavior to not having the feature at all),
// NTFY_URL defaults to DefaultURL, and NTFY_TOKEN is optional and sent as a
// Bearer Authorization header. ntfy topics are effectively a password, so
// Publish's errors never include the request URL — a caller logging them
// (see mg jdi's stderr warnings) must not leak the URL+token combination.
package notify

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/lmuskalla/manigot/internal/config"
)

// DefaultURL is the ntfy server used when NTFY_URL is unset.
const DefaultURL = "https://ntfy.sh"

// defaultTimeout bounds a single publish request — a notification is
// best-effort and must never hang the caller, so the client gives up after
// roughly this long.
const defaultTimeout = 10 * time.Second

// Message is an ntfy publish request (https://docs.ntfy.sh/publish/).
type Message struct {
	// Title is shown as the notification's headline.
	Title string
	// Message is the notification body.
	Message string
	// Priority is 1 (min) … 5 (max); 0 leaves the header off and the server
	// uses its default (3).
	Priority int
	// Tags are emoji shortcodes (e.g. "white_check_mark", "warning") shown
	// next to the notification.
	Tags []string
}

// Client publishes messages to one ntfy topic.
type Client struct {
	// URL is the ntfy server base URL (e.g. https://ntfy.sh), no trailing
	// slash.
	URL string
	// Topic is the ntfy topic messages are published to. Empty disables the
	// client: Publish becomes a strict no-op (see Enabled).
	Topic string
	// Token, when non-empty, is sent as a "Bearer <token>" Authorization
	// header (the ntfy access-token form).
	Token string
	// HTTP is the client used for the request. FromConfig always supplies one
	// with defaultTimeout; a nil value behaves like http.DefaultClient.
	HTTP *http.Client
}

// FromConfig builds a Client from manigot's .env: NTFY_URL (default
// DefaultURL), NTFY_TOPIC (the activation key) and NTFY_TOKEN (optional
// access token).
func FromConfig() Client {
	base := strings.TrimRight(strings.TrimSpace(config.EnvValue("NTFY_URL")), "/")
	if base == "" {
		base = DefaultURL
	}
	return Client{
		URL:   base,
		Topic: strings.TrimSpace(config.EnvValue("NTFY_TOPIC")),
		Token: strings.TrimSpace(config.EnvValue("NTFY_TOKEN")),
		HTTP:  &http.Client{Timeout: defaultTimeout},
	}
}

// Enabled reports whether the client is configured to send: NTFY_TOPIC is
// set. Callers use it to skip building a message at all when notifications
// are off; Publish itself is also a no-op in that case.
func (c Client) Enabled() bool {
	return c.Topic != ""
}

// Publish POSTs msg to {URL}/{Topic}. It is a strict no-op when the client
// is not enabled (NTFY_TOPIC unset): nothing is sent and no error is
// returned. Any other failure (transport, non-2xx response) is returned to
// the caller — never wrapped with the topic or token, since the topic is
// effectively a password and the caller will likely log the error.
func (c Client) Publish(msg Message) error {
	if !c.Enabled() {
		return nil
	}

	req, err := http.NewRequest(http.MethodPost, c.URL+"/"+c.Topic, strings.NewReader(msg.Message))
	if err != nil {
		// Strip the request URL from the error — a parse failure (e.g. a
		// malformed NTFY_URL) embeds the full URL, which contains the topic
		// (effectively a password).
		var uerr *url.Error
		if errors.As(err, &uerr) && uerr.Err != nil {
			return fmt.Errorf("ntfy: could not build request: %v", uerr.Err)
		}
		return fmt.Errorf("ntfy: could not build request: %v", err)
	}
	req.Header.Set("Content-Type", "text/plain")
	if msg.Title != "" {
		req.Header.Set("Title", msg.Title)
	}
	if msg.Priority > 0 {
		req.Header.Set("Priority", strconv.Itoa(msg.Priority))
	}
	if len(msg.Tags) > 0 {
		req.Header.Set("Tags", strings.Join(msg.Tags, ","))
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	httpc := c.HTTP
	if httpc == nil {
		httpc = http.DefaultClient
	}
	resp, err := httpc.Do(req)
	if err != nil {
		// Strip the request URL from the error — http.Client errors embed it,
		// and the URL contains the topic, which is effectively a password.
		var uerr *url.Error
		if errors.As(err, &uerr) && uerr.Err != nil {
			return fmt.Errorf("ntfy: request failed: %v", uerr.Err)
		}
		return fmt.Errorf("ntfy: request failed: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body) // drain so the connection can be reused
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ntfy: server returned %s", resp.Status)
	}
	return nil
}
