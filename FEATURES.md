# OpenTick feature status

What OpenTick does today compared with TickTick, checked against the code in `web/`, `web-desktop/` and `pkg/`. A control that is shown in the UI but doesn't change anything yet counts as **not implemented**.

Legend: ✅ implemented · ❌ not implemented

## Tasks

| Feature | Status |
| --- | --- |
| Create, edit, complete and delete tasks | ✅ |
| Quick add with smart parsing (dates, times, `!priority`, `#tags`, `~list`) and suggestions, in English and French | ✅ |
| Due dates, all-day and timed tasks, time ranges | ✅ |
| Priorities (high / medium / low / none) | ✅ |
| Tags (add, create, color) | ✅ |
| Markdown descriptions with slash commands | ✅ |
| Subtasks / checklist items | ✅ |
| Reminders | ✅ |
| Recurring tasks (RRULE, weekdays, after completion, end conditions) | ✅ |
| Assignees | ✅ |
| Comments | ✅ |
| Attachments | ✅ |
| "Won't do" status | ✅ |
| Duplicate a task | ✅ |
| Right-click context menu (date, priority, move, duplicate, delete) | ✅ |
| Drag tasks between lists, manual ordering | ✅ |
| Sort by date, priority, title, created or custom order; show/hide completed | ✅ |
| Trash with restore | ✅ |

## Lists and views

| Feature | Status |
| --- | --- |
| Smart lists: Today, Next 7 days, Inbox, Completed, Tags | ✅ |
| Custom lists (create, edit, color, archive, delete) | ✅ |
| Folders / nested lists | ✅ |
| Saved filters (custom smart lists) | ✅ |
| List, kanban and timeline views per list | ✅ |
| Search | ✅ |
| Calendar: day / week / month, drag to create, continuous month scrolling | ✅ |
| Subscribed external calendars (ICS) | ✅ |
| Eisenhower matrix | ✅ |
| Resizable sidebar and detail pane | ✅ |
| Phone layout for the web app | ✅ |
| Smart list visibility settings (panel is UI only) | ❌ |

## Productivity

| Feature | Status |
| --- | --- |
| Pomodoro / focus timer with session history | ✅ |
| Habit tracker | ✅ |
| Countdowns, anniversaries and birthdays | ✅ |
| Statistics and achievement score | ✅ |

## Collaboration

| Feature | Status |
| --- | --- |
| Share lists with users and teams | ✅ |
| Collaborate settings panel (UI only) | ❌ |

## Account, sync and settings

| Feature | Status |
| --- | --- |
| Username / password login, two-factor auth on login | ✅ |
| Clerk sign in (hosted) | ✅ |
| Account settings, avatar upload with crop | ✅ |
| Appearance: theme, colors, background image and blur | ✅ |
| Date & time settings (week start, 12/24h, date format, week numbers) | ✅ |
| Interface in English and French (follows the system language) | ✅ |
| Background sync every minute and on window focus | ✅ |
| CSV backup export and import | ✅ |
| MCP server settings and API tokens for AI clients | ✅ |
| Basic keyboard shortcuts (`n` new task, `/` search, `g` + key to jump between views) | ✅ |
| Focus timer desktop notifications | ✅ |
| Language picker (setting is UI only) | ❌ |
| Full TickTick shortcut set / customizable shortcuts (panel is UI only) | ❌ |
| Features, Notifications and More settings panels (UI only) | ❌ |
| Task reminder notifications in the app (the server only emails them) | ❌ |
| Import from TickTick, Todoist and others in the Integrations panel (UI only) | ❌ |
| CalDAV and calendar integrations in the Integrations panel (UI only) | ❌ |

## Platforms

| Feature | Status |
| --- | --- |
| Web app | ✅ |
| Docker image | ✅ |
| Hosted deployment (Heroku, Neon, S3) | ✅ |
| Linux desktop app (.deb, AppImage; x64 and arm64) with auto update | ✅ |
| macOS desktop app (.dmg; Intel and Apple Silicon, unsigned) | ✅ |
| Windows desktop app (.exe installer, unsigned) | ✅ |
| Mobile apps | ❌ |
| Browser extension | ❌ |

## Totals

**51 of 61 features implemented (84%).** Recount and update the badge in the README when this file changes.
