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

package migration

import (
	"time"

	"src.techknowlogick.com/xormigrate"
	"xorm.io/xorm"
)

type contacts20261114120000 struct {
	ID           int64     `xorm:"bigint autoincr not null unique pk"`
	UserID       int64     `xorm:"bigint not null index"`
	FirstName    string    `xorm:"varchar(250) null"`
	LastName     string    `xorm:"varchar(250) null"`
	FullName     string    `xorm:"varchar(250) null"`
	Headline     string    `xorm:"varchar(500) null"`
	Company      string    `xorm:"varchar(250) null"`
	Title        string    `xorm:"varchar(250) null"`
	Location     string    `xorm:"varchar(250) null"`
	Email        string    `xorm:"varchar(250) null index"`
	Phone        string    `xorm:"varchar(100) null"`
	LinkedinURL  string    `xorm:"varchar(250) null index"`
	Birthday     string    `xorm:"varchar(10) null"`
	About        string    `xorm:"longtext null"`
	Source       string    `xorm:"varchar(30) not null default 'manual'"`
	RawData      string    `xorm:"longtext null"`
	LockedFields []string  `xorm:"json null"`
	ImageFileID  int64     `xorm:"bigint null"`
	ImportID     string    `xorm:"varchar(64) null index"`
	Created      time.Time `xorm:"created not null"`
	Updated      time.Time `xorm:"updated not null"`
}

func (contacts20261114120000) TableName() string {
	return "contacts"
}

type contactExperiences20261114120000 struct {
	ID          int64  `xorm:"bigint autoincr not null unique pk"`
	ContactID   int64  `xorm:"bigint not null index"`
	Company     string `xorm:"varchar(250) null"`
	Title       string `xorm:"varchar(250) null"`
	Location    string `xorm:"varchar(250) null"`
	StartDate   string `xorm:"varchar(50) null"`
	EndDate     string `xorm:"varchar(50) null"`
	Description string `xorm:"longtext null"`
	SortOrder   int    `xorm:"int not null default 0"`
}

func (contactExperiences20261114120000) TableName() string {
	return "contact_experiences"
}

type contactEducations20261114120000 struct {
	ID           int64  `xorm:"bigint autoincr not null unique pk"`
	ContactID    int64  `xorm:"bigint not null index"`
	School       string `xorm:"varchar(250) null"`
	Degree       string `xorm:"varchar(250) null"`
	FieldOfStudy string `xorm:"varchar(250) null"`
	StartYear    string `xorm:"varchar(20) null"`
	EndYear      string `xorm:"varchar(20) null"`
	SortOrder    int    `xorm:"int not null default 0"`
}

func (contactEducations20261114120000) TableName() string {
	return "contact_educations"
}

type contactNotes20261114120000 struct {
	ID        int64     `xorm:"bigint autoincr not null unique pk"`
	ContactID int64     `xorm:"bigint not null index"`
	Body      string    `xorm:"longtext not null"`
	Created   time.Time `xorm:"created not null"`
	Updated   time.Time `xorm:"updated not null"`
}

func (contactNotes20261114120000) TableName() string {
	return "contact_notes"
}

type contactGroups20261114120000 struct {
	ID          int64     `xorm:"bigint autoincr not null unique pk"`
	UserID      int64     `xorm:"bigint not null index"`
	Name        string    `xorm:"varchar(250) not null"`
	HexColor    string    `xorm:"varchar(6) null"`
	Description string    `xorm:"longtext null"`
	Created     time.Time `xorm:"created not null"`
	Updated     time.Time `xorm:"updated not null"`
}

func (contactGroups20261114120000) TableName() string {
	return "contact_groups"
}

type contactGroupMembers20261114120000 struct {
	ID        int64 `xorm:"bigint autoincr not null unique pk"`
	ContactID int64 `xorm:"bigint not null unique(contact_group)"`
	GroupID   int64 `xorm:"bigint not null unique(contact_group) index"`
}

func (contactGroupMembers20261114120000) TableName() string {
	return "contact_group_members"
}

type contactTags20261114120000 struct {
	ID       int64  `xorm:"bigint autoincr not null unique pk"`
	UserID   int64  `xorm:"bigint not null index"`
	Name     string `xorm:"varchar(250) not null"`
	HexColor string `xorm:"varchar(6) null"`
}

func (contactTags20261114120000) TableName() string {
	return "contact_tags"
}

type contactTagLinks20261114120000 struct {
	ID        int64 `xorm:"bigint autoincr not null unique pk"`
	ContactID int64 `xorm:"bigint not null unique(contact_tag)"`
	TagID     int64 `xorm:"bigint not null unique(contact_tag) index"`
}

func (contactTagLinks20261114120000) TableName() string {
	return "contact_tag_links"
}

type contactLinkedinStatus20261114120000 struct {
	ID        int64     `xorm:"bigint autoincr not null unique pk"`
	ContactID int64     `xorm:"bigint not null unique(contact_method)"`
	Method    string    `xorm:"varchar(20) not null unique(contact_method)"`
	Status    string    `xorm:"varchar(20) not null"`
	Detail    string    `xorm:"text null"`
	Attempts  int       `xorm:"int not null default 0"`
	Updated   time.Time `xorm:"updated not null"`
}

func (contactLinkedinStatus20261114120000) TableName() string {
	return "contact_linkedin_status"
}

func contactTables20261114120000() []any {
	return []any{
		contacts20261114120000{},
		contactExperiences20261114120000{},
		contactEducations20261114120000{},
		contactNotes20261114120000{},
		contactGroups20261114120000{},
		contactGroupMembers20261114120000{},
		contactTags20261114120000{},
		contactTagLinks20261114120000{},
		contactLinkedinStatus20261114120000{},
	}
}

func addContacts20261114120000(tx *xorm.Engine) error {
	return tx.Sync(contactTables20261114120000()...) //nolint:forbidigo // brand-new tables, nothing to drop
}

func dropContacts20261114120000(tx *xorm.Engine) error {
	return tx.DropTables(contactTables20261114120000()...)
}

func init() {
	migrations = append(migrations, &xormigrate.Migration{
		ID:          "20261114120000",
		Description: "Add contacts (personal CRM)",
		Migrate:     addContacts20261114120000,
		Rollback:    dropContacts20261114120000,
	})
}
