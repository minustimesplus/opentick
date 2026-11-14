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

package cmd

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"code.vikunja.io/api/pkg/models"

	"github.com/spf13/cobra"
)

// The Personal CRM desktop app keeps its data in SQLite. This command reads
// that file on the machine it runs on and sends everything to a server over
// the API, so it works whether the server is local or hosted.

const personalCRMBatchSize = 100

var (
	importPersonalCRMDir   string
	importPersonalCRMURL   string
	importPersonalCRMToken string
	importPersonalCRMDry   bool
)

func init() {
	home, _ := os.UserHomeDir()
	importPersonalCRMCmd.Flags().StringVar(&importPersonalCRMDir, "dir", filepath.Join(home, ".local", "share", "personalcrm"), "The Personal CRM data directory (holds crm.sqlite3 and images/).")
	importPersonalCRMCmd.Flags().StringVar(&importPersonalCRMURL, "url", "", "The server to import into, e.g. https://tasks.example.com.")
	importPersonalCRMCmd.Flags().StringVar(&importPersonalCRMToken, "token", os.Getenv("OPENTICK_TOKEN"), "An API token of the account to import into (or set OPENTICK_TOKEN). It needs the contacts permissions.")
	importPersonalCRMCmd.Flags().BoolVar(&importPersonalCRMDry, "dry-run", false, "Only report what would be imported.")
	rootCmd.AddCommand(importPersonalCRMCmd)
}

