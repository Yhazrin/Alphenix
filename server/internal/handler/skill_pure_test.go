package handler

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
)

// ---------------------------------------------------------------------------
// validateFilePath
// ---------------------------------------------------------------------------

func TestValidateFilePath_RelativeClean(t *testing.T) {
	if !validateFilePath("path/to/file.txt") {
		t.Error("expected true for clean relative path")
	}
}

func TestValidateFilePath_Simple(t *testing.T) {
	if !validateFilePath("file.txt") {
		t.Error("expected true for simple filename")
	}
}

func TestValidateFilePath_Empty(t *testing.T) {
	if validateFilePath("") {
		t.Error("expected false for empty path")
	}
}

func TestValidateFilePath_AbsoluteUnix(t *testing.T) {
	if validateFilePath("/etc/passwd") {
		t.Error("expected false for absolute Unix path")
	}
}

func TestValidateFilePath_AbsoluteWindows(t *testing.T) {
	if validateFilePath(`C:\Windows\system32`) {
		t.Error("expected false for absolute Windows path")
	}
}

func TestValidateFilePath_DotDotTraversal(t *testing.T) {
	if validateFilePath("../../etc/passwd") {
		t.Error("expected false for .. traversal")
	}
}

func TestValidateFilePath_DotDotMid(t *testing.T) {
	if validateFilePath("a/../../b") {
		t.Error("expected false for .. in middle of path")
	}
}

func TestValidateFilePath_DotSlash(t *testing.T) {
	if !validateFilePath("./foo") {
		t.Error("expected true for ./foo")
	}
}

func TestValidateFilePath_JustDotDot(t *testing.T) {
	if validateFilePath("..") {
		t.Error("expected false for bare ..")
	}
}

// ---------------------------------------------------------------------------
// parseSkillFrontmatter
// ---------------------------------------------------------------------------

func TestParseSkillFrontmatter_Valid(t *testing.T) {
	content := "---\nname: my-skill\ndescription: Does stuff\n---\n# Content"
	name, desc := parseSkillFrontmatter(content)
	if name != "my-skill" {
		t.Errorf("name = %q, want my-skill", name)
	}
	if desc != "Does stuff" {
		t.Errorf("desc = %q, want Does stuff", desc)
	}
}

func TestParseSkillFrontmatter_Quoted(t *testing.T) {
	content := "---\nname: \"My Skill\"\n---\nbody"
	name, _ := parseSkillFrontmatter(content)
	if name != "My Skill" {
		t.Errorf("name = %q, want My Skill", name)
	}
}

func TestParseSkillFrontmatter_SingleQuoted(t *testing.T) {
	content := "---\nname: 'My Skill'\n---\nbody"
	name, _ := parseSkillFrontmatter(content)
	if name != "My Skill" {
		t.Errorf("name = %q, want My Skill", name)
	}
}

func TestParseSkillFrontmatter_NoFrontmatter(t *testing.T) {
	name, desc := parseSkillFrontmatter("# Just markdown")
	if name != "" || desc != "" {
		t.Errorf("expected empty for no frontmatter, got (%q, %q)", name, desc)
	}
}

func TestParseSkillFrontmatter_Unclosed(t *testing.T) {
	name, desc := parseSkillFrontmatter("---\nname: foo")
	if name != "" || desc != "" {
		t.Errorf("expected empty for unclosed frontmatter, got (%q, %q)", name, desc)
	}
}

func TestParseSkillFrontmatter_NameOnly(t *testing.T) {
	content := "---\nname: just-name\n---\n"
	name, desc := parseSkillFrontmatter(content)
	if name != "just-name" {
		t.Errorf("name = %q", name)
	}
	if desc != "" {
		t.Errorf("desc should be empty, got %q", desc)
	}
}

func TestParseSkillFrontmatter_Empty(t *testing.T) {
	name, desc := parseSkillFrontmatter("")
	if name != "" || desc != "" {
		t.Errorf("expected empty, got (%q, %q)", name, desc)
	}
}

// ---------------------------------------------------------------------------
// detectImportSource
// ---------------------------------------------------------------------------

func TestDetectImportSource_Empty(t *testing.T) {
	_, _, err := detectImportSource("")
	if err == nil {
		t.Error("empty URL should return error")
	}
}

func TestDetectImportSource_WhitespaceOnly(t *testing.T) {
	_, _, err := detectImportSource("   ")
	if err == nil {
		t.Error("whitespace-only should return error")
	}
}

