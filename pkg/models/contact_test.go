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

package models

import (
	"testing"

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fixtures (pkg/db/fixtures/contacts.yml and contact_*.yml): contacts #1 Ada
// Lovelace (company locked, in group Pioneers, one job, one note), #2 Grace
// Hopper (tagged friends, one job, Yale) and #3 Jean Dupont (Sciences Po)
// belong to user 1. Contact #4, group #2 and tag #2 belong to user 2.

func TestNormalizeLinkedinURL(t *testing.T) {
	for in, want := range map[string]string{
		"":                                        "",
		"https://www.linkedin.com/in/Ada/":        "https://www.linkedin.com/in/ada",
		"http://linkedin.com/in/ada?trk=abc":      "https://www.linkedin.com/in/ada",
		"de.linkedin.com/in/ada#experience":       "https://www.linkedin.com/in/ada",
		"https://www.linkedin.com/in/jean-dupont": "https://www.linkedin.com/in/jean-dupont",
	} {
		assert.Equal(t, want, NormalizeLinkedinURL(in), in)
	}
}

func TestNormalizeBirthday(t *testing.T) {
	for in, want := range map[string]string{
		"1815-12-10": "1815-12-10",
		"10.12.1815": "1815-12-10",
		"12-10":      "12-10",
		"--12-10":    "12-10",
		"1815-13-10": "",
		"02-30":      "",
		"soon":       "",
		"":           "",
	} {
		assert.Equal(t, want, NormalizeBirthday(in), in)
	}
}

func TestFoldContactText(t *testing.T) {
	assert.Equal(t, "sciencespo", FoldContactText("Sciences Po"))
	assert.Equal(t, "sciencespo", FoldContactText("sciences-po"))
	assert.Equal(t, "etudiantasciencespo", FoldContactText("Étudiant à SciencesPo"))
	assert.Equal(t, "strasse", FoldContactText("Straße"))
}

func TestContact_Create(t *testing.T) {
	u := &user.User{ID: 1}

	t.Run("normal", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		c := &Contact{
			FullName:    "  Alan   Turing ",
			Company:     "Bletchley Park",
			LinkedinURL: "linkedin.com/in/Alan-Turing/?trk=x",
			Birthday:    "23.06.1912",
			UserID:      99,
			Source:      "brightdata",
		}
		can, err := c.CanCreate(s, u)
		require.NoError(t, err)
		assert.True(t, can)
		require.NoError(t, c.Create(s, u))
		require.NoError(t, s.Commit())

		assert.Equal(t, int64(1), c.UserID, "the owner comes from the auth, not the body")
		assert.Equal(t, "Alan Turing", c.FullName)
		assert.Equal(t, "Alan", c.FirstName)
		assert.Equal(t, "Turing", c.LastName)
		assert.Equal(t, "1912-06-23", c.Birthday)
		assert.Equal(t, ContactSourceManual, c.Source)
		assert.Empty(t, c.LockedFields, "fields given on create are not locked")
		db.AssertExists(t, "contacts", map[string]any{
			"full_name":    "Alan Turing",
			"user_id":      1,
			"linkedin_url": "https://www.linkedin.com/in/alan-turing",
		}, false)
	})
	t.Run("duplicate LinkedIn URL", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		c := &Contact{
			FullName:    "Ada again",
			LinkedinURL: "https://linkedin.com/in/ada/",
		}
		err := c.Create(s, u)
		require.Error(t, err)
		assert.True(t, IsErrContactAlreadyExists(err))
		assert.Equal(t, int64(1), err.(ErrContactAlreadyExists).ID)
	})
	t.Run("duplicate email, other case", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		err := (&Contact{FullName: "Ada again", Email: "ADA@example.com"}).Create(s, u)
		assert.True(t, IsErrContactAlreadyExists(err))
	})
	t.Run("duplicate allowed", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		c := &Contact{
			FullName:       "Ada again",
			Email:          "ada@example.com",
			AllowDuplicate: true,
		}
		require.NoError(t, c.Create(s, u))
	})
	t.Run("another user's contact is no duplicate", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		c := &Contact{
			FullName:    "Else",
			LinkedinURL: "https://www.linkedin.com/in/else",
		}
		require.NoError(t, c.Create(s, u))
	})
	t.Run("nothing to identify it by", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		err := (&Contact{Company: "Nowhere"}).Create(s, u)
		assert.True(t, IsErrContactNeedsIdentity(err))
	})
	t.Run("link share", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		can, err := (&Contact{FullName: "X"}).CanCreate(s, &LinkSharing{ID: 1})
		require.NoError(t, err)
		assert.False(t, can)
	})
}

