package main

import (
	"testing"

	gh "github.com/alanisme/awesome-zed-extensions/internal/github"
)

func TestIsDedicatedZedRepo_ZedExtensionsOrg(t *testing.T) {
	info := &gh.RepoInfo{Stars: 50000}
	if !isDedicatedZedRepo(info, "zed-extensions", "some-ext", 5000) {
		t.Error("zed-extensions org should always be dedicated")
	}
}

func TestIsDedicatedZedRepo_ZedIndustriesOrg(t *testing.T) {
	info := &gh.RepoInfo{Stars: 50000}
	if !isDedicatedZedRepo(info, "zed-industries", "some-ext", 5000) {
		t.Error("zed-industries org should always be dedicated")
	}
}

func TestIsDedicatedZedRepo_SmallRepo(t *testing.T) {
	info := &gh.RepoInfo{Stars: 100}
	if !isDedicatedZedRepo(info, "random-user", "my-tool", 5000) {
		t.Error("repos with <5000 stars should be dedicated")
	}
}

func TestIsDedicatedZedRepo_LargeWithZedInName(t *testing.T) {
	info := &gh.RepoInfo{Stars: 10000}
	if !isDedicatedZedRepo(info, "someone", "zed-theme", 5000) {
		t.Error("large repo with 'zed' in name should be dedicated")
	}
}

func TestIsDedicatedZedRepo_LargeWithZedInDescription(t *testing.T) {
	info := &gh.RepoInfo{Stars: 10000, Description: "A theme for Zed editor"}
	if !isDedicatedZedRepo(info, "someone", "cool-theme", 5000) {
		t.Error("large repo with 'zed' in description should be dedicated")
	}
}

func TestIsDedicatedZedRepo_LargeWithZedInTopics(t *testing.T) {
	info := &gh.RepoInfo{Stars: 10000, Topics: []string{"editor", "zed-editor"}}
	if !isDedicatedZedRepo(info, "someone", "cool-theme", 5000) {
		t.Error("large repo with 'zed' in topics should be dedicated")
	}
}

func TestIsDedicatedZedRepo_LargeWithoutZed(t *testing.T) {
	info := &gh.RepoInfo{Stars: 10000, Description: "A general purpose tool", Topics: []string{"cli"}}
	if isDedicatedZedRepo(info, "someone", "opencode", 5000) {
		t.Error("large repo without 'zed' mention should NOT be dedicated")
	}
}

func TestIsDedicatedZedRepo_ExactThreshold(t *testing.T) {
	info := &gh.RepoInfo{Stars: 4999}
	if !isDedicatedZedRepo(info, "someone", "my-tool", 5000) {
		t.Error("repos with 4999 stars should be dedicated")
	}

	info5000 := &gh.RepoInfo{Stars: 5000, Description: "unrelated"}
	if isDedicatedZedRepo(info5000, "someone", "my-tool", 5000) {
		t.Error("repos with exactly 5000 stars and no zed mention should NOT be dedicated")
	}
}