func TestDetectImportSource_ClawHubURL(t *testing.T) {
	tests := []string{
		"https://clawhub.ai/owner/skill-name",
		"http://clawhub.ai/owner/skill-name",
		"https://www.clawhub.ai/owner/skill-name",
		"clawhub.ai/owner/skill-name",
	}
	for _, raw := range tests {
		src, normalized, err := detectImportSource(raw)
		if err != nil {
			t.Errorf("detectImportSource(%q) unexpected error: %v", raw, err)
			continue
		}
		if src != sourceClawHub {
			t.Errorf("detectImportSource(%q) source = %d, want sourceClawHub", raw, src)
		}
		if normalized == "" {
			t.Errorf("detectImportSource(%q) returned empty normalized URL", raw)
		}
	}
}

func TestDetectImportSource_SkillsShURL(t *testing.T) {
	tests := []string{
		"https://skills.sh/owner/repo/skill-name",
		"http://skills.sh/owner/repo/skill-name",
		"https://www.skills.sh/owner/repo/skill-name",
		"skills.sh/owner/repo/skill-name",
	}
	for _, raw := range tests {
		src, normalized, err := detectImportSource(raw)
		if err != nil {
			t.Errorf("detectImportSource(%q) unexpected error: %v", raw, err)
			continue
		}
		if src != sourceSkillsSh {
			t.Errorf("detectImportSource(%q) source = %d, want sourceSkillsSh", raw, src)
		}
		if normalized == "" {
			t.Errorf("detectImportSource(%q) returned empty normalized URL", raw)
		}
	}
}

func TestDetectImportSource_BareSlug(t *testing.T) {
	// A bare slug with no dots or slashes defaults to clawhub
	tests := []string{
		"my-skill",
		"some-cool-skill",
	}
	for _, raw := range tests {
		src, _, err := detectImportSource(raw)
		if err != nil {
			t.Errorf("detectImportSource(%q) unexpected error: %v", raw, err)
			continue
		}
		if src != sourceClawHub {
			t.Errorf("detectImportSource(%q) source = %d, want sourceClawHub (bare slug default)", raw, src)
		}
	}
}

func TestDetectImportSource_UnsupportedHost(t *testing.T) {
	// A URL with a dot and slash that isn't clawhub or skills.sh
	_, _, err := detectImportSource("https://github.com/owner/repo")
	if err == nil {
		t.Error("unsupported host should return error")
	}
}

func TestDetectImportSource_NormalizedURLHasScheme(t *testing.T) {
	_, normalized, err := detectImportSource("clawhub.ai/owner/skill")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if normalized != "https://clawhub.ai/owner/skill" {
		t.Errorf("normalized = %q, want https:// prefix", normalized)
	}
}

// ---------------------------------------------------------------------------
// parseClawHubSlug
// ---------------------------------------------------------------------------

func TestParseClawHubSlug_TwoSegments(t *testing.T) {
	slug, err := parseClawHubSlug("https://clawhub.ai/owner/my-skill")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if slug != "my-skill" {
		t.Errorf("slug = %q, want %q", slug, "my-skill")
	}
}

func TestParseClawHubSlug_OneSegment(t *testing.T) {
	slug, err := parseClawHubSlug("https://clawhub.ai/my-skill")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if slug != "my-skill" {
		t.Errorf("slug = %q, want %q", slug, "my-skill")
	}
}

func TestParseClawHubSlug_BareHost(t *testing.T) {
	_, err := parseClawHubSlug("https://clawhub.ai")
	if err == nil {
		t.Error("bare host should return error")
	}
}

func TestParseClawHubSlug_HostWithTrailingSlash(t *testing.T) {
	_, err := parseClawHubSlug("https://clawhub.ai/")
	if err == nil {
		t.Error("host with trailing slash should return error")
	}
}

func TestParseClawHubSlug_ThreeSegments(t *testing.T) {
	// /a/b/c — too many segments, should return error
	_, err := parseClawHubSlug("https://clawhub.ai/a/b/c")
	if err == nil {
		t.Error("three path segments should return error")
	}
}

func TestParseClawHubSlug_PreservesSlugCase(t *testing.T) {
	slug, err := parseClawHubSlug("https://clawhub.ai/owner/My-SKILL")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if slug != "My-SKILL" {
		t.Errorf("slug = %q, want %q (case preserved)", slug, "My-SKILL")
	}
}