var importPersonalCRMCmd = &cobra.Command{
	Use:   "import-personalcrm",
	Short: "Import the contacts of the Personal CRM desktop app into a server",
	Long: `Reads the Personal CRM desktop app's local database and sends its contacts,
with their history, notes, tags, groups, locked fields and photos, to a server
through the API. Running it again updates the contacts it imported before
instead of adding them a second time.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		cmd.SilenceUsage = true
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		return runImportPersonalCRM(ctx, cmd.OutOrStdout())
	},
}

type personalCRMPhoto struct {
	importID string
	path     string
}

func runImportPersonalCRM(ctx context.Context, out io.Writer) error {
	bundle, photos, err := readPersonalCRM(ctx, importPersonalCRMDir)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "Found %d contacts, %d groups, %d tags and %d photos in %s\n", len(bundle.Contacts), len(bundle.Groups), len(bundle.Tags), len(photos), importPersonalCRMDir)
	if importPersonalCRMDry {
		return nil
	}
	if importPersonalCRMURL == "" || importPersonalCRMToken == "" {
		return errors.New("--url and --token are required (or pass --dry-run)")
	}
	base := strings.TrimRight(importPersonalCRMURL, "/") + "/api/v2"
	client := &http.Client{Timeout: 5 * time.Minute}

	ids := map[string]int64{}
	created, updated := 0, 0
	for start := 0; start < len(bundle.Contacts); start += personalCRMBatchSize {
		part := &models.ContactBundle{
			Contacts: bundle.Contacts[start:min(start+personalCRMBatchSize, len(bundle.Contacts))],
			Groups:   bundle.Groups,
			Tags:     bundle.Tags,
		}
		body, err := json.Marshal(part)
		if err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/contacts/import/bundle", bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		res := &models.ContactBundleResult{}
		if err := doPersonalCRMRequest(client, req, res); err != nil {
			return fmt.Errorf("importing contacts %d-%d: %w", start+1, start+len(part.Contacts), err)
		}
		created += res.Created
		updated += res.Updated
		for importID, id := range res.IDs {
			ids[importID] = id
		}
		fmt.Fprintf(out, "Contacts: %d/%d\n", start+len(part.Contacts), len(bundle.Contacts))
	}

	uploaded, failed := 0, 0
	for _, photo := range photos {
		id, ok := ids[photo.importID]
		if !ok {
			continue
		}
		if err := uploadPersonalCRMPhoto(ctx, client, base, id, photo.path); err != nil {
			failed++
			fmt.Fprintf(out, "Photo of contact %d: %s\n", id, err)
			continue
		}
		uploaded++
		if uploaded%50 == 0 {
			fmt.Fprintf(out, "Photos: %d/%d\n", uploaded, len(photos))
		}
	}
	fmt.Fprintf(out, "Done: %d contacts created, %d updated, %d photos uploaded", created, updated, uploaded)
	if failed > 0 {
		fmt.Fprintf(out, ", %d photos failed", failed)
	}
	fmt.Fprintln(out)
	return nil
}

func doPersonalCRMRequest(client *http.Client, req *http.Request, into any) error {
	req.Header.Set("Authorization", "Bearer "+importPersonalCRMToken)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024*1024))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		text := strings.TrimSpace(string(data))
		if len(text) > 400 {
			text = text[:400]
		}
		return fmt.Errorf("server answered %d: %s", resp.StatusCode, text)
	}
	if into == nil {
		return nil
	}
	return json.Unmarshal(data, into)
}

func uploadPersonalCRMPhoto(ctx context.Context, client *http.Client, base string, contactID int64, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	body := &bytes.Buffer{}
	form := multipart.NewWriter(body)
	part, err := form.CreateFormFile("photo", filepath.Base(path))
	if err != nil {
		return err
	}
	if _, err := part.Write(data); err != nil {
		return err
	}
	if err := form.Close(); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/contacts/%d/photo", base, contactID), body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", form.FormDataContentType())
	return doPersonalCRMRequest(client, req, nil)
}

func personalCRMTime(value sql.NullString) time.Time {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, value.String); err == nil {
			return t
		}
	}
	return time.Time{}
}

// readPersonalCRM loads the desktop app's database into a bundle. Photos are
// returned as paths, keyed by the contact's id in that database.
func readPersonalCRM(ctx context.Context, dir string) (*models.ContactBundle, []personalCRMPhoto, error) {
	dbPath := filepath.Join(dir, "crm.sqlite3")
	if _, err := os.Stat(dbPath); err != nil {
		return nil, nil, fmt.Errorf("no Personal CRM database at %s: %w", dbPath, err)
	}
	// Read-only, so a running desktop app is not disturbed.
	conn, err := sql.Open("sqlite3", "file:"+url.PathEscape(dbPath)+"?mode=ro")
	if err != nil {
		return nil, nil, err
	}
	defer conn.Close()

	bundle := &models.ContactBundle{}
	photos := []personalCRMPhoto{}
	byID := map[string]*models.ContactBundleContact{}

	rows, err := conn.QueryContext(ctx, `SELECT id, first_name, last_name, full_name, headline, company, title, location, email, phone,
		linkedin_url, birthday, profile_image_key, about, source, raw_data, locked_fields, created_at, updated_at
		FROM contacts ORDER BY created_at`)
	if err != nil {
		return nil, nil, fmt.Errorf("reading contacts: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var f [18]sql.NullString
		dest := []any{&id}
		for i := range f {
			dest = append(dest, &f[i])
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, nil, err
		}
		c := &models.ContactBundleContact{
			ImportID:    id,
			FirstName:   f[0].String,
			LastName:    f[1].String,
			FullName:    f[2].String,
			Headline:    f[3].String,
			Company:     f[4].String,
			Title:       f[5].String,
			Location:    f[6].String,
			Email:       f[7].String,
			Phone:       f[8].String,
			LinkedinURL: f[9].String,
			Birthday:    f[10].String,
			About:       f[12].String,
			Source:      f[13].String,
			RawData:     f[14].String,
			Created:     personalCRMTime(f[16]),
			Updated:     personalCRMTime(f[17]),
		}
		_ = json.Unmarshal([]byte(f[15].String), &c.LockedFields)
		if key := f[11].String; key != "" {
			// The app caches photos under their storage key with slashes replaced.
			path := filepath.Join(dir, "images", strings.NewReplacer("/", "__", "\\", "__").Replace(key))
			if _, err := os.Stat(path); err == nil {
				photos = append(photos, personalCRMPhoto{
					importID: id,
					path:     path,
				})
			}
		}
		byID[id] = c
		bundle.Contacts = append(bundle.Contacts, c)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	// each runs a query of (contact id, strings...) rows and hands every row of a known contact to fn.
	each := func(query string, cols int, fn func(c *models.ContactBundleContact, v []sql.NullString)) error {
		rows, err := conn.QueryContext(ctx, query)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			v := make([]sql.NullString, cols)
			dest := []any{&id}
			for i := range v {
				dest = append(dest, &v[i])
			}
			if err := rows.Scan(dest...); err != nil {
				return err
			}
			if c := byID[id]; c != nil {
				fn(c, v)
			}
		}
		return rows.Err()
	}

	err = each(`SELECT contact_id, company, title, location, start_date, end_date, description FROM experiences ORDER BY sort_order, rowid`, 6,
		func(c *models.ContactBundleContact, v []sql.NullString) {
			c.Experiences = append(c.Experiences, &models.ContactExperience{
				Company:     v[0].String,
				Title:       v[1].String,
				Location:    v[2].String,
				StartDate:   v[3].String,
				EndDate:     v[4].String,
				Description: v[5].String,
			})
		})
	if err != nil {
		return nil, nil, fmt.Errorf("reading experiences: %w", err)
	}
	err = each(`SELECT contact_id, school, degree, field_of_study, start_year, end_year FROM educations ORDER BY sort_order, rowid`, 5,
		func(c *models.ContactBundleContact, v []sql.NullString) {
			c.Educations = append(c.Educations, &models.ContactEducation{
				School:       v[0].String,
				Degree:       v[1].String,
				FieldOfStudy: v[2].String,
				StartYear:    v[3].String,
				EndYear:      v[4].String,
			})
		})
	if err != nil {
		return nil, nil, fmt.Errorf("reading educations: %w", err)
	}
	err = each(`SELECT contact_id, body, created_at, updated_at FROM notes ORDER BY created_at`, 3,
		func(c *models.ContactBundleContact, v []sql.NullString) {
			c.Notes = append(c.Notes, &models.ContactBundleNote{
				Body:    v[0].String,
				Created: personalCRMTime(v[1]),
				Updated: personalCRMTime(v[2]),
			})
		})
	if err != nil {
		return nil, nil, fmt.Errorf("reading notes: %w", err)
	}
	err = each(`SELECT ct.contact_id, t.name FROM contact_tags ct JOIN tags t ON t.id = ct.tag_id`, 1,
		func(c *models.ContactBundleContact, v []sql.NullString) {
			c.Tags = append(c.Tags, v[0].String)
		})
	if err != nil {
		return nil, nil, fmt.Errorf("reading tags: %w", err)
	}
	err = each(`SELECT cg.contact_id, g.name FROM contact_groups cg JOIN groups g ON g.id = cg.group_id`, 1,
		func(c *models.ContactBundleContact, v []sql.NullString) {
			c.Groups = append(c.Groups, v[0].String)
		})
	if err != nil {
		return nil, nil, fmt.Errorf("reading groups: %w", err)
	}
	err = each(`SELECT contact_id, method, status, detail, CAST(attempts AS TEXT) FROM linkedin_status`, 4,
		func(c *models.ContactBundleContact, v []sql.NullString) {
			attempts := 0
			_, _ = fmt.Sscan(v[3].String, &attempts)
			c.Statuses = append(c.Statuses, &models.ContactBundleStat{
				Method:   v[0].String,
				Status:   v[1].String,
				Detail:   v[2].String,
				Attempts: attempts,
			})
		})
	if err != nil {
		return nil, nil, fmt.Errorf("reading LinkedIn statuses: %w", err)
	}

	groupRows, err := conn.QueryContext(ctx, `SELECT name, color, description FROM groups`)
	if err != nil {
		return nil, nil, fmt.Errorf("reading groups: %w", err)
	}
	defer groupRows.Close()
	for groupRows.Next() {
		var name string
		var color, description sql.NullString
		if err := groupRows.Scan(&name, &color, &description); err != nil {
			return nil, nil, err
		}
		bundle.Groups = append(bundle.Groups, &models.ContactBundleGroup{
			Name:        name,
			HexColor:    color.String,
			Description: description.String,
		})
	}
	if err := groupRows.Err(); err != nil {
		return nil, nil, err
	}
	tagRows, err := conn.QueryContext(ctx, `SELECT name, color FROM tags`)
	if err != nil {
		return nil, nil, fmt.Errorf("reading tags: %w", err)
	}
	defer tagRows.Close()
	for tagRows.Next() {
		var name string
		var color sql.NullString
		if err := tagRows.Scan(&name, &color); err != nil {
			return nil, nil, err
		}
		bundle.Tags = append(bundle.Tags, &models.ContactBundleTag{
			Name:     name,
			HexColor: color.String,
		})
	}
	if err := tagRows.Err(); err != nil {
		return nil, nil, err
	}
	return bundle, photos, nil
}