func TestContact_ReadOne(t *testing.T) {
	t.Run("owner", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		c := &Contact{ID: 1}
		can, maxPermission, err := c.CanRead(s, &user.User{ID: 1})
		require.NoError(t, err)
		assert.True(t, can)
		assert.Equal(t, int(PermissionAdmin), maxPermission)
		require.NoError(t, c.ReadOne(s, &user.User{ID: 1}))
		assert.Equal(t, "Ada Lovelace", c.Name)
		require.Len(t, c.Experiences, 1)
		assert.Equal(t, "Mathematician", c.Experiences[0].Title)
		require.Len(t, c.Notes, 1)
		assert.Equal(t, []string{"Pioneers"}, c.Groups)
		assert.Equal(t, []int64{1}, c.GroupIDs)
		assert.Empty(t, c.Tags)
		assert.Equal(t, []string{"company"}, c.LockedFields)
	})
	t.Run("other user", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		can, _, err := (&Contact{ID: 4}).CanRead(s, &user.User{ID: 1})
		require.NoError(t, err)
		assert.False(t, can)
	})
	t.Run("link share", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		can, _, err := (&Contact{ID: 1}).CanRead(s, &LinkSharing{ID: 1})
		require.NoError(t, err)
		assert.False(t, can)
	})
	t.Run("nonexisting", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		_, _, err := (&Contact{ID: 9999}).CanRead(s, &user.User{ID: 1})
		assert.True(t, IsErrContactDoesNotExist(err))
	})
}

func TestContact_ReadAll(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	s := db.NewSession()
	defer s.Close()

	result, _, total, err := (&Contact{}).ReadAll(s, &user.User{ID: 1}, "", 1, 50)
	require.NoError(t, err)
	contacts := result.([]*Contact)
	require.Len(t, contacts, 3)
	assert.EqualValues(t, 3, total)
	assert.Equal(t, "Ada Lovelace", contacts[0].Name, "ordered by name")
	assert.Equal(t, []string{"friends"}, contacts[1].Tags)

	result, _, _, err = (&Contact{}).ReadAll(s, &user.User{ID: 1}, "navy", 1, 50)
	require.NoError(t, err)
	require.Len(t, result.([]*Contact), 1)

	_, _, _, err = (&Contact{}).ReadAll(s, &LinkSharing{ID: 1}, "", 1, 50)
	assert.True(t, IsErrGenericForbidden(err))
}

func TestContact_Update(t *testing.T) {
	u := &user.User{ID: 1}

	t.Run("changed fields get locked", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		c := &Contact{
			ID:          2,
			FullName:    "Grace Brewster Hopper",
			Company:     "US Navy",
			Location:    "New York, United States",
			LinkedinURL: "https://www.linkedin.com/in/grace",
			Title:       "Rear Admiral",
		}
		can, err := c.CanUpdate(s, u)
		require.NoError(t, err)
		assert.True(t, can)
		require.NoError(t, c.Update(s, u))
		require.NoError(t, s.Commit())

		assert.ElementsMatch(t, []string{"full_name", "title"}, c.LockedFields, "unchanged fields stay unlocked")
		assert.Equal(t, "Grace", c.FirstName)
		assert.Equal(t, "Brewster Hopper", c.LastName)
		assert.Equal(t, ContactSourceCSV, c.Source, "the source cannot be changed")
		assert.Equal(t, []string{"friends"}, c.Tags)
		db.AssertExists(t, "contacts", map[string]any{"id": 2, "title": "Rear Admiral"}, false)
	})
	t.Run("an empty string clears and locks", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		c := &Contact{
			ID:          2,
			FullName:    "Grace Hopper",
			Location:    "New York, United States",
			LinkedinURL: "https://www.linkedin.com/in/grace",
		}
		require.NoError(t, c.Update(s, u))
		assert.Empty(t, c.Company)
		assert.Equal(t, []string{"company"}, c.LockedFields)
	})
	t.Run("unlocking", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		c := &Contact{ID: 1}
		require.NoError(t, c.load(s))
		c.LockedFields = []string{"nonsense"}
		require.NoError(t, c.Update(s, u))
		assert.Empty(t, c.LockedFields)
	})
	t.Run("invalid birthday", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		err := (&Contact{ID: 1, FullName: "Ada Lovelace", Birthday: "someday"}).Update(s, u)
		assert.True(t, IsErrContactInvalidBirthday(err))
	})
	t.Run("other user", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		can, err := (&Contact{ID: 4, FullName: "Mine now"}).CanUpdate(s, u)
		require.NoError(t, err)
		assert.False(t, can)
	})
}

