// node test-extract.js — checks the Voyager parser on a representative
// payload, the blocked-page checks, and the pacing and payload helpers in core.js.
const assert = require("assert");
const path = require("path");
const { parseVoyager, vanityFromUrl, blockReason, errorKind, HttpError } = require(path.join(__dirname, "extract.js"));
const C = require(path.join(__dirname, "core.js"));

const P = "com.linkedin.voyager.dash.identity.profile.";
const payload = {
  data: { "*elements": ["urn:li:fsd_profile:1"] },
  included: [
    { $type: P + "Profile", entityUrn: "urn:li:fsd_profile:1", publicIdentifier: "jane-doe", firstName: "Jane", lastName: "Doe",
      headline: "CTO at Acme", geoLocationName: "Berlin", summary: "About me",
      profilePicture: { displayImageReference: { vectorImage: { rootUrl: "https://media.licdn.com/a/", artifacts: [
        { width: 100, fileIdentifyingUrlPathSegment: "100.jpg" }, { width: 800, fileIdentifyingUrlPathSegment: "800.jpg" }] } } } },
    { $type: P + "Profile", entityUrn: "urn:li:fsd_profile:2", publicIdentifier: "someone-else", firstName: "X" },
    { $type: P + "PositionGroup", entityUrn: "urn:li:fsd_profilePositionGroup:1", companyName: "Acme" },
    { $type: P + "Position", entityUrn: "urn:li:fsd_profilePosition:1", title: "Engineer", companyName: "Acme",
      dateRange: { start: { month: 3, year: 2019 }, end: { month: 12, year: 2021 } } },
    { $type: P + "Position", entityUrn: "urn:li:fsd_profilePosition:2", title: "CTO", companyName: "Acme", locationName: "Berlin",
      dateRange: { start: { month: 1, year: 2022 } }, description: "Leading" },
    { $type: P + "Position", entityUrn: "urn:li:fsd_profilePosition:2", title: "CTO", companyName: "Acme" },
    { $type: P + "Education", entityUrn: "urn:li:fsd_profileEducation:1", schoolName: "TU Berlin", degreeName: "BSc",
      fieldOfStudy: "CS", dateRange: { start: { year: 2015 }, end: { year: 2018 } } },
  ],
};

const r = parseVoyager(payload, "Jane-Doe");
assert.strictEqual(r.profile.first_name, "Jane");
assert.strictEqual(r.profile.avatar_url, "https://media.licdn.com/a/800.jpg");
assert.strictEqual(r.experiences.length, 2);
assert.deepStrictEqual(r.experiences[0], { title: "CTO", company: "Acme", location: "Berlin", start_date: "Jan 2022", end_date: "Present", description: "Leading" });
assert.strictEqual(r.experiences[1].end_date, "Dec 2021");
assert.deepStrictEqual(r.educations[0], { school: "TU Berlin", degree: "BSc", field_of_study: "CS", start_year: "2015", end_year: "2018" });
assert.strictEqual(parseVoyager({ included: [] }, "x"), null);

// Legacy profileView shape.
const legacy = { included: [
  { $type: "com.linkedin.voyager.identity.profile.Profile", firstName: "A", lastName: "B", publicIdentifier: "ab", locationName: "Paris" },
  { $type: "com.linkedin.voyager.identity.profile.Position", title: "Analyst", companyName: "Bank", timePeriod: { startDate: { year: 2020, month: 6 } } },
  { $type: "com.linkedin.voyager.identity.profile.Education", schoolName: "Sciences Po", timePeriod: { startDate: { year: 2016 }, endDate: { year: 2019 } } },
] };
const l = parseVoyager(legacy, "ab");
assert.strictEqual(l.experiences[0].start_date, "Jun 2020");
assert.strictEqual(l.educations[0].end_year, "2019");

assert.strictEqual(vanityFromUrl("https://www.linkedin.com/in/jane-doe/details/experience/"), "jane-doe");
assert.strictEqual(vanityFromUrl("https://www.linkedin.com/company/acme/"), null);

// Pages and errors that must stop a batch.
assert.strictEqual(blockReason("https://www.linkedin.com/checkpoint/challenge/AgFabc"), "checkpoint");
assert.strictEqual(blockReason("https://www.linkedin.com/authwall?trk=x"), "logged_out");
assert.strictEqual(blockReason("https://www.linkedin.com/login?session_redirect=x"), "logged_out");
assert.strictEqual(blockReason("https://www.linkedin.com/uas/login"), "logged_out");
assert.strictEqual(blockReason("https://www.linkedin.com/in/jane-doe/"), null);
assert.strictEqual(blockReason("https://www.linkedin.com/feed/"), null);
assert.strictEqual(blockReason("https://example.com/checkpoint/"), null);
assert.strictEqual(blockReason("not a url"), null);
assert.strictEqual(errorKind(new HttpError(429)), "rate_limited");
assert.strictEqual(errorKind(new HttpError(999)), "rate_limited");
assert.strictEqual(errorKind(new HttpError(401)), "logged_out");
assert.strictEqual(errorKind(new HttpError(403, "x", "checkpoint")), "checkpoint");
assert.strictEqual(errorKind(new HttpError(404)), "not_found");
assert.strictEqual(errorKind(new HttpError(500)), null);

