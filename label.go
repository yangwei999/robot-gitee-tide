package main

import (
	"fmt"
	"strings"
	"time"

	sdk "github.com/opensourceways/go-gitee/gitee"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"
)

type labelLog struct {
	label string
	who   string
	t     time.Time
}

func getLatestLog(ops []sdk.OperateLog, label string, log *logrus.Entry) (labelLog, bool) {
	var t time.Time
	index := -1

	for i := range ops {
		op := &ops[i]

		if !strings.Contains(op.Content, label) {
			continue
		}

		ut, err := time.Parse(time.RFC3339, op.CreatedAt)
		if err != nil {
			log.Warnf("parse time:%s failed", op.CreatedAt)
			continue
		}

		if index < 0 || ut.After(t) {
			t = ut
			index = i
		}
	}

	if index >= 0 {
		if user := ops[index].User; user != nil && user.Login != "" {
			return labelLog{
				label: label,
				t:     t,
				who:   user.Login,
			}, true
		}
	}

	return labelLog{}, false
}

func areAllLabelsReady(labels sets.String, cfg *botConfig) bool {
	for _, l := range cfg.Labels {
		if !labels.Has(l.Label) {
			return false
		}
	}

	for _, l := range cfg.MissingLabels {
		if labels.Has(l.Label) {
			return false
		}
	}

	return true
}

func checkPRLabel(labels sets.String, ops []sdk.OperateLog, cfg *botConfig, log *logrus.Entry) string {
	s := checkLabelNeeded(labels, ops, cfg, log)
	s1 := checkMissingLabel(labels, cfg)

	if s != "" && s1 != "" {
		return s + "\n\n" + s1
	}

	return s + s1
}

func checkMissingLabel(labels sets.String, cfg *botConfig) string {
	n := len(cfg.MissingLabels)
	if n == 0 {
		return ""
	}

	v := make([]string, 0, n)
	for _, label := range cfg.MissingLabels {
		if labels.Has(label.Label) {
			v = append(v, fmt.Sprintf("%s: %s", label.Label, label.TipsIfExisting))
		}
	}

	if n = len(v); n > 0 {
		s := "label exists"

		if n > 1 {
			s = "labels exist"
		}

		return fmt.Sprintf("**The following %s**.\n\n%s", s, strings.Join(v, "\n\n"))
	}

	return ""
}

func checkLabelNeeded(labels sets.String, ops []sdk.OperateLog, cfg *botConfig, log *logrus.Entry) string {
	f := func(label labelConfig) string {
		name := label.Label

		if !labels.Has(name) {
			return label.TipsIfMissing
		}

		v, b := getLatestLog(ops, name, log)
		if !b {
			return fmt.Sprintf("The corresponding operation log is missing. you should delete the label and add it again by correct way")
		}

		if b, s := label.isExpiry(v.t); b {
			return s
		}

		if b, s := label.isAddByOthers(v.who); b {
			return s
		}

		return ""
	}

	v := make([]string, 0, len(cfg.Labels))

	for _, label := range cfg.Labels {
		if s := f(label); s != "" {
			v = append(v, fmt.Sprintf("%s: %s", label.Label, s))
		}
	}

	if n := len(v); n > 0 {
		s := "label is"

		if n > 1 {
			s = "labels are"
		}

		return fmt.Sprintf("**The following %s not ready**.\n\n%s", s, strings.Join(v, "\n\n"))
	}

	return ""
}
