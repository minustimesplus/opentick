// Vikunja is a to-do list application to facilitate your life.
// Copyright 2018-present Vikunja and contributors. All rights reserved.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package icalfeed

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"code.vikunja.io/api/pkg/utils"
	"code.vikunja.io/api/pkg/version"

	"golang.org/x/sync/singleflight"
)

const (
	// maxFeedSize caps how much of a feed is read; larger calendars are rejected.
	maxFeedSize = 10 << 20
	cacheTTL    = 15 * time.Minute
	// Failures are cached briefly so a broken feed is not hammered on every
	// calendar navigation, but a fixed one recovers quickly.
	errorCacheTTL   = time.Minute
	maxCacheEntries = 512
)

// ErrInvalidURL is returned for URLs that are not http(s) or webcal(s).
var ErrInvalidURL = errors.New("the calendar url must be an http, https or webcal url")

// FetchError describes why a feed could not be loaded.
type FetchError struct {
	Reason string
}

func (e *FetchError) Error() string { return e.Reason }

// NormalizeURL validates a subscription URL and rewrites webcal(s):// to https://.
func NormalizeURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return "", ErrInvalidURL
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		u.Scheme = strings.ToLower(u.Scheme)
	case "webcal", "webcals":
		u.Scheme = "https"
	default:
		return "", ErrInvalidURL
	}
	return u.String(), nil
}

type cacheEntry struct {
	feed    *Feed
	err     error
	expires time.Time
}

var (
	cacheMu sync.Mutex
	cache   = map[string]*cacheEntry{}
	flight  singleflight.Group
)

// Fetch loads and parses the feed at rawURL, serving it from an in-memory
// cache for a few minutes. Requests go through the SSRF-guarded client, so
// feeds on private networks are only reachable when the admin allows it.
func Fetch(ctx context.Context, rawURL string) (*Feed, error) {
	feedURL, err := NormalizeURL(rawURL)
	if err != nil {
		return nil, err
	}

	cacheMu.Lock()
	if e, ok := cache[feedURL]; ok && time.Now().Before(e.expires) {
		cacheMu.Unlock()
		return e.feed, e.err
	}
	cacheMu.Unlock()

	v, err, _ := flight.Do(feedURL, func() (any, error) {
		// Detached from the caller so one aborted request does not fail the
		// other callers sharing this fetch.
		feed, err := download(context.WithoutCancel(ctx), feedURL)
		store(feedURL, feed, err)
		return feed, err
	})
	if err != nil {
		return nil, err
	}
	return v.(*Feed), nil
}

// Invalidate drops a cached feed, so the next Fetch reloads it.
func Invalidate(rawURL string) {
	feedURL, err := NormalizeURL(rawURL)
	if err != nil {
		return
	}
	cacheMu.Lock()
	delete(cache, feedURL)
	cacheMu.Unlock()
}

func store(feedURL string, feed *Feed, err error) {
	ttl := cacheTTL
	if err != nil {
		ttl = errorCacheTTL
	}
	now := time.Now()

	cacheMu.Lock()
	defer cacheMu.Unlock()
	if len(cache) >= maxCacheEntries {
		for k, e := range cache {
			if now.After(e.expires) {
				delete(cache, k)
			}
		}
		// Still full: drop an arbitrary entry rather than grow unbounded.
		for k := range cache {
			if len(cache) < maxCacheEntries {
				break
			}
			delete(cache, k)
		}
	}
	cache[feedURL] = &cacheEntry{feed: feed, err: err, expires: now.Add(ttl)}
}

func download(ctx context.Context, feedURL string) (*Feed, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return nil, &FetchError{Reason: "invalid calendar url"}
	}
	req.Header.Set("Accept", "text/calendar, */*;q=0.5")
	req.Header.Set("User-Agent", "Vikunja/"+version.Version)

	res, err := utils.NewSSRFSafeHTTPClient().Do(req)
	if err != nil {
		return nil, &FetchError{Reason: fmt.Sprintf("could not reach the calendar server: %s", describeRequestError(err))}
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, &FetchError{Reason: fmt.Sprintf("the calendar server responded with %s", res.Status)}
	}

	data, err := io.ReadAll(io.LimitReader(res.Body, maxFeedSize+1))
	if err != nil {
		return nil, &FetchError{Reason: "could not read the calendar"}
	}
	if len(data) > maxFeedSize {
		return nil, &FetchError{Reason: fmt.Sprintf("the calendar is larger than %d MB", maxFeedSize>>20)}
	}

	feed, err := Parse(data)
	if err != nil {
		if errors.Is(err, ErrNotACalendar) {
			return nil, &FetchError{Reason: "the url does not point to an iCalendar (.ics) feed"}
		}
		return nil, &FetchError{Reason: "the calendar could not be parsed"}
	}
	return feed, nil
}

// describeRequestError strips the URL from transport errors: it may carry a
// secret token and is already known to the caller.
func describeRequestError(err error) string {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		err = urlErr.Err
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timed out"
	}
	return err.Error()
}