// ---------------------------------------------------------------------------
// parseSkillsShParts
// ---------------------------------------------------------------------------

func TestParseSkillsShParts_ValidURL(t *testing.T) {
	owner, repo, skill, err := parseSkillsShParts("https://skills.sh/microsoft/copilot/prompts")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if owner != "microsoft" {
		t.Errorf("owner = %q, want %q", owner, "microsoft")
	}
	if repo != "copilot" {
		t.Errorf("repo = %q, want %q", repo, "copilot")
	}
	if skill != "prompts" {
		t.Errorf("skill = %q, want %q", skill, "prompts")
	}
}

func TestParseSkillsShParts_WithoutScheme(t *testing.T) {
	owner, repo, skill, err := parseSkillsShParts("skills.sh/owner/repo/my-skill")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if owner != "owner" {
		t.Errorf("owner = %q", owner)
	}
	if repo != "repo" {
		t.Errorf("repo = %q", repo)
	}
	if skill != "my-skill" {
		t.Errorf("skill = %q", skill)
	}
}

func TestParseSkillsShParts_TooFewSegments(t *testing.T) {
	_, _, _, err := parseSkillsShParts("https://skills.sh/owner/repo")
	if err == nil {
		t.Error("2 segments should return error")
	}

	_, _, _, err = parseSkillsShParts("https://skills.sh/owner")
	if err == nil {
		t.Error("1 segment should return error")
	}
}

func TestParseSkillsShParts_TooManySegments(t *testing.T) {
	_, _, _, err := parseSkillsShParts("https://skills.sh/a/b/c/d")
	if err == nil {
		t.Error("4 segments should return error")
	}
}

func TestParseSkillsShParts_BareHost(t *testing.T) {
	_, _, _, err := parseSkillsShParts("https://skills.sh")
	if err == nil {
		t.Error("bare host should return error")
	}
}

func TestParseSkillsShParts_HostWithTrailingSlash(t *testing.T) {
	_, _, _, err := parseSkillsShParts("https://skills.sh/")
	if err == nil {
		t.Error("host with trailing slash should return error")
	}
}

// ---------------------------------------------------------------------------
// skillToResponse
// ---------------------------------------------------------------------------

func TestSkillToResponse_Basic(t *testing.T) {
	s := db.Skill{
		ID:          testUUID("11111111-1111-1111-1111-111111111111"),
		WorkspaceID: testUUID("22222222-2222-2222-2222-222222222222"),
		Name:        "my-skill",
		Description: "A test skill",
		Content:     "# SKILL.md content",
		CreatedBy:   testUUID("33333333-3333-3333-3333-333333333333"),
		CreatedAt:   testTimestampFromInt(1700000000),
		UpdatedAt:   testTimestampFromInt(1700000000),
	}

	resp := skillToResponse(s)

	if resp.ID != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("ID = %q", resp.ID)
	}
	if resp.WorkspaceID != "22222222-2222-2222-2222-222222222222" {
		t.Errorf("WorkspaceID = %q", resp.WorkspaceID)
	}
	if resp.Name != "my-skill" {
		t.Errorf("Name = %q", resp.Name)
	}
	if resp.Description != "A test skill" {
		t.Errorf("Description = %q", resp.Description)
	}
	if resp.Content != "# SKILL.md content" {
		t.Errorf("Content = %q", resp.Content)
	}
	if resp.CreatedBy == nil {
		t.Fatal("CreatedBy should not be nil")
	}
	if *resp.CreatedBy != "33333333-3333-3333-3333-333333333333" {
		t.Errorf("CreatedBy = %q", *resp.CreatedBy)
	}
	if resp.CreatedAt == "" {
		t.Error("CreatedAt should not be empty")
	}
}

func TestSkillToResponse_NilConfig(t *testing.T) {
	s := db.Skill{
		ID:          testUUID("11111111-1111-1111-1111-111111111111"),
		WorkspaceID: testUUID("22222222-2222-2222-2222-222222222222"),
		Name:        "no-config",
		Config:      nil,
		CreatedAt:   testTimestampFromInt(1700000000),
		UpdatedAt:   testTimestampFromInt(1700000000),
	}
	resp := skillToResponse(s)

	// nil Config should become empty map
	m, ok := resp.Config.(map[string]any)
	if !ok {
		t.Fatalf("Config should be map[string]any, got %T", resp.Config)
	}
	if len(m) != 0 {
		t.Errorf("Config should be empty map, got %d keys", len(m))
	}
}

