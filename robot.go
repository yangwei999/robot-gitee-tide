package main

import (
	"fmt"
	"regexp"

	"github.com/opensourceways/community-robot-lib/config"
	"github.com/opensourceways/community-robot-lib/giteeclient"
	"github.com/opensourceways/community-robot-lib/robot-gitee-framework"
	sdk "github.com/opensourceways/go-gitee/gitee"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"
)

const botName = "tide"

var (
	checkPRRe          = regexp.MustCompile(`(?mi)^/check-pr\s*$`)
	tideNotification   = "@%s, This pr is not mergeable."
	tideNotificationRe = regexp.MustCompile(fmt.Sprintf(tideNotification, "(.*)"))
)

type iClient interface {
	CreatePRComment(owner, repo string, number int32, comment string) error
	DeletePRComment(org, repo string, ID int32) error
	ListPRComments(org, repo string, number int32) ([]sdk.PullRequestComments, error)
	MergePR(owner, repo string, number int32, opt sdk.PullRequestMergePutParam) error
	ListPROperationLogs(org, repo string, number int32) ([]sdk.OperateLog, error)
	GetBot() (sdk.User, error)
	UpdatePullRequest(org, repo string, number int32, param sdk.PullRequestUpdateParam) (sdk.PullRequest, error)
}

func newRobot(cli iClient, botName string) *robot {
	return &robot{cli: cli, botName: botName}
}

type robot struct {
	cli     iClient
	botName string
}

func (bot *robot) NewConfig() config.Config {
	return &configuration{}
}

func (bot *robot) getConfig(cfg config.Config, org, repo string) (*botConfig, error) {
	c, ok := cfg.(*configuration)
	if !ok {
		return nil, fmt.Errorf("can't convert to configuration")
	}

	if bc := c.configFor(org, repo); bc != nil {
		return bc, nil
	}

	return nil, fmt.Errorf("no config for this repo:%s/%s", org, repo)
}

func (bot *robot) RegisterEventHandler(f framework.HandlerRegitster) {
	f.RegisterPullRequestHandler(bot.handlePREvent)
	f.RegisterNoteEventHandler(bot.handleNoteEvent)
}

func (bot *robot) handlePREvent(e *sdk.PullRequestEvent, c config.Config, log *logrus.Entry) error {
	if e.GetPullRequest().GetState() != sdk.StatusOpen {
		return nil
	}

	if sdk.GetPullRequestAction(e) != sdk.PRActionUpdatedLabel {
		return nil
	}

	org, repo := e.GetOrgRepo()

	// In order to avoid adding two or more comments in the concurrence of webhook,
	// do pre-check first. Besides, this will enhance the user experience.
	// Because if it doesn't leave a comment, developer has to comment /check-pr
	// to get details.
	return bot.handle(org, repo, e.GetPullRequest(), c, log, areAllLabelsReady)
}

func (bot *robot) handleNoteEvent(e *sdk.NoteEvent, c config.Config, log *logrus.Entry) error {
	if !e.IsCreatingCommentEvent() || !e.IsPullRequest() {
		return nil
	}

	if !checkPRRe.MatchString(e.GetComment().GetBody()) {
		return nil
	}

	org, repo := e.GetOrgRepo()
	if !e.GetPullRequest().GetMergeable() {
		return bot.writeComment(
			org, repo, e.GetPRNumber(), e.GetPRAuthor(),
			" Because it conflicts to the target branch.",
		)
	}

	return bot.handle(org, repo, e.GetPullRequest(), c, log, nil)
}

func (bot *robot) handle(
	org, repo string,
	pr *sdk.PullRequestHook,
	c config.Config,
	log *logrus.Entry,
	preCheck func(labels sets.String, cfg *botConfig) bool,
) error {
	cfg, err := bot.getConfig(c, org, repo)
	if err != nil {
		return err
	}

	if preCheck != nil && !preCheck(pr.LabelsToSet(), cfg) {
		return nil
	}

	number := pr.GetNumber()

	ops, err := bot.cli.ListPROperationLogs(org, repo, number)
	if err != nil {
		return err
	}

	if details := checkPRLabel(pr.LabelsToSet(), ops, cfg, log); details != "" {
		return bot.writeComment(org, repo, number, pr.GetUser().GetLogin(), "\n\n"+details)
	}

	if pr.GetNeedTest() || pr.GetNeedReview() {
		v := int32(0)
		p := sdk.PullRequestUpdateParam{
			AssigneesNumber: &v,
			TestersNumber:   &v,
		}

		if _, err := bot.cli.UpdatePullRequest(org, repo, number, p); err != nil {
			return err
		}
	}

	return bot.cli.MergePR(
		org, repo, number,
		sdk.PullRequestMergePutParam{
			MergeMethod: string(cfg.getMergeMethod(pr.GetBase().GetRef())),
		},
	)
}

func (bot *robot) writeComment(org, repo string, number int32, prAuthor, c string) error {
	_ = bot.deleteOldComments(org, repo, number)

	return bot.cli.CreatePRComment(
		org, repo, number,
		fmt.Sprintf(tideNotification, prAuthor)+c,
	)
}

func (bot *robot) deleteOldComments(org, repo string, number int32) error {
	comments, err := bot.cli.ListPRComments(org, repo, number)
	if err != nil {
		return err
	}

	for _, c := range giteeclient.FindBotComment(comments, bot.botName, tideNotificationRe.MatchString) {
		_ = bot.cli.DeletePRComment(org, repo, c.CommentID)
	}

	return nil
}
