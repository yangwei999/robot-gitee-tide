package main

import (
	"fmt"
	"time"

	"github.com/opensourceways/community-robot-lib/config"
)

// PullRequestMergeType enumerates the types of merges the GITEE API
type PullRequestMergeType string

// Possible types of merges for the gitee merge API
const (
	MergeMerge  PullRequestMergeType = "merge"
	MergeSquash PullRequestMergeType = "squash"
	MergeRebase PullRequestMergeType = "rebase"
)

type configuration struct {
	ConfigItems []botConfig `json:"config_items,omitempty"`
}

func (c *configuration) configFor(org, repo string) *botConfig {
	if c == nil {
		return nil
	}

	items := c.ConfigItems
	v := make([]config.IRepoFilter, len(items))
	for i := range items {
		v[i] = &items[i]
	}

	if i := config.Find(org, repo, v); i >= 0 {
		return &items[i]
	}

	return nil
}

func (c *configuration) Validate() error {
	if c == nil {
		return nil
	}

	items := c.ConfigItems
	for i := range items {
		if err := items[i].validate(); err != nil {
			return err
		}
	}

	return nil
}

func (c *configuration) SetDefault() {
	if c == nil {
		return
	}

	Items := c.ConfigItems
	for i := range Items {
		Items[i].setDefault()
	}
}

type botConfig struct {
	config.RepoFilter

	// MergeMethod is the method to merge PR.
	// The default method of merge. Valid options are squash, rebase, and merge.
	MergeMethod PullRequestMergeType `json:"merge_method,omitempty"`

	// BranchMergeMethod sepecifies the merge method for branches
	BranchMergeMethod map[string]PullRequestMergeType `json:"branch_merge_method,omitempty"`

	// Labels specifies the ones which a PR must have to be merged.
	Labels []labelConfig `json:"labels" required:"true"`

	// MissingLabels specifies the ones which a PR must not have to be merged.
	MissingLabels []missingLabelConfig `json:"missing_labels,omitempty"`
}

func (c *botConfig) setDefault() {
	if c.MergeMethod == "" {
		c.MergeMethod = MergeMerge
	}
}

func (c *botConfig) validate() error {
	invalidMethod := func(m PullRequestMergeType) bool {
		return !(m == MergeMerge || m == MergeSquash || m == MergeRebase)
	}

	if invalidMethod(c.MergeMethod) {
		return fmt.Errorf("unsupported merge method:%s", c.MergeMethod)
	}

	for _, m := range c.BranchMergeMethod {
		if invalidMethod(m) {
			return fmt.Errorf("unsupported merge method:%s", m)
		}
	}

	if len(c.Labels) == 0 {
		return fmt.Errorf("missing labels")
	}

	for _, item := range c.Labels {
		if err := item.validate(); err != nil {
			return err
		}
	}

	for _, item := range c.MissingLabels {
		if err := item.validate(); err != nil {
			return err
		}
	}

	return c.RepoFilter.Validate()
}

func (c *botConfig) getMergeMethod(branch string) PullRequestMergeType {
	if len(c.BranchMergeMethod) == 0 {
		return c.MergeMethod
	}

	if m, ok := c.BranchMergeMethod[branch]; ok {
		return m
	}

	return c.MergeMethod
}

type missingLabelConfig struct {
	// Label is the name of it
	Label string `json:"label" required:"true"`

	// TipsIfExisting describe tips when the label exists.
	TipsIfExisting string `json:"tips_if_existing" required:"true"`
}

func (c missingLabelConfig) validate() error {
	if c.Label == "" {
		return fmt.Errorf("missing label")
	}

	if c.TipsIfExisting == "" {
		return fmt.Errorf("missing tips_if_existing")
	}

	return nil
}

type labelConfig struct {
	// Label is the name of it
	Label string `json:"label" required:"true"`

	// TipsIfMissing describe tips when the label is not exist.
	TipsIfMissing string `json:"tips_if_missing" required:"true"`

	// Person specifies who can add the label. The value is the login id on Gitee
	Person string `json:"person,omitempty"`

	// TipsIfAddedByOthers describe tips when the label is added by others.
	TipsIfAddedByOthers string `json:"tips_if_added_by_others,omitempty"`

	// ActiveTime is the time in hours that the label becomes invalid after it from created
	ActiveTime int `json:"active_time,omitempty"`

	// TipsIfExpiry describe tips when the label is expiry.
	TipsIfExpiry string `json:"tips_if_expiry,omitempty"`
}

func (l labelConfig) validate() error {
	if l.Label == "" {
		return fmt.Errorf("missing label")
	}

	if l.TipsIfMissing == "" {
		return fmt.Errorf("missing tips_if_missing")
	}

	if l.Person != "" && l.TipsIfAddedByOthers == "" {
		return fmt.Errorf("must set tips_if_added_by_others if person is set")
	}

	if l.ActiveTime > 0 && l.TipsIfExpiry == "" {
		return fmt.Errorf("must set tips_if_expiry if active_time is set")
	}

	return nil
}

func (l labelConfig) isExpiry(t time.Time) (bool, string) {
	f := func() bool {
		if l.ActiveTime <= 0 {
			return false
		}

		return t.Add(time.Duration(l.ActiveTime) * time.Hour).Before(time.Now())
	}

	return f(), l.TipsIfExpiry
}

func (l labelConfig) isAddByOthers(other string) (bool, string) {
	b := l.Person != "" && l.Person != other

	return b, l.TipsIfAddedByOthers
}