func TestContact_Delete(t *testing.T) {
	t.Run("owner", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		c := &Contact{ID: 1}
		can, err := c.CanDelete(s, &user.User{ID: 1})
		require.NoError(t, err)
		assert.True(t, can)
		require.NoError(t, c.Delete(s, &user.User{ID: 1}))
		require.NoError(t, s.Commit())

		db.AssertMissing(t, "contacts", map[string]any{"id": 1})
		db.AssertMissing(t, "contact_experiences", map[string]any{"contact_id": 1})
		db.AssertMissing(t, "contact_notes", map[string]any{"contact_id": 1})
		db.AssertMissing(t, "contact_group_members", map[string]any{"contact_id": 1})
		db.AssertExists(t, "contact_groups", map[string]any{"id": 1}, false)
		db.AssertExists(t, "contact_notes", map[string]any{"contact_id": 4}, false)
	})
	t.Run("other user", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		can, err := (&Contact{ID: 4}).CanDelete(s, &user.User{ID: 1})
		require.NoError(t, err)
		assert.False(t, can)
	})
	t.Run("with the user", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		require.NoError(t, deleteContactsOfUser(s, 1))
		require.NoError(t, s.Commit())
		db.AssertMissing(t, "contacts", map[string]any{"user_id": 1})
		db.AssertMissing(t, "contact_groups", map[string]any{"user_id": 1})
		db.AssertMissing(t, "contact_tags", map[string]any{"user_id": 1})
		db.AssertExists(t, "contacts", map[string]any{"id": 4}, false)
		db.AssertExists(t, "contact_tags", map[string]any{"id": 2}, false)
	})
}

func TestContact_History(t *testing.T) {
	u := &user.User{ID: 1}

	t.Run("a current job goes on top and locks the list", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		c, err := LoadOwnedContact(s, u, 2)
		require.NoError(t, err)
		e := &ContactExperience{
			Company:   " COBOL Inc ",
			Title:     "Consultant",
			StartDate: "1967",
		}
		require.NoError(t, c.AddExperience(s, e))
		require.NoError(t, s.Commit())

		assert.Equal(t, "COBOL Inc", e.Company)
		assert.Equal(t, 0, e.SortOrder)
		db.AssertExists(t, "contact_experiences", map[string]any{"id": 2, "sort_order": 1}, false)
		stored, err := LoadOwnedContact(s, u, 2)
		require.NoError(t, err)
		assert.Contains(t, stored.LockedFields, ContactLockExperiences)
	})
	t.Run("an entry of another contact", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		c, err := LoadOwnedContact(s, u, 1)
		require.NoError(t, err)
		assert.True(t, IsErrContactEntryDoesNotExist(c.DeleteExperience(s, 3)))
		assert.True(t, IsErrContactEntryDoesNotExist(c.UpdateNote(s, &ContactNote{ID: 2, Body: "mine"})))
		assert.True(t, IsErrContactEntryEmpty(c.AddEducation(s, &ContactEducation{Degree: "PhD"})))
	})
	t.Run("another user's contact", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		_, err := LoadOwnedContact(s, u, 4)
		assert.True(t, IsErrContactDoesNotExist(err))
	})
	t.Run("notes", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		c, err := LoadOwnedContact(s, u, 3)
		require.NoError(t, err)
		n := &ContactNote{Body: " Met in Reims "}
		require.NoError(t, c.AddNote(s, n))
		n.Body = "Met in Reims, 2021"
		require.NoError(t, c.UpdateNote(s, n))
		require.NoError(t, s.Commit())
		db.AssertExists(t, "contact_notes", map[string]any{"contact_id": 3, "body": "Met in Reims, 2021"}, false)
	})
}

func TestSearchContacts(t *testing.T) {
	u := &user.User{ID: 1}
	search := func(t *testing.T, q *ContactSearch) *ContactSearchResult {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		res, err := SearchContacts(s, u, q)
		require.NoError(t, err)
		return res
	}

	t.Run("org matches schools and the headline, whatever the spelling", func(t *testing.T) {
		res := search(t, &ContactSearch{Org: []string{"sciences-po"}})
		require.Len(t, res.Contacts, 1)
		assert.Equal(t, int64(3), res.Contacts[0].ID)
		assert.Equal(t, []string{"school: Sciences Po (Master), 2019–2021", "headline: Étudiant à SciencesPo"}, res.Contacts[0].Matched)
	})
	t.Run("all filters must match", func(t *testing.T) {
		res := search(t, &ContactSearch{
			Company:  []string{"navy"},
			Location: []string{"london"},
		})
		assert.Equal(t, 0, res.Total)
	})
	t.Run("any filter may match", func(t *testing.T) {
		res := search(t, &ContactSearch{
			Company:  []string{"navy"},
			Location: []string{"london"},
			Match:    "any",
			IDsOnly:  true,
		})
		assert.ElementsMatch(t, []int64{1, 2}, res.IDs)
	})
	t.Run("past jobs count unless current_only", func(t *testing.T) {
		assert.Equal(t, 1, search(t, &ContactSearch{Company: []string{"remington"}}).Total)
		assert.Equal(t, 0, search(t, &ContactSearch{Company: []string{"remington"}, CurrentOnly: true}).Total)
	})
	t.Run("exclusions, has and missing", func(t *testing.T) {
		res := search(t, &ContactSearch{NotGroup: []string{"pioneers"}, IDsOnly: true})
		assert.ElementsMatch(t, []int64{2, 3}, res.IDs)
		res = search(t, &ContactSearch{Has: []string{"email"}, Missing: []string{"experience"}, IDsOnly: true})
		assert.Equal(t, []int64{3}, res.IDs)
	})
	t.Run("free text spans notes and tags", func(t *testing.T) {
		assert.Equal(t, 1, search(t, &ContactSearch{Text: "royal ada"}).Total)
		assert.Equal(t, 1, search(t, &ContactSearch{Text: "friends"}).Total)
	})
	t.Run("another user's contacts never match", func(t *testing.T) {
		assert.Equal(t, 0, search(t, &ContactSearch{Text: "else"}).Total)
		assert.Equal(t, 0, search(t, &ContactSearch{IDs: []int64{4}}).Total)
	})
	t.Run("invalid", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		_, err := SearchContacts(s, u, &ContactSearch{Has: []string{"shoe_size"}})
		assert.True(t, IsErrContactInvalidSearch(err))
		_, err = SearchContacts(s, &LinkSharing{ID: 1}, &ContactSearch{})
		assert.True(t, IsErrGenericForbidden(err))
	})
}