// core.js: settings, server URL, pacing.
const s = C.normalizeSettings({ minDelay: 5, maxDelay: 2, dailyCap: "0", activeFrom: "25:00", serverUrl: "tick.example.com/api/v2/" });
assert.strictEqual(s.minDelay, 30);
assert.strictEqual(s.maxDelay, 30);
assert.strictEqual(s.dailyCap, 1);
assert.strictEqual(s.activeFrom, "08:00");
assert.strictEqual(s.serverUrl, "https://tick.example.com");
assert.strictEqual(C.normalizeSettings({}).dailyCap, 80);
assert.strictEqual(C.normalizeServerUrl("http://localhost:3456/"), "http://localhost:3456");
assert.strictEqual(C.normalizeServerUrl("https://example.com/tick/api/v2"), "https://example.com/tick");
assert.strictEqual(C.normalizeServerUrl("ftp://example.com"), null);
assert.strictEqual(C.originPattern("http://localhost:3456/x"), "http://localhost:3456/*");
assert.strictEqual(C.apiUrl("https://t.example.com/", "/contacts/linkedin/queue"), "https://t.example.com/api/v2/contacts/linkedin/queue");
assert.strictEqual(C.contactLink("https://t.example.com", 7), "https://t.example.com/people/all/7");

const at = (h, m) => new Date(2026, 10, 21, h, m);
const day = { activeHoursEnabled: true, activeFrom: "09:00", activeTo: "18:00" };
assert.ok(C.inActiveHours(day, at(9, 0)));
assert.ok(!C.inActiveHours(day, at(18, 0)));
assert.ok(!C.inActiveHours(day, at(7, 59)));
assert.ok(C.inActiveHours({ activeHoursEnabled: false }, at(3, 0)));
const night = { activeHoursEnabled: true, activeFrom: "22:00", activeTo: "06:00" };
assert.ok(C.inActiveHours(night, at(23, 30)));
assert.ok(C.inActiveHours(night, at(5, 0)));
assert.ok(!C.inActiveHours(night, at(12, 0)));
assert.deepStrictEqual(C.nextActiveStart(day, at(20, 0)), new Date(2026, 10, 22, 9, 0));
assert.deepStrictEqual(C.nextActiveStart(day, at(7, 0)), new Date(2026, 10, 21, 9, 0));
assert.deepStrictEqual(C.nextActiveStart(day, at(10, 0)), at(10, 0));
assert.deepStrictEqual(C.nextDayStart({}, at(23, 0)), new Date(2026, 10, 22, 0, 0, 30));
assert.deepStrictEqual(C.nextDayStart(day, at(10, 0)), new Date(2026, 10, 22, 9, 0));
assert.deepStrictEqual(C.nextTimeOfDay("09:00", at(9, 0)), new Date(2026, 10, 22, 9, 0));
assert.strictEqual(C.randomDelayMs({}, 0), 45000);
assert.strictEqual(C.randomDelayMs({}, 1), 90000);
assert.strictEqual(C.backoffMs(1), 30000);
assert.strictEqual(C.backoffMs(3), 120000);
assert.strictEqual(C.backoffMs(20), 3600000);
assert.strictEqual(C.dayKey(at(0, 5)), "2026-11-21");

// Payloads match what POST /api/v2/contacts/linkedin/profiles accepts.
const body = C.profilePayload("https://www.linkedin.com/in/jane-doe", { ...r, method: "voyager-dash" }, { version: "2.0.0", create: false });
assert.strictEqual(body.complete, true);
assert.strictEqual(body.create, false);
assert.strictEqual(body.experiences.length, 2);
assert.strictEqual(C.profileName(body), "Jane Doe");
const dom = C.domPayload("https://www.linkedin.com/in/jane-doe", [{ lines: ["CTO", "Acme"], sub: [] }], [], { create: true });
assert.strictEqual(dom.complete, false);
assert.strictEqual(dom.method, "dom");
assert.strictEqual(C.profileName(dom), "jane-doe");
assert.strictEqual(
  C.describeOutcome({ created: true, experiences: 2, educations: 1, experiences_written: true, photo_saved: true }),
  "Added to OpenTick: 2 jobs, 1 school (updated), photo added",
);

assert.strictEqual(C.nameFromTitle("(3) Jane Doe | LinkedIn"), "Jane Doe");
assert.strictEqual(C.nameFromTitle("Experience | Jane Doe | LinkedIn"), "Jane Doe");
assert.strictEqual(C.nameFromTitle("LinkedIn"), "");

console.log("extract.js and core.js: all tests passed");