func TestSkillToResponse_NilCreatedBy(t *testing.T) {
	s := db.Skill{
		ID:          testUUID("11111111-1111-1111-1111-111111111111"),
		WorkspaceID: testUUID("22222222-2222-2222-2222-222222222222"),
		Name:        "no-creator",
		CreatedBy:   pgtype.UUID{Valid: false},
		CreatedAt:   testTimestampFromInt(1700000000),
		UpdatedAt:   testTimestampFromInt(1700000000),
	}
	resp := skillToResponse(s)

	if resp.CreatedBy != nil {
		t.Errorf("CreatedBy should be nil when UUID is invalid, got %q", *resp.CreatedBy)
	}
}

// ---------------------------------------------------------------------------
// skillFileToResponse
// ---------------------------------------------------------------------------

func TestSkillFileToResponse_Basic(t *testing.T) {
	f := db.SkillFile{
		ID:        testUUID("44444444-4444-4444-4444-444444444444"),
		SkillID:   testUUID("55555555-5555-5555-5555-555555555555"),
		Path:      "scripts/run.sh",
		Content:   "#!/bin/bash\necho hello",
		CreatedAt: testTimestampFromInt(1700000000),
		UpdatedAt: testTimestampFromInt(1700000000),
	}
	resp := skillFileToResponse(f)

	if resp.ID != "44444444-4444-4444-4444-444444444444" {
		t.Errorf("ID = %q", resp.ID)
	}
	if resp.SkillID != "55555555-5555-5555-5555-555555555555" {
		t.Errorf("SkillID = %q", resp.SkillID)
	}
	if resp.Path != "scripts/run.sh" {
		t.Errorf("Path = %q", resp.Path)
	}
	if resp.Content != "#!/bin/bash\necho hello" {
		t.Errorf("Content = %q", resp.Content)
	}
	if resp.CreatedAt == "" {
		t.Error("CreatedAt should not be empty")
	}
}

func TestSkillFileToResponse_EmptyContent(t *testing.T) {
	f := db.SkillFile{
		ID:        testUUID("44444444-4444-4444-4444-444444444444"),
		SkillID:   testUUID("55555555-5555-5555-5555-555555555555"),
		Path:      "empty.txt",
		Content:   "",
		CreatedAt: testTimestampFromInt(1700000000),
		UpdatedAt: testTimestampFromInt(1700000000),
	}
	resp := skillFileToResponse(f)

	if resp.Content != "" {
		t.Errorf("Content should be empty, got %q", resp.Content)
	}
	if resp.Path != "empty.txt" {
		t.Errorf("Path = %q", resp.Path)
	}
}

// ---------------------------------------------------------------------------
// validateFetchURL
// ---------------------------------------------------------------------------

func TestValidateFetchURL_ValidHTTP(t *testing.T) {
	err := validateFetchURL("http://example.com/path")
	if err != nil {
		t.Errorf("valid http URL should not error: %v", err)
	}
}

func TestValidateFetchURL_ValidHTTPS(t *testing.T) {
	err := validateFetchURL("https://example.com/path")
	if err != nil {
		t.Errorf("valid https URL should not error: %v", err)
	}
}

func TestValidateFetchURL_UnsupportedScheme(t *testing.T) {
	err := validateFetchURL("ftp://example.com/file")
	if err == nil {
		t.Error("ftp scheme should return error")
	}
}

func TestValidateFetchURL_MissingHostname(t *testing.T) {
	err := validateFetchURL("https:///path")
	if err == nil {
		t.Error("missing hostname should return error")
	}
}

func TestValidateFetchURL_LoopbackIPv4(t *testing.T) {
	err := validateFetchURL("http://127.0.0.1/internal")
	if err == nil {
		t.Error("loopback IP should return error")
	}
}

func TestValidateFetchURL_LoopbackHost(t *testing.T) {
	err := validateFetchURL("http://localhost/secret")
	if err == nil {
		t.Error("localhost should return error")
	}
}

func TestValidateFetchURL_PrivateIP(t *testing.T) {
	err := validateFetchURL("http://192.168.1.1/admin")
	if err == nil {
		t.Error("private IP should return error")
	}
}

func TestValidateFetchURL_FileScheme(t *testing.T) {
	err := validateFetchURL("file:///etc/passwd")
	if err == nil {
		t.Error("file scheme should return error")
	}
}