func TestGetContactDistinctValues(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	s := db.NewSession()
	defer s.Close()

	res, err := GetContactDistinctValues(s, &user.User{ID: 1}, "company", "", 0)
	require.NoError(t, err)
	assert.Equal(t, 3, res.TotalValues, "current and past employers, not other users'")
	res, err = GetContactDistinctValues(s, &user.User{ID: 1}, "school", "SCIENCES PO", 0)
	require.NoError(t, err)
	require.Len(t, res.Values, 1)
	assert.Equal(t, "Sciences Po", res.Values[0].Value)
	assert.Equal(t, 1, res.Values[0].Contacts)
}

func TestChangeContactMembership(t *testing.T) {
	u := &user.User{ID: 1}

	t.Run("dry run", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		res, err := ChangeContactMembership(s, u, &ContactMembershipChange{
			Kind:       ContactMembershipGroup,
			Target:     "pioneers",
			ContactIDs: []int64{1, 2, 2, 4, 9999},
			On:         true,
			DryRun:     true,
		})
		require.NoError(t, err)
		require.NoError(t, s.Commit())
		assert.Equal(t, 1, res.Changed)
		assert.Equal(t, 1, res.Unchanged)
		assert.Equal(t, []int64{4, 9999}, res.UnknownIDs, "another user's contact is unknown")
		assert.Equal(t, "Grace Hopper", res.Contacts[0].Name)
		db.AssertMissing(t, "contact_group_members", map[string]any{"contact_id": 2})
	})
	t.Run("add, by id", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		_, err := ChangeContactMembership(s, u, &ContactMembershipChange{
			Kind:       ContactMembershipGroup,
			Target:     "1",
			ContactIDs: []int64{2},
			On:         true,
		})
		require.NoError(t, err)
		require.NoError(t, s.Commit())
		db.AssertExists(t, "contact_group_members", map[string]any{"contact_id": 2, "group_id": 1}, false)
	})
	t.Run("unknown group", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		_, err := ChangeContactMembership(s, u, &ContactMembershipChange{
			Kind:       ContactMembershipGroup,
			Target:     "Someone else's group",
			ContactIDs: []int64{1},
			On:         true,
		})
		require.Error(t, err)
		assert.True(t, IsErrContactGroupNotFound(err))
		assert.Equal(t, []string{"Pioneers"}, err.(ErrContactGroupNotFound).Groups)
	})
	t.Run("create the group", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		res, err := ChangeContactMembership(s, u, &ContactMembershipChange{
			Kind:       ContactMembershipGroup,
			Target:     "Sciences Po",
			ContactIDs: []int64{3},
			On:         true,
			Create:     true,
		})
		require.NoError(t, err)
		require.NoError(t, s.Commit())
		assert.True(t, res.Target.Created)
		db.AssertExists(t, "contact_groups", map[string]any{"name": "Sciences Po", "user_id": 1}, false)
		db.AssertExists(t, "contact_group_members", map[string]any{"contact_id": 3, "group_id": res.Target.ID}, false)
	})
	t.Run("remove", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		res, err := ChangeContactMembership(s, u, &ContactMembershipChange{
			Kind:       ContactMembershipGroup,
			Target:     "Pioneers",
			ContactIDs: []int64{1, 3},
		})
		require.NoError(t, err)
		require.NoError(t, s.Commit())
		assert.Equal(t, 1, res.Changed)
		assert.Equal(t, 1, res.Unchanged)
		db.AssertMissing(t, "contact_group_members", map[string]any{"contact_id": 1})
	})
	t.Run("tags are created on demand, but not to untag", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		res, err := ChangeContactMembership(s, u, &ContactMembershipChange{
			Kind:       ContactMembershipTag,
			Target:     "theirs",
			ContactIDs: []int64{1},
			On:         true,
		})
		require.NoError(t, err)
		assert.True(t, res.Target.Created, "another user's tag of the same name is not reused")
		assert.NotEqual(t, int64(2), res.Target.ID)

		_, err = ChangeContactMembership(s, u, &ContactMembershipChange{
			Kind:       ContactMembershipTag,
			Target:     "nope",
			ContactIDs: []int64{1},
		})
		assert.True(t, IsErrContactTagDoesNotExist(err))
	})
}

