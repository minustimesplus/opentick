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

package inboundmail

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"strconv"
	"time"

	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/log"
	"code.vikunja.io/api/pkg/models"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

const (
	fetchBatchSize = 25
	// Even with IDLE the mailbox is searched now and then, in case a notification got lost.
	idleRecheck = 10 * time.Minute
	maxBackoff  = 5 * time.Minute
)

// Start reads the configured mailbox in the background until ctx ends. It does nothing
// when inbound mail is turned off.
func Start(ctx context.Context) {
	if !models.InboundMailEnabled() {
		if config.InboundMailEnabled.GetBool() {
			log.Errorf("[inbound mail] inboundmail.enabled is set, but inboundmail.host or inboundmail.address is missing; not reading mail")
		}
		return
	}
	go run(ctx)
}

func run(ctx context.Context) {
	backoff := 10 * time.Second
	for {
		started := time.Now()
		err := readMailbox(ctx)
		if ctx.Err() != nil {
			return
		}
		if time.Since(started) > maxBackoff {
			backoff = 10 * time.Second
		}
		log.Errorf("[inbound mail] mailbox connection ended: %v; reconnecting in %s", err, backoff)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, maxBackoff)
	}
}

func pollInterval() time.Duration {
	secs := config.InboundMailPollInterval.GetInt()
	if secs < 10 {
		secs = 10
	}
	return time.Duration(secs) * time.Second
}

func dial(newMail chan<- struct{}) (*imapclient.Client, error) {
	host := config.InboundMailHost.GetString()
	port := config.InboundMailPort.GetInt()
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	opts := &imapclient.Options{
		TLSConfig: &tls.Config{
			ServerName:         host,
			InsecureSkipVerify: config.InboundMailSkipTLSVerify.GetBool(), //nolint:gosec // opt-in for self-signed servers
			MinVersion:         tls.VersionTLS12,
		},
		UnilateralDataHandler: &imapclient.UnilateralDataHandler{
			Mailbox: func(data *imapclient.UnilateralDataMailbox) {
				if data.NumMessages != nil {
					select {
					case newMail <- struct{}{}:
					default:
					}
				}
			},
		},
	}
	if port == 993 {
		return imapclient.DialTLS(addr, opts)
	}
	return imapclient.DialStartTLS(addr, opts)
}

// readMailbox holds one connection: it applies every unread mail, then waits
// for more with IDLE (or by polling) until the connection fails or ctx ends.
func readMailbox(ctx context.Context) error {
	newMail := make(chan struct{}, 1)
	c, err := dial(newMail)
	if err != nil {
		return err
	}
	defer c.Close()
	go func() {
		select {
		case <-ctx.Done():
			_ = c.Close()
		case <-c.Closed():
		}
	}()

	if err := c.Login(config.InboundMailUsername.GetString(), config.InboundMailPassword.GetString()).Wait(); err != nil {
		return err
	}
	if _, err := c.Select(config.InboundMailMailbox.GetString(), nil).Wait(); err != nil {
		return err
	}
	canIdle := c.Caps().Has(imap.CapIdle) || c.Caps().Has(imap.CapIMAP4rev2)
	log.Infof("[inbound mail] reading %s on %s (idle: %t)", config.InboundMailMailbox.GetString(), config.InboundMailHost.GetString(), canIdle)

	for {
		if err := processUnread(c); err != nil {
			return err
		}
		if !canIdle {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-c.Closed():
				return errors.New("server closed the connection")
			case <-time.After(pollInterval()):
			}
			continue
		}
		idle, err := c.Idle()
		if err != nil {
			return err
		}
		select {
		case <-ctx.Done():
		case <-c.Closed():
		case <-newMail:
		case <-time.After(idleRecheck):
		}
		if err := idle.Close(); err != nil {
			return err
		}
		if err := idle.Wait(); err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
}

// processUnread applies every unread mail and marks it read. A mail that fails
// for a temporary reason stays unread and is tried again next time.
func processUnread(c *imapclient.Client) error {
	found, err := c.UIDSearch(&imap.SearchCriteria{NotFlag: []imap.Flag{imap.FlagSeen}}, nil).Wait()
	if err != nil {
		return err
	}
	uids := found.AllUIDs()
	section := &imap.FetchItemBodySection{Peek: true}
	for start := 0; start < len(uids); start += fetchBatchSize {
		batch := uids[start:min(start+fetchBatchSize, len(uids))]
		msgs, err := c.Fetch(imap.UIDSetNum(batch...), &imap.FetchOptions{
			UID:         true,
			RFC822Size:  true,
			BodySection: []*imap.FetchItemBodySection{section},
		}).Collect()
		if err != nil {
			return err
		}
		done := make([]imap.UID, 0, len(msgs))
		for _, msg := range msgs {
			if msg.RFC822Size > maxMailBytes {
				log.Warningf("[inbound mail] skipping mail %d: %d bytes is too large", msg.UID, msg.RFC822Size)
				done = append(done, msg.UID)
				continue
			}
			if err := Process(msg.FindBodySection(section)); err != nil {
				log.Errorf("[inbound mail] could not apply mail %d, will retry: %s", msg.UID, err)
				continue
			}
			done = append(done, msg.UID)
		}
		if len(done) == 0 {
			continue
		}
		if err := c.Store(imap.UIDSetNum(done...), &imap.StoreFlags{
			Op:     imap.StoreFlagsAdd,
			Silent: true,
			Flags:  []imap.Flag{imap.FlagSeen},
		}, nil).Close(); err != nil {
			return err
		}
	}
	return nil
}