func TestContactGroup(t *testing.T) {
	u := &user.User{ID: 1}

	t.Run("create reuses a group of the same name", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		g := &ContactGroup{
			Name:     "pioneers",
			HexColor: "#ff0000",
		}
		require.NoError(t, g.Create(s, u))
		assert.Equal(t, int64(1), g.ID)
		assert.Equal(t, "Pioneers", g.Name)
		assert.Equal(t, "ff0000", g.HexColor)
		assert.Equal(t, "People who were first", g.Description)
	})
	t.Run("permissions", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		can, _, err := (&ContactGroup{ID: 1}).CanRead(s, u)
		require.NoError(t, err)
		assert.True(t, can)
		can, _, err = (&ContactGroup{ID: 2}).CanRead(s, u)
		require.NoError(t, err)
		assert.False(t, can)
		can, err = (&ContactGroup{ID: 2}).CanUpdate(s, u)
		require.NoError(t, err)
		assert.False(t, can)
		can, err = (&ContactGroup{ID: 2}).CanDelete(s, u)
		require.NoError(t, err)
		assert.False(t, can)
		can, err = (&ContactGroup{ID: 1}).CanDelete(s, &LinkSharing{ID: 1})
		require.NoError(t, err)
		assert.False(t, can)
	})
	t.Run("delete keeps the contacts", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		require.NoError(t, (&ContactGroup{ID: 1}).Delete(s, u))
		require.NoError(t, s.Commit())
		db.AssertMissing(t, "contact_group_members", map[string]any{"group_id": 1})
		db.AssertExists(t, "contacts", map[string]any{"id": 1}, false)
	})
	t.Run("deleting a tag", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		assert.True(t, IsErrContactTagDoesNotExist(DeleteContactTag(s, u, 2)), "another user's tag")
		require.NoError(t, DeleteContactTag(s, u, 1))
		require.NoError(t, s.Commit())
		db.AssertMissing(t, "contact_tag_links", map[string]any{"tag_id": 1})
		db.AssertExists(t, "contact_tags", map[string]any{"id": 2}, false)
	})
}

func TestImportLinkedinConnectionsCSV(t *testing.T) {
	const csv = "Notes:\n\"When exporting your connection data, some emails may be missing.\"\n\n" +
		"First Name,Last Name,URL,Email Address,Company,Position,Connected On\n" +
		"Ada,Lovelace,https://www.linkedin.com/in/ada,,Babbage & Co,Countess,01 Jan 2024\n" +
		"Jean,Dupont,https://www.linkedin.com/in/jean-dupont,JEAN@example.com,\"BCG, Paris\",Consultant,02 Jan 2024\n" +
		"Katherine,Johnson,https://www.linkedin.com/in/katherine,,NASA,Mathematician,03 Jan 2024\n" +
		",,,,,,\n"

	db.LoadAndAssertFixtures(t)
	s := db.NewSession()
	defer s.Close()

	res, err := ImportLinkedinConnectionsCSV(s, &user.User{ID: 1}, csv)
	require.NoError(t, err)
	require.NoError(t, s.Commit())
	assert.Equal(t, &ContactImportResult{Created: 1, Updated: 2}, res)

	// Matched by URL: the unlocked title is refreshed, the locked company is kept.
	db.AssertExists(t, "contacts", map[string]any{"id": 1, "company": "Analytical Engines", "title": "Countess", "source": "csv"}, false)
	// Matched by email, whatever its case: the URL is filled in.
	db.AssertExists(t, "contacts", map[string]any{"id": 3, "company": "BCG, Paris", "linkedin_url": "https://www.linkedin.com/in/jean-dupont"}, false)
	db.AssertExists(t, "contacts", map[string]any{"full_name": "Katherine Johnson", "user_id": 1, "source": "csv"}, false)

	_, err = ImportLinkedinConnectionsCSV(s, &user.User{ID: 1}, "nothing useful")
	assert.True(t, IsErrContactImportEmpty(err))
}

func TestParseLinkedinDateRange(t *testing.T) {
	for _, tc := range []struct {
		line       string
		start, end string
		ok         bool
	}{
		{line: "Jan 2020 - Present · 4 yrs", start: "Jan 2020", end: "Present", ok: true},
		{line: "2018 – 2022", start: "2018", end: "2022", ok: true},
		{line: "2018-2022", start: "2018", end: "2022", ok: true},
		{line: "Sep 2019 - Jun 2021 · 1 yr 10 mos", start: "Sep 2019", end: "Jun 2021", ok: true},
		{line: "janv. 2021 - aujourd'hui", start: "janv. 2021", end: "Present", ok: true},
		{line: "2015", start: "2015", ok: true},
		{line: "Paris, France"},
		{line: "Built the 2020 roadmap for the whole company and shipped it"},
	} {
		start, end, ok := ParseLinkedinDateRange(tc.line)
		assert.Equal(t, tc.ok, ok, tc.line)
		assert.Equal(t, tc.start, start, tc.line)
		assert.Equal(t, tc.end, end, tc.line)
	}
}

func TestLinkedinProfileFromBrowser(t *testing.T) {
	t.Run("text lines, with grouped roles", func(t *testing.T) {
		p, err := LinkedinProfileFromBrowser(map[string]any{
			"linkedin_url": "https://www.linkedin.com/in/Grace/",
			"profile": map[string]any{
				"full_name": "Grace Hopper",
				"headline":  "Rear Admiral",
			},
			"experience_items": []any{
				map[string]any{
					"lines": []any{"US Navy", "Full-time · 20 yrs", "Washington, DC"},
					"sub": []any{
						[]any{"Rear Admiral", "Rear Admiral", "1985 - 1986 · 1 yr"},
						[]any{"Captain", "1973 - 1985 · 12 yrs", "Arlington", "Led the COBOL standardisation.", "Skills: COBOL"},
					},
				},
				map[string]any{
					"lines": []any{"Senior Mathematician", "Remington Rand · Full-time", "1949 - Present · 70 yrs", "Philadelphia, United States"},
				},
			},
			"education_items": []any{
				[]any{"Yale University", "PhD, Mathematics", "1930 - 1934"},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "https://www.linkedin.com/in/grace", p.LinkedinURL)
		assert.True(t, p.Complete)
		require.Len(t, p.Experiences, 3)
		assert.Equal(t, ContactExperience{
			Company:   "US Navy",
			Title:     "Rear Admiral",
			Location:  "Washington, DC",
			StartDate: "1985",
			EndDate:   "1986",
		}, *p.Experiences[0])
		assert.Equal(t, "Arlington", p.Experiences[1].Location)
		assert.Equal(t, "Led the COBOL standardisation.", p.Experiences[1].Description)
		assert.Equal(t, "Remington Rand", p.Experiences[2].Company)
		assert.Equal(t, "Present", p.Experiences[2].EndDate)
		assert.Equal(t, "Remington Rand", p.Company, "the current job fills company and title")
		assert.Equal(t, "Senior Mathematician", p.Title)
		require.Len(t, p.Educations, 1)
		assert.Equal(t, ContactEducation{
			School:       "Yale University",
			Degree:       "PhD",
			FieldOfStudy: "Mathematics",
			StartYear:    "1930",
			EndYear:      "1934",
		}, *p.Educations[0])
	})
	t.Run("not a profile", func(t *testing.T) {
		_, err := LinkedinProfileFromBrowser(map[string]any{"linkedin_url": "https://www.linkedin.com/company/acme"})
		assert.True(t, IsErrContactInvalidProfile(err))
		_, err = LinkedinProfileFromBrowser(map[string]any{})
		assert.True(t, IsErrContactInvalidProfile(err))
	})
}

func TestLinkedinProfileFromBrightData(t *testing.T) {
	t.Run("profile", func(t *testing.T) {
		row := LinkedinProfileFromBrightData(map[string]any{
			"input": map[string]any{"url": "https://www.linkedin.com/in/grace/"},
			"name":  "Grace Hopper",
			"city":  "New York",
			"current_company": map[string]any{
				"name":  "US Navy",
				"title": "Rear Admiral",
			},
			"avatar": "https://media.example.com/grace.jpg",
			"experience": []any{
				map[string]any{
					"title": "US Navy",
					"positions": []any{
						map[string]any{"title": "Rear Admiral", "meta": "1985 - 1986"},
					},
				},
			},
			"education": []any{
				map[string]any{"title": "Yale University", "degree": "PhD, Mathematics", "start_year": "1930"},
			},
		})
		require.NotNil(t, row.Profile)
		p := row.Profile
		assert.Equal(t, "https://www.linkedin.com/in/grace", p.LinkedinURL)
		assert.False(t, p.Complete)
		assert.Equal(t, "Rear Admiral", p.Title)
		require.Len(t, p.Experiences, 1)
		assert.Equal(t, "US Navy", p.Experiences[0].Company)
		assert.Equal(t, "1985", p.Experiences[0].StartDate)
		assert.Equal(t, "Mathematics", p.Educations[0].FieldOfStudy)
		assert.Equal(t, "https://media.example.com/grace.jpg", p.AvatarURL)
	})
	t.Run("errors", func(t *testing.T) {
		row := LinkedinProfileFromBrightData(map[string]any{
			"input":      map[string]any{"url": "https://www.linkedin.com/in/gone"},
			"error_code": "dead_page",
		})
		assert.Nil(t, row.Profile)
		assert.False(t, row.Retry)
		assert.Equal(t, "https://www.linkedin.com/in/gone", row.URL)

		row = LinkedinProfileFromBrightData(map[string]any{
			"input": map[string]any{"url": "https://www.linkedin.com/in/slow"},
			"error": "timeout",
		})
		assert.True(t, row.Retry)
		assert.Equal(t, "timeout", row.Error)
	})
}

func TestApplyLinkedinProfile(t *testing.T) {
	thin := func() *LinkedinProfile {
		return &LinkedinProfile{
			LinkedinURL: "https://www.linkedin.com/in/ada",
			FullName:    "Augusta Ada King",
			Company:     "Babbage & Co",
			Headline:    "Enchantress of Number",
			AvatarURL:   "https://media.example.com/ada.jpg",
			Experiences: []*ContactExperience{
				{Company: "Babbage & Co"},
			},
			Educations: []*ContactEducation{
				{School: "Home schooled"},
			},
			Raw: map[string]any{"id": "ada"},
		}
	}

	t.Run("a thin scrape fills, but replaces no richer list or locked field", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		out, err := ApplyLinkedinProfile(s, 1, thin(), ContactSourceBrightData, false)
		require.NoError(t, err)
		require.NoError(t, s.Commit())
		require.NotNil(t, out)
		assert.Equal(t, int64(1), out.ContactID)
		assert.False(t, out.Created)
		assert.False(t, out.ExperiencesWritten, "the stored job has a title and a start date")
		assert.True(t, out.EducationsWritten, "there was no education")
		assert.Equal(t, "https://media.example.com/ada.jpg", out.WantsPhoto)
		db.AssertExists(t, "contacts", map[string]any{
			"id":        1,
			"full_name": "Augusta Ada King",
			"company":   "Analytical Engines",
			"headline":  "Enchantress of Number",
			"source":    "brightdata",
			"email":     "ada@example.com",
		}, false)
		db.AssertExists(t, "contact_experiences", map[string]any{"id": 1, "title": "Mathematician"}, false)
		db.AssertExists(t, "contact_educations", map[string]any{"contact_id": 1, "school": "Home schooled"}, false)
	})
	t.Run("a complete capture replaces lists, unless hand-edited", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		p := thin()
		p.Complete = true
		out, err := ApplyLinkedinProfile(s, 1, p, ContactSourceLinkedin, false)
		require.NoError(t, err)
		assert.True(t, out.ExperiencesWritten)
		assert.Equal(t, 1, out.Experiences)

		c, err := LoadOwnedContact(s, &user.User{ID: 2}, 4)
		require.NoError(t, err)
		require.NoError(t, c.AddExperience(s, &ContactExperience{Title: "By hand"}))
		p = thin()
		p.Complete = true
		p.LinkedinURL = "https://www.linkedin.com/in/else"
		out, err = ApplyLinkedinProfile(s, 2, p, ContactSourceLinkedin, false)
		require.NoError(t, err)
		assert.False(t, out.ExperiencesWritten)
		assert.Equal(t, 2, out.Experiences)
	})
	t.Run("unknown people are only created on request", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		p := thin()
		// User 1 has nobody with this URL; user 2's contact must not be touched.
		p.LinkedinURL = "https://www.linkedin.com/in/else"
		out, err := ApplyLinkedinProfile(s, 1, p, ContactSourceBrightData, false)
		require.NoError(t, err)
		assert.Nil(t, out)

		out, err = ApplyLinkedinProfile(s, 1, p, ContactSourceLinkedin, true)
		require.NoError(t, err)
		require.NoError(t, s.Commit())
		assert.True(t, out.Created)
		assert.NotEqual(t, int64(4), out.ContactID)
		db.AssertExists(t, "contacts", map[string]any{"id": 4, "full_name": "Someone Else"}, false)
		db.AssertExists(t, "contacts", map[string]any{"id": out.ContactID, "user_id": 1, "first_name": "Augusta"}, false)
	})
}

func TestLinkedinBrowserQueue(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	s := db.NewSession()
	defer s.Close()
	u := &user.User{ID: 1}

	// Ada lacks education and Grace is complete; Jean has no LinkedIn URL.
	coverage, err := GetLinkedinCoverage(s, u)
	require.NoError(t, err)
	assert.Equal(t, &LinkedinCoverage{
		Total:          3,
		WithURL:        2,
		WithExperience: 2,
		WithEducation:  2,
		MissingAny:     1,
		BrowserQueue:   1,
	}, coverage)
	queue, err := GetLinkedinBrowserQueue(s, u, 50)
	require.NoError(t, err)
	require.Len(t, queue, 1)
	assert.Equal(t, int64(1), queue[0].ContactID)

	// Failures are retried up to three times; "unavailable" is final.
	for range 2 {
		require.NoError(t, RecordLinkedinBrowserFailure(s, u, "linkedin.com/in/ada/", false, "timeout"))
	}
	queue, err = GetLinkedinBrowserQueue(s, u, 50)
	require.NoError(t, err)
	assert.Len(t, queue, 1)
	require.NoError(t, RecordLinkedinBrowserFailure(s, u, "linkedin.com/in/ada/", false, "timeout"))
	queue, err = GetLinkedinBrowserQueue(s, u, 50)
	require.NoError(t, err)
	assert.Empty(t, queue)

	urls, err := GetBrightDataTargets(s, 1, true)
	require.NoError(t, err)
	assert.Equal(t, []string{"https://www.linkedin.com/in/ada"}, urls)
	urls, err = GetBrightDataTargets(s, 1, false)
	require.NoError(t, err)
	assert.Len(t, urls, 2)

	assert.True(t, IsErrContactNotInContacts(RecordLinkedinBrowserFailure(s, u, "linkedin.com/in/else", true, "")), "another user's contact")
}

func TestImportContactBundle(t *testing.T) {
	bundle := func() *ContactBundle {
		return &ContactBundle{
			Groups: []*ContactBundleGroup{
				{Name: "Navy", HexColor: "#112233", Description: "Sailors"},
			},
			Contacts: []*ContactBundleContact{
				{
					ImportID:     "3f2b",
					FullName:     "Grace Hopper",
					LinkedinURL:  "https://www.linkedin.com/in/grace/",
					Source:       "scraper",
					LockedFields: []string{"fullName", "experiences", "bogus"},
					Experiences: []*ContactExperience{
						{Company: "US Navy", Title: "Rear Admiral"},
					},
					Notes: []*ContactBundleNote{
						{Body: "Coined the term bug"},
					},
					Tags:   []string{"Friends", "legends"},
					Groups: []string{"Navy", "pioneers"},
					Statuses: []*ContactBundleStat{
						{Method: "browser", Status: "ok"},
					},
				},
			},
		}
	}

	db.LoadAndAssertFixtures(t)
	s := db.NewSession()
	defer s.Close()
	u := &user.User{ID: 1}

	res, err := ImportContactBundle(s, u, bundle())
	require.NoError(t, err)
	require.NoError(t, s.Commit())
	assert.Equal(t, 1, res.Created)
	id := res.IDs["3f2b"]
	require.NotZero(t, id)
	assert.NotEqual(t, int64(2), id, "bundles add contacts as they are; they are not merged by URL")

	c, err := LoadOwnedContact(s, u, id)
	require.NoError(t, err)
	require.NoError(t, c.ReadOne(s, u))
	assert.Equal(t, "Grace", c.FirstName)
	assert.Equal(t, ContactSourceLinkedin, c.Source)
	assert.Equal(t, []string{"full_name", "experiences"}, c.LockedFields)
	assert.Equal(t, []string{"friends", "legends"}, c.Tags, "existing tags are reused whatever their case")
	assert.Equal(t, []string{"Navy", "Pioneers"}, c.Groups)
	assert.Len(t, c.Experiences, 1)
	assert.Len(t, c.Notes, 1)
	db.AssertExists(t, "contact_groups", map[string]any{"name": "Navy", "hex_color": "112233", "description": "Sailors"}, false)
	db.AssertExists(t, "contact_linkedin_status", map[string]any{"contact_id": id, "method": "browser", "status": "ok"}, false)

	// Importing the same id again replaces the contact instead of adding one.
	again := bundle()
	again.Contacts[0].Notes = nil
	again.Contacts[0].Company = "US Navy"
	res, err = ImportContactBundle(s, u, again)
	require.NoError(t, err)
	require.NoError(t, s.Commit())
	assert.Equal(t, 0, res.Created)
	assert.Equal(t, 1, res.Updated)
	assert.Equal(t, id, res.IDs["3f2b"])
	db.AssertExists(t, "contacts", map[string]any{"id": id, "company": "US Navy"}, false)
	db.AssertMissing(t, "contact_notes", map[string]any{"contact_id": id})

	_, err = ImportContactBundle(s, u, &ContactBundle{})
	assert.True(t, IsErrContactImportEmpty(err))
}
